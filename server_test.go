package rest

import (
	"testing"
)

const ServerAddr = "0.0.0.0:8080"

var closeChan chan bool = make(chan bool)

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

var RefStruct Struct = Struct{
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
	go ListenAndServe(ServerAddr, closeChan)
}

func TestHandleGet(t *testing.T) {
	path := "/"
	HandleGet(path, func() *Struct {
		return NewStruct()
	})

	var result Struct
	err := GetJson("http://"+ServerAddr+path, &result)
	if err != nil {
		t.Error(err)
	}

	if result != RefStruct {
		t.Errorf("GET %s: invalid result", ServerAddr+path)
	}
}

func TestStopServer(t *testing.T) {
	closeChan <- true
}
