# go-piloted
Service discovery and registration in [golang](https://golang.org) using [ContainerPilot](https://www.joyent.com/containerpilot). Inspired by [node-piloted](https://github.com/joyent/node-piloted).

## Usage
Use this lib to have your golang app discover its services using container pilot.

```golang
import piloted "github.com/deitch/gopiloted"

// initialize using the config file
err := piloted.Config("/path/to/containerpilot.json")

// get an endpoint for a service
endpoint, err := piloted.Service("servicename")

// do something with the endpoint
fmt.Printf("service instance is at %s:%d\n", endpoint.Address, endpoint.Port)
```

## API

The API consists of the following parts:

1. Initialize the `piloted` instance
2. Retrieve the address and port for a service

### Initialize
go-piloted is initialized by calling `piloted.Config()` and passing it a single argument, the path to your `containerpilot.json`. If the argument is a blank string `""`, then it will look for the path to the config file in the environment variable `CONTAINERPILOT`.

If the file is not there, or both the argument to `Config()` and the env var `CONTAINERPILOT` are blank, an error is returned.

#### Templating
go-piloted will template your configuration file, similar to the way that [ContainerPilot](https://www.joyent.com/containerpilot/docs/configuration) does. If you have an environment variable such as `FOO=BAR` then you can use `{{.FOO}}` in your configuration file and it will be substituted with `BAR`.

### Service
Once go-piloted has been initialized, you can retrieve an IP address and port for a service. For example, to get an IP address and port for a service named `"servicename"`:

```golang
endpoint, err := piloted.Service("servicename")
```

The return value is a `gopiloted.Endpoint` struct:

```golang
type Endpoint struct {
	Address string
	Port    int
}
```

Consul itself can store multiple endpoints for a service. go-piloted caches _all_ of the endpoints for a given service, and returns one of them with each call to `piloted.Service()`, rotating through them in a round-robin fashion.

For example, if the service `"appserver"` has the following three available addresses:

* `10.0.10.10:3000`
* `10.0.20.20:2000`
* `10.0.30.30:5000`

Then the first call to `piloted.Service()` will return `10.0.10.10:3000`, the next `10.0.20.20:2000`, the next `10.0.30.30:5000`, then again `10.0.10.10:3000`, etc.

### Updates
go-piloted listens for a `SIGHUP` and, upon receiving one, updates its cache from consul for each service configured as one of the `backends` in `containerpilot.json`. If any of them changes, it updates its cached list for those services, such that the next call to `piloted.Service()` will return one of the newly-correct addresses.

If go-piloted has received a request for a `Service()` and it still is in process of updating, the call will block until the update is complete.


### Events
go-piloted does not yet support passing events for updated services to the calling application. It is targeted for next release.
