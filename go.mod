module github.com/OpenSlides/openslides-icc-service

go 1.17

require (
	github.com/OpenSlides/openslides-autoupdate-service v0.4.1-0.20220210150646-5678dc385a7d
	github.com/gomodule/redigo v1.8.8
	github.com/google/go-cmp v0.5.7 // indirect
	github.com/ory/dockertest/v3 v3.8.1
	github.com/ostcar/topic v0.3.4
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

//replace github.com/OpenSlides/openslides-autoupdate-service => github.com/MJJojo97/openslides-autoupdate-service v0.4.1-0.20220223151412-4b45460726af
replace github.com/OpenSlides/openslides-autoupdate-service => github.com/MJJojo97/openslides-autoupdate-service v0.4.1-0.20220223164137-658448bbb4dc

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Microsoft/go-winio v0.5.1 // indirect
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/cenkalti/backoff/v4 v4.1.2 // indirect
	github.com/containerd/continuity v0.2.2 // indirect
	github.com/docker/cli v20.10.12+incompatible // indirect
	github.com/docker/docker v20.10.12+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.3.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/mitchellh/mapstructure v1.4.3 // indirect
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/opencontainers/runc v1.1.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	golang.org/x/sys v0.0.0-20220209214540-3681064d5158 // indirect
)
