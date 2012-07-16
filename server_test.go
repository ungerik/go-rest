package rest

import (
	"net/http"
	"testing"
)

const ServerAddr = ":8080"

var closeChan chan bool = make(chan bool)

func startServer() {
	ListenAndServe(ServerAddr, closeChan)
}

func stopServer() {
	closeChan <- true
}

func TestStartStopServer(t *testing.T) {
	go ListenAndServe(ServerAddr, closeChan)
	closeChan <- true
}

type TestStruct struct {
	Int    int
	String string
}

func TestHandleGet(t *testing.T) {
	http.DefaultServeMux = http.NewServeMux()
	HandleGet("/", func() *TestStruct {
		return nil
	})
	// Output:
}
