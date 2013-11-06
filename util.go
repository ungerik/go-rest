package rest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// GetJSON sends a HTTP GET request to addr and
// unmarshalles the JSON response to out.
func GetJSON(addr string, out interface{}) error {
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

// GetJSONStrict sends a HTTP GET request to addr and
// unmarshalles the JSON response to out.
// Returns an error if Content-Type is not application/json.
func GetJSONStrict(addr string, out interface{}) error {
	response, err := http.Get(addr)
	if err != nil {
		return err
	}
	if ct := response.Header.Get("Content-Type"); ct != "application/json" {
		return fmt.Errorf("GetJSONStrict expected Content-Type 'application/json', but got '%s'", ct)
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}
