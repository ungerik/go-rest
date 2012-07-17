package rest

import (
	"encoding/json"
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
