// +build !goci
package rest

import (
	"testing"
)

const ServerAddr = "0.0.0.0:8080"

var closeChan = make(chan struct{})

type MyIntType int

type Struct struct {
	Bool      bool
	Int       int
	Uint      uint
	Ignore    int `json:"-"`
	Float32   float32
	Float64   float64
	String    string
	SubStruct SubStruct
}

type SubStruct struct {
	A MyIntType
	B MyIntType
}

func NewStruct() *Struct {
	return &Struct{
		Bool:    true,
		Int:     1,
		Uint:    2,
		Ignore:  3,
		Float32: 4,
		Float64: 5,
		String:  "7",
		SubStruct: SubStruct{
			A: 8,
			B: 9,
		},
	}
}

var RefStruct = Struct{
	Bool:    true,
	Int:     1,
	Uint:    2,
	Ignore:  0, // default value instead of 3 because of json:"-"
	Float32: 4,
	Float64: 5,
	String:  "7",
	SubStruct: SubStruct{
		A: 8,
		B: 9,
	},
}

func TestStartServer(t *testing.T) {
	go RunServer(ServerAddr, closeChan)
}

func TestHandleGET_struct(t *testing.T) {
	path := "/struct.json"
	HandleGET(path, func() *Struct {
		return NewStruct()
	})
	var result Struct
	err := GetJSONStrict("http://"+ServerAddr+path, &result)
	if err != nil {
		t.Error(err)
	}
	if result != RefStruct {
		t.Errorf("GET %s: invalid result", ServerAddr+path)
	}
}

func TestHandleGET_struct_error(t *testing.T) {
	noerrorPath := "/struct_noerror.json"
	HandleGET(noerrorPath, func() (*Struct, error) {
		return NewStruct(), nil
	})
	var result Struct
	err := GetJSONStrict("http://"+ServerAddr+noerrorPath, &result)
	if err != nil {
		t.Error(err)
	}
	if result != RefStruct {
		t.Errorf("GET %s: invalid result", ServerAddr+noerrorPath)
	}

	// errorPath := "/struct_error.json"
	// errorMessage := "Test Error"
	// HandleGET(path, func() (*Struct, error) {
	// 	return nil, errors.New(errorMessage)
	// })
	// response, err := http.Get("http://"+ServerAddr+errorPath)
	// if err != nil {
	// 	t.Error(err)
	// }
	// if response.Header.
	// if result != RefStruct {
	// 	t.Errorf("GET %s: invalid result", ServerAddr+errorPath)
	// }
}

// TODO: needs much more testing, but see example for some working code

func TestStopServer(t *testing.T) {
	closeChan <- struct{}{}
}
