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

	rest.HandleGet("/struct.json", func() *Struct {
		return NewStruct()
	})

	rest.HandleGet("/get-method", (*Struct).Get, NewStruct())

	rest.HandleGet("/index.html", func() string {
		return "<!doctype html><p>Hello World!"
	})

	rest.HandleGet("/error", func() (*Struct, error) {
		return nil, errors.New("This is an error!")
	})

	rest.HandleGet("/close", func() string {
		stopServerChan <- true
		return "stoping server..."
	})

	// Try: http://0.0.0.0:8080/post/struct.json?Int=66&Bool=true
	rest.HandlePost("/post/struct.json", func(in *Struct) *Struct {
		return in
	})

	// Try: http://0.0.0.0:8080/post/values?Int=66&Bool=true
	rest.HandlePost("/post/values", func(in url.Values) string {
		return fmt.Sprintf("%v", in)
	})

	rest.RunServer("0.0.0.0:8080", stopServerChan)
}
