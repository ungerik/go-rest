package main

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

func (self *Struct) Get() *Struct {
	return self
}

type MyIntType int

type SubStruct struct {
	A MyIntType
	B MyIntType
}
