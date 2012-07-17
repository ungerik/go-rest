package rest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// GetJson sends a HTTP GET request to addr and
// unmarshalles the JSON response to out.
func GetJson(addr string, out interface{}) error {
	response, err := http.Get(addr)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

// GetJson sends a HTTP GET request to addr and
// unmarshalles the JSON response to out.
// Returns an error if Content-Type is not application/json.
func GetJsonStrict(addr string, out interface{}) error {
	response, err := http.Get(addr)
	if err != nil {
		return err
	}
	if ct := response.Header.Get("Content-Type"); ct != "application/json" {
		return fmt.Errorf("GetJsonStrict expected Content-Type 'application/json', but got '%s'", ct)
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}
