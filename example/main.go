package main

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/ungerik/go-rest"
)

func main() {
	// Make debugging easier
	rest.DontCheckRequestMethod = true
	rest.IndentJSON = "  "

	// See RunServer below
	stopServerChan := make(chan bool)

	rest.HandleGET("/struct.json", func() *Struct {
		return NewStruct()
	})

	rest.HandleGET("/get-method", (*Struct).Get, NewStruct())

	rest.HandleGET("/index.html", func() string {
		return "<!doctype html><p>Hello World!"
	})

	rest.HandleGET("/error", func() (*Struct, error) {
		return nil, errors.New("This is an error!")
	})

	rest.HandleGET("/close", func() string {
		stopServerChan <- true
		return "stoping server..."
	})

	// Try: http://0.0.0.0:8080/post/struct.json?Int=66&Bool=true
	rest.HandlePOST("/post/struct.json", func(in *Struct) *Struct {
		return in
	})

	// Try: http://0.0.0.0:8080/post/values?Int=66&Bool=true
	rest.HandlePOST("/post/values", func(in url.Values) string {
		return fmt.Sprintf("%v", in)
	})

	rest.RunServer("0.0.0.0:8080", stopServerChan)
}
