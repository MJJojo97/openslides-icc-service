package run

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/OpenSlides/openslides-autoupdate-service/pkg/auth"
	"github.com/OpenSlides/openslides-autoupdate-service/pkg/datastore"
	messageBusRedis "github.com/OpenSlides/openslides-autoupdate-service/pkg/redis"
	"github.com/OpenSlides/openslides-icc-service/internal/applause"
	"github.com/OpenSlides/openslides-icc-service/internal/icchttp"
	"github.com/OpenSlides/openslides-icc-service/internal/icclog"
	"github.com/OpenSlides/openslides-icc-service/internal/notify"
	"github.com/OpenSlides/openslides-icc-service/internal/redis"
)

// Run starts the http server.
//
// The server is automaticly closed when ctx is done.
//
// The service is configured by the argument `environment`. It expect strings in
// the format `KEY=VALUE`, like the output from `os.Environmen()`.
func Run(ctx context.Context, environment []string, secret func(name string) (string, error)) error {
	env := defaultEnv(environment)

	errHandler := buildErrHandler()

	messageBus, err := buildMessageBus(env)
	if err != nil {
		return fmt.Errorf("building message bus: %w", err)
	}

	auth, err := buildAuth(
		ctx,
		env,
		secret,
		messageBus,
		errHandler,
	)
	if err != nil {
		return fmt.Errorf("building auth: %w", err)
	}

	ds, err := buildDatastore(env, messageBus)
	if err != nil {
		return fmt.Errorf("build datastore service: %w", err)
	}

	backend := redis.New(env["ICC_REDIS_HOST"] + ":" + env["ICC_REDIS_PORT"])

	notifyService := notify.New(ctx, backend)
	applauseService := applause.New(backend, ds, ctx.Done())
	go applauseService.Loop(ctx, errHandler)
	go applauseService.PruneOldData(ctx)

	mux := http.NewServeMux()
	icchttp.HandleHealth(mux)
	notify.HandleReceive(mux, notifyService, auth)
	notify.HandlePublish(mux, notifyService, auth)
	applause.HandleReceive(mux, applauseService, auth)
	applause.HandleSend(mux, applauseService, auth)

	listenAddr := ":" + env["ICC_PORT"]
	srv := &http.Server{Addr: listenAddr, Handler: mux}

	// Shutdown logic in separate goroutine.
	wait := make(chan error)
	go func() {
		// Wait for the context to be closed.
		<-ctx.Done()

		if err := srv.Shutdown(context.Background()); err != nil {
			wait <- fmt.Errorf("HTTP server shutdown: %w", err)
			return
		}
		wait <- nil
	}()

	icclog.Info("Listen on %s", listenAddr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("HTTP Server failed: %v", err)
	}

	return <-wait
}

// defaultEnv parses the environment (output from os.Environ()) and sets specific
// defaut values.
func defaultEnv(environment []string) map[string]string {
	env := map[string]string{
		"ICC_PORT": "9007",

		"ICC_REDIS_HOST": "localhost",
		"ICC_REDIS_PORT": "6379",

		"DATASTORE_READER_HOST":     "localhost",
		"DATASTORE_READER_PORT":     "9010",
		"DATASTORE_READER_PROTOCOL": "http",

		"MESSAGING":        "fake",
		"MESSAGE_BUS_HOST": "localhost",
		"MESSAGE_BUS_PORT": "6379",
		"REDIS_TEST_CONN":  "true",

		"AUTH":          "fake",
		"AUTH_PROTOCOL": "http",
		"AUTH_HOST":     "localhost",
		"AUTH_PORT":     "9004",

		"OPENSLIDES_DEVELOPMENT": "false",
	}

	for _, value := range environment {
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 {
			panic(fmt.Sprintf("Invalid value from environment(): %s", value))
		}

		env[parts[0]] = parts[1]
	}
	return env
}

func secret(name string, getSecret func(name string) (string, error), dev bool) (string, error) {
	defaultSecrets := map[string]string{
		"auth_token_key":  auth.DebugTokenKey,
		"auth_cookie_key": auth.DebugCookieKey,
	}

	d, ok := defaultSecrets[name]
	if !ok {
		return "", fmt.Errorf("unknown secret %s", name)
	}

	s, err := getSecret(name)
	if err != nil {
		if !dev {
			return "", fmt.Errorf("can not read secret %s: %w", s, err)
		}
		s = d
	}
	return s, nil
}

func buildErrHandler() func(err error) {
	return func(err error) {
		var closing interface {
			Closing()
		}
		if !errors.As(err, &closing) {
			icclog.Info("Error: %v", err)
		}
	}
}

// buildAuth returns the auth service needed by the http server.
func buildAuth(
	ctx context.Context,
	env map[string]string,
	getSecret func(name string) (string, error),
	receiver auth.LogoutEventer,
	errHandler func(error),
) (icchttp.Authenticater, error) {
	method := env["AUTH"]
	switch method {
	case "ticket":
		icclog.Info("Auth Method: ticket")
		tokenKey, err := secret("auth_token_key", getSecret, env["OPENSLIDES_DEVELOPMENT"] != "false")
		if err != nil {
			return nil, fmt.Errorf("getting token secret: %w", err)
		}

		cookieKey, err := secret("auth_cookie_key", getSecret, env["OPENSLIDES_DEVELOPMENT"] != "false")
		if err != nil {
			return nil, fmt.Errorf("getting cookie secret: %w", err)
		}

		if tokenKey == auth.DebugTokenKey || cookieKey == auth.DebugCookieKey {
			icclog.Info("Auth with debug key")
		}

		protocol := env["AUTH_PROTOCOL"]
		host := env["AUTH_HOST"]
		port := env["AUTH_PORT"]
		url := protocol + "://" + host + ":" + port

		icclog.Info("Auth Service: %s", url)

		a, err := auth.New(url, ctx.Done(), []byte(tokenKey), []byte(cookieKey))
		if err != nil {
			return nil, fmt.Errorf("creating auth connection: %w", err)
		}

		go a.ListenOnLogouts(ctx, receiver, errHandler)
		go a.PruneOldData(ctx)
		return a, nil

	case "fake":
		icclog.Info("Auth Method: FakeAuth (User ID 1 for all requests)")
		return authStub(1), nil

	default:
		return nil, fmt.Errorf("unknown auth method %s", method)
	}
}

// authStub implements the authenticater interface. It allways returs the given
// user id.
type authStub int

// Authenticate does nothing.
func (a authStub) Authenticate(w http.ResponseWriter, r *http.Request) (context.Context, error) {
	return r.Context(), nil
}

// FromContext returns the uid the object was initialiced with.
func (a authStub) FromContext(ctx context.Context) int {
	return int(a)
}

type messageBus interface {
	auth.LogoutEventer
	datastore.Updater
}

func buildMessageBus(env map[string]string) (messageBus, error) {
	serviceName := env["MESSAGING"]
	icclog.Info("Messaging Service: %s", serviceName)

	var conn messageBusRedis.Connection
	switch serviceName {
	case "redis":
		redisAddress := env["MESSAGE_BUS_HOST"] + ":" + env["MESSAGE_BUS_PORT"]
		c := messageBusRedis.NewConnection(redisAddress)
		if env["REDIS_TEST_CONN"] == "true" {
			if err := c.TestConn(); err != nil {
				return nil, fmt.Errorf("connect to redis: %w", err)
			}
		}

		conn = c

	case "fake":
		conn = messageBusRedis.BlockingConn{}
	default:
		return nil, fmt.Errorf("unknown messagin service `%s`", serviceName)
	}

	return &messageBusRedis.Redis{Conn: conn}, nil
}

// buildDatastore configures the datastore service.
func buildDatastore(env map[string]string, updater datastore.Updater) (*datastore.Datastore, error) {
	protocol := env["DATASTORE_READER_PROTOCOL"]
	host := env["DATASTORE_READER_HOST"]
	port := env["DATASTORE_READER_PORT"]
	url := protocol + "://" + host + ":" + port
	source := datastore.NewSourceDatastore(url, updater)
	return datastore.New(source, nil), nil
}
