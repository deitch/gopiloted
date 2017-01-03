package gopiloted

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	consulapi "github.com/hashicorp/consul/api"
)

const (
	// CONFVAR name of environment variable that contains default path to containerpilot.json
	CONFVAR = "CONTAINERPILOT"
)

var (
	confMap       map[string]string
	configfile    string
	sigc          chan os.Signal
	backends      map[string]*backend
	catalog       *consulapi.Catalog
	updating      = false
	updateChannel = make(chan bool)
)

// Endpoint struct containing a single service endpoint, composed of an Address string and Port int
type Endpoint struct {
	Address string
	Port    int
}
type backend struct {
	pointer  int
	services []*consulapi.CatalogService
}

func reloadServices() error {
	for name := range backends {
		services, _, err := catalog.Service(name, "", nil)
		if err != nil {
			return err
		}
		backends[name] = &backend{pointer: 0, services: services}
	}
	return nil
}

func completeUpdate() {
	updating = false
	// make sure to tell anyone waiting, and use a non-blocking channel
	select {
	case updateChannel <- true:
	default:
	}
}

// Service returns a single Endpoint for a service of the given name, error if the service is unknown
func Service(name string) (Endpoint, error) {
	// if we are in the middle of an update, wait until it is done, checking every POLL milliseconds
	if updating {
		<-updateChannel
	}
	// find the next service to use
	backendEntry, exists := backends[name]
	if !exists {
		return Endpoint{}, fmt.Errorf("Invalid service: %s", name)
	}
	// calculate next index to use and save it
	// we probably should just use math.Mod here, but for small integers, why bother converting to float64
	pointer := backendEntry.pointer
	service := backendEntry.services[pointer]
	endpoint := Endpoint{
		Address: service.ServiceAddress,
		Port:    service.ServicePort,
	}
	pointer++
	if pointer >= len(backendEntry.services) {
		pointer = 0
	}
	backendEntry.pointer = pointer
	return endpoint, nil
}

// Config initializes go-piloted using the containerpilot.json configfile at the path passed in the argument.
// If blank, default to the configfile available in the path that is the value of the environment variable
// CONTAINERPILOT. Returns error if: no configfile defined in argument or env var; no config file
// at that path; or config file at that path is invalid
func Config(configfile string) error {
	// we are updating when config is running
	updating = true
	// very beginning, we need our config
	if len(configfile) == 0 {
		configfile = os.Getenv(CONFVAR)
	}
	if len(configfile) == 0 {
		return errors.New("undefined configfile")
	}

	// read containerpilot config
	config, err := ioutil.ReadFile(configfile)
	if err != nil {
		return err
	}

	// parse the template
	matcher := regexp.MustCompile(`\{\{\s*\.(\w+)\s*\}\}`)
	parsed := matcher.ReplaceAllStringFunc(string(config), func(m string) string {
		parts := matcher.FindStringSubmatch(m)
		env := os.Getenv(parts[1])
		if len(env) != 0 {
			return env
		}
		return m
	})

	var parsedData interface{}
	err = json.Unmarshal([]byte(parsed), &parsedData)
	if err != nil {
		return err
	}
	parsedStructured := parsedData.(map[string]interface{})
	consulAddr, exists := parsedStructured["consul"]
	if !exists {
		return errors.New(`Must specify "consul" property in containerpilot config file`)
	}
	switch consulAddr.(type) {
	case string:
		// all is ok
	default:
		return errors.New(`Must specify "consul" property as string in containerpilot config file`)
	}
	// finally convert to string
	consulAddrStr := consulAddr.(string)

	// now we can look it up
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = consulAddrStr
	consul, err := consulapi.NewClient(consulConfig)

	if err != nil {
		return err
	}

	catalog = consul.Catalog()

	// get our backends and initialize each service
	backends = make(map[string]*backend)
	for _, b := range parsedStructured["backends"].([]interface{}) {
		backendEntry := b.(map[string]interface{})
		name := backendEntry["name"].(string)
		services, _, err := catalog.Service(name, "", nil)
		if err != nil {
			return err
		}
		backends[name] = &backend{pointer: 0, services: services}
	}

	// handle SIGHUP to reload config
	if sigc == nil {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGHUP)
		go func() {
			<-sigc
			if !updating {
				updating = true
				err := reloadServices()
				if err != nil {
					// log an error
				}
				completeUpdate()
			}
		}()
	}
	completeUpdate()

	return nil
}
