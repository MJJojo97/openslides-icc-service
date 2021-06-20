package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenSlides/openslides-icc-service/internal/log"
	"github.com/gomodule/redigo/redis"
)

const (
	// iccKey is the name of the icc stream name.
	iccKey = "icc"

	// applauseKey is the name of the redis key for applause.
	applauseKey = "applause"
)

// Redis implements the icc backend by saving the data to redis.
//
// Has to be created with redis.New().
type Redis struct {
	pool      *redis.Pool
	lastICCID string
}

// New creates a new initializes redis instance.
func New(addr string) *Redis {
	pool := redis.Pool{
		MaxActive:   100,
		Wait:        true,
		MaxIdle:     10,
		IdleTimeout: 240 * time.Second,
		Dial:        func() (redis.Conn, error) { return redis.Dial("tcp", addr) },
	}

	return &Redis{
		pool: &pool,
	}
}

// Wait blocks until a connection to redis can be established.
func (r *Redis) Wait(ctx context.Context) {
	for ctx.Err() == nil {
		conn := r.pool.Get()
		_, err := conn.Do("PING")
		conn.Close()
		if err == nil {
			return
		}
		log.Info("Waiting for redis: %v", err)
		time.Sleep(500 * time.Millisecond)
	}
}

// SendICC saves a valid icc message.
func (r *Redis) SendICC(message []byte) error {
	conn := r.pool.Get()
	defer conn.Close()

	_, err := conn.Do("XADD", iccKey, "*", "content", message)
	if err != nil {
		return fmt.Errorf("xadd: %w", err)
	}
	return nil
}

// ReceiveICC is a blocking function that receives the messages.
//
// The first call returnes the first icc message, the next call the second
// an so on. If there are no more messages to read, the function blocks
// until there is or the context ist canceled.
//
// It is expected, that only one goroutine is calling this function.
func (r *Redis) ReceiveICC(ctx context.Context) ([]byte, error) {
	id := r.lastICCID
	if id == "" {
		id = "$"
	}

	type streamReturn struct {
		id   string
		data []byte
		err  error
	}

	streamFinished := make(chan streamReturn)

	go func() {
		conn := r.pool.Get()
		defer conn.Close()

		id, data, err := stream(conn.Do("XREAD", "COUNT", 1, "BLOCK", "0", "STREAMS", iccKey, id))
		streamFinished <- streamReturn{id, data, err}
	}()

	var received streamReturn
	select {
	case received = <-streamFinished:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if received.id != "" {
		r.lastICCID = id
	}

	if err := received.err; err != nil {
		return nil, fmt.Errorf("read icc message from redis: %w", err)
	}

	return received.data, nil
}

// SendApplause saves an applause for the user at a given time as unix time
// stamp.
func (r *Redis) SendApplause(userID int, time int64) error {
	conn := r.pool.Get()
	defer conn.Close()

	if _, err := conn.Do("ZADD", applauseKey, time, userID); err != nil {
		return fmt.Errorf("adding applause in redis: %w", err)
	}

	return nil
}

// ReceiveApplause returned all applause since a given time as unix time stamp.
// Each user is only called once.
func (r *Redis) ReceiveApplause(since int64) (int, error) {
	conn := r.pool.Get()
	defer conn.Close()

	n, err := redis.Int(conn.Do("ZCOUNT", applauseKey, since, "+inf"))
	if err != nil {
		return 0, fmt.Errorf("getting applause from redis: %w", err)
	}

	return n, nil
}

// DeleteOldApplause removes applause that is older then a given time.
func (r *Redis) DeleteOldApplause(olderThen int64) error {
	conn := r.pool.Get()
	defer conn.Close()

	if _, err := conn.Do("ZREMRANGEBYSCORE", applauseKey, 0, olderThen-1); err != nil {
		return fmt.Errorf("removing old applause from redis: %w", err)
	}
	return nil
}
