# YakAPI Server

The YakAPI Server provides an API for [Yak
Rovers](https://github.com/The-Yak-Collective/yakrover). It is intended to
provide a generic interface and allow specific rovers to implement functionality
appropriate for their hardware, capabilities and purpose.

## Usage

`yakapi` acts as a http server that presents a JSON API

### API

The JSON API is versioned, so start with `/v1` and you'll receive a nice hello:

```ShellSession
$ curl -s http://localhost:8080/v1 | jq .
{
  "name": "YakAPI (development)",
  "uptime": 1,
  "resources": [
    {
      "name": "metrics",
      "ref": "/metrics"
    },
    {
      "name": "ci",
      "ref": "/v1/ci"
    },
    {
      "name": "cam",
      "ref": "/v1/cam/capture"
    },
    {
      "name": "project",
      "ref": "https://github.com/The-Yak-Collective/yakapi"
    }
  ]
}
```

Clients should expect resources to vary.

### Development

YakAPI lives by [scripts-to-rule-them-all](https://github.com/github/scripts-to-rule-them-all) rules.

But dependencies and setup are limited. Really you should be able to go out of the box with:

```ShellSession
$ script/server
2022-05-13T03:09:53.020Z        INFO    yakapi/main.go:218      starting        {"version": "1.0.0", "port": "8080"}
```

### Production

Sky is the limit, but for easy integration a docker image is available for both
amd64 and arm7 (raspberry pi) platforms:

```ShellSession

$ docker pull docker pull ghcr.io/the-yak-collective/yakapi:latest
...
$ docker run --rm \
  -p 80:8080 \
  -e YAKAPI_NAME="My Rover" \
  -e YAKAPI_MOTOR_ADAPTER="/var/motor/motor.py" \
  -v /var/motor:/var/motor \
  -e YAKAPI_CAM_CAPTURE_PATH="/var/cam/capture.jpeg" \
  -v /var/cam:/var/cam \
 ghcr.io/the-yak-collective/yakapi:latest
...
```

The tricky part is correctly mounting any external dependencies such as
script hooks and camera images into the container.

### Configuration

Configuration is primarily through environment variables

* `YAKAPI_PORT` [default `8080`] port for api server to listen on
* `YAKAPI_NAME` [default `YakBot`] name for rover 
* `YAKAPI_PROJECT_URL` [default `https://github.com/The-Yak-Collective/yakrover`] URL for more information
* `YAKAPI_ADAPTER_MOTOR` [default `echo`] command to be executed to interact with motors
* `YAKAPI_CAM_CAPTURE_PATH` path to image for camera.

## Components

### Metrics

Metrics are served in [prometheus](https://prometheus.io) format:

```ShellSession
$ curl -s http://localhost:8080/metrics
# HELP yakapi_processed_ops_total The total number of processed requests
# TYPE yakapi_processed_ops_total counter
yakapi_processed_ops_total 0
...

```

### ci (command injection)

This service translates commands into motor settings. 

### cam

The camera component can current serve an image if placed in a configured path.
