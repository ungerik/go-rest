package main

import (
	"fmt"
	"github.com/ungerik/go-rest"
)

func main() {
	rest.HandleGet("/", func() interface{} {
		return "test"
	})

	fmt.Println("Hello World")
}
