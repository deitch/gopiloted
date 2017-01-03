package gopiloted_test

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	gp "github.com/deitch/gopiloted"
)

const (
	CPVAR       = "CONTAINERPILOT"
	INVALIDPATH = "/asasashashas/2wqwqwqsqsqs.qasas"
	CONSULPORT  = "8510"
	SERVICENAME = "abc"
)

type ServicePoint struct {
	ServiceAddress string
	ServicePort    int
}

var (
	pointLists    [2][]ServicePoint
	active        = 0
	updateChannel = make(chan bool)
)

func svcaHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	marshalled, _ := json.Marshal(pointLists[active])
	w.Write(marshalled)
	select {
	case updateChannel <- true:
	default:
	}
}
func servicesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`["a":[]]`))
}

func TestMain(m *testing.M) {
	// initialize the ServicePoint slice
	pointLists[0] = []ServicePoint{
		ServicePoint{ServiceAddress: "10.10.10.10", ServicePort: 5000},
		ServicePoint{ServiceAddress: "10.10.20.20", ServicePort: 6000},
		ServicePoint{ServiceAddress: "10.10.30.30", ServicePort: 7000},
	}
	pointLists[1] = []ServicePoint{
		ServicePoint{ServiceAddress: "10.50.10.10", ServicePort: 5100},
		ServicePoint{ServiceAddress: "10.50.20.20", ServicePort: 6100},
		ServicePoint{ServiceAddress: "10.50.30.30", ServicePort: 7100},
	}

	os.Setenv("PORT", "8000")
	os.Setenv("CONSULPORT", CONSULPORT)
	os.Setenv("SERVICENAME", SERVICENAME)
	http.HandleFunc("/v1/catalog/services", servicesHandler)
	http.HandleFunc("/v1/catalog/service/"+SERVICENAME, svcaHandler)
	go func() {
		http.ListenAndServe(":"+CONSULPORT, nil)
	}()
	os.Exit(m.Run())
}

func getConfigFile() string {
	cp, err := filepath.Abs("./containerpilot.json")
	if err != nil {
		log.Fatal(err)
	}
	return cp
}

func TestService(t *testing.T) {
	// cleanly initialize
	err := gp.Config(getConfigFile())
	if err != nil {
		t.Errorf("returned error %v", err)
	}
	t.Run("valid service name", testServiceValidName())
	t.Run("invalid service name", testServiceInvalidName())
	t.Run("reset list", testServiceResetList())
}

func compareEndpoints(count int, actual gp.Endpoint, expected ServicePoint, t *testing.T) {
	// make sure the returned endpoint matches
	actualStr := fmt.Sprintf("%s:%d", actual.Address, actual.Port)
	expectedStr := fmt.Sprintf("%s:%d", expected.ServiceAddress, expected.ServicePort)
	if actualStr != expectedStr {
		t.Errorf("count %d: actual %s instead of %s\n", count, actualStr, expectedStr)
	}
}

func testServiceInvalidName() func(*testing.T) {
	return func(t *testing.T) {
		_, err := gp.Service("abcsadgaswjejas")
		if err == nil {
			t.Errorf("returned error %v\n", err)
		}
	}
}

func testServiceValidName() func(*testing.T) {
	return func(t *testing.T) {
		active = 0
		testServiceResponses(t)
	}
}

func testServiceResetList() func(*testing.T) {
	return func(t *testing.T) {
		active = 1
		pid := os.Getpid()
		process, err := os.FindProcess(pid)
		if err != nil {
			t.Error(err)
		}
		process.Signal(syscall.SIGHUP)
		// wait for it to send new data
		<-updateChannel
		testServiceResponses(t)
		active = 0
	}
}

func testServiceResponses(t *testing.T) {
	// set our activeList
	var endpoint gp.Endpoint
	var expected ServicePoint
	var err error
	activeList := pointLists[active]
	count := 0

	// get the service count 0
	endpoint, err = gp.Service(SERVICENAME)
	if err != nil {
		t.Errorf("returned error %v\n", err)
	}
	// make sure the returned endpoint matches
	expected = activeList[count]
	compareEndpoints(count, endpoint, expected, t)

	// get the service count 1
	count++
	endpoint, err = gp.Service(SERVICENAME)
	if err != nil {
		t.Errorf("returned error %v\n", err)
	}
	// make sure the returned endpoint matches
	expected = activeList[count]
	compareEndpoints(count, endpoint, expected, t)

	// get the service count 2
	count++
	endpoint, err = gp.Service(SERVICENAME)
	if err != nil {
		t.Errorf("returned error %v\n", err)
	}
	// make sure the returned endpoint matches
	expected = activeList[count]
	compareEndpoints(count, endpoint, expected, t)

	// reset
	count = 0
	endpoint, err = gp.Service(SERVICENAME)
	if err != nil {
		t.Errorf("returned error %v\n", err)
	}
	// make sure the returned endpoint matches
	expected = activeList[count]
	compareEndpoints(count, endpoint, expected, t)
}

func TestConfig(t *testing.T) {
	t.Run("empty arg and no env var", testConfigEmptyArgNoEnv())
	t.Run("empty arg and env var to invalid path", testConfigEmptyArgInvalidEnvPath())
	t.Run("empty arg and env var to valid path", testConfigEmptyArgValidEnvPath())
	t.Run("full arg to invalid path", testConfigArgToInvalidEnvPath())
	t.Run("full arg to invalid path", testConfigArgToValidEnvPath())
	t.Run("missing consul", testConfigMissingConsul())
}

func testConfigEmptyArgNoEnv() func(*testing.T) {
	return func(t *testing.T) {
		// just to be safe, unset it
		os.Unsetenv(CPVAR)
		err := gp.Config("")
		if err == nil {
			t.Error("failed to return error")
		}
	}
}
func testConfigEmptyArgInvalidEnvPath() func(*testing.T) {
	return func(t *testing.T) {
		os.Setenv(CPVAR, INVALIDPATH)
		err := gp.Config("")
		if err == nil {
			t.Error("failed to return error")
		}
	}
}
func testConfigEmptyArgValidEnvPath() func(*testing.T) {
	return func(t *testing.T) {
		os.Setenv(CPVAR, getConfigFile())
		err := gp.Config("")
		if err != nil {
			t.Errorf("returned error %v", err)
		}
	}
}
func testConfigArgToInvalidEnvPath() func(*testing.T) {
	return func(t *testing.T) {
		err := gp.Config(INVALIDPATH)
		if err == nil {
			t.Error("failed to return error")
		}
	}
}
func testConfigArgToValidEnvPath() func(*testing.T) {
	return func(t *testing.T) {
		err := gp.Config(getConfigFile())
		if err != nil {
			t.Errorf("returned error %v", err)
		}
	}
}
func testConfigMissingConsul() func(*testing.T) {
	return func(t *testing.T) {
		os.Setenv("CONSULPORT", "23232")
		err := gp.Config(getConfigFile())
		os.Setenv("CONSULPORT", CONSULPORT)
		if err == nil {
			t.Errorf("returned error %v", err)
		}
	}
}
