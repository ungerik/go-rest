/*
## go-rest A small and evil REST framework for Go

### Reflection, Go structs, and JSON marshalling FTW!

* go get github.com/ungerik/go-rest
* import "github.com/ungerik/go-rest"
* Documentation: http://go.pkgdoc.org/github.com/ungerik/go-rest
* License: Public Domain

Download, build and run example:

	go get github.com/ungerik/go-rest
	go install github.com/ungerik/go-rest/example && example

Small?

Yes, the framework consists of only three functions:
HandleGET, HandlePOST, RunServer.

Evil?

Well, this package can be considered bad design because
HandleGET and HandlePOST use dynamic typing to hide 36 combinations
of handler function types to make the interface _easy_ to use.
36 static functions would have been more lines of code but
dramatic _simpler_ in their individual implementations.
So simple in fact, that there wouldn't be a point in
abstracting them away in an extra framework.
See this great talk about easy vs. simple:
http://www.infoq.com/presentations/Simple-Made-Easy
Rob Pike may also dislike this approach:
https://groups.google.com/d/msg/golang-nuts/z4T_n4MHbXM/jT9PoYc6I1IJ
So yes, this package can be called evil because it is an
anti-pattern to all that is good and right about Go.

Why use it then? By maximizing dynamic code
it is easy to use and reduces code.
Yes, that introduces some internal complexity,
but this complexity is still very low in absolute terms
and thus easy to control and debug.
The complexity of the dynamic code also does not spill over
into the package users' code, because the arguments and
results of the handler functions must be static typed
and can't be interface{}.

Now let's have some fun:

HandleGET uses a handler function that returns a struct or string
to create the GET response. Structs will be marshalled as JSON,
strings will be used as body with auto-detected content type.

Format of GET handler:

	func([url.Values]) ([struct|*struct|string][, error]) {}

Example:

	type MyStruct struct {
		A in
		B string
	}

	rest.HandleGET("/data.json", func() *MyStruct {
		return &MyStruct{A: 1, B: "Hello World"}
	})

	rest.HandleGET("/index.html", func() string {
		return "<!doctype html><p>Hello World"
	})

The GET handler function can optionally accept an url.Values argument
and return an error as second result value that will be displayed as
500 internal server error if not nil.

Example:

	rest.HandleGET("/data.json", func(params url.Values) (string, error) {
		v := params.Get("value")
		if v == "" {
			return nil, errors.New("Expecting GET parameter 'value'")
		}
		return "value = " + v, nil
	})

HandlePOST maps POST form data or a JSON document to a struct that is passed
to the handler function. An error result from handler will be displayed
as 500 internal server error message. An optional first string result
will be displayed as a 200 response body with auto-detected content type.

Suported content types for POST requests are:
* application/x-www-form-urlencoded
* multipart/form-data
* text/plain
* application/json
* application/xml

Format of POST handler:

	func([*struct|url.Values]) ([struct|*struct|string],[error]) {}

Example:

	rest.HandlePOST("/change-data", func(data *MyStruct) (err error) {
		// save data
		return err
	})

Both HandleGET and HandlePOST also accept one optional object argument.
In that case handler is interpreted as a method of the type of object
and called accordingly.

Example:

	rest.HandleGET("/method-call", (*myType).MethodName, myTypeObject)
*/
package rest

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
)

var (
	// IndentJSON is the string with which JSON output will be indented.
	IndentJSON string

	// Log is a function pointer compatible to fmt.Println or log.Println.
	// The default value is log.Println.
	Log = log.Println

	// DontCheckRequestMethod disables checking for the correct
	// request method for a handler, which would result in a
	// 405 error if not correct.
	DontCheckRequestMethod bool
)

/*
HandleGET registers a HTTP GET handler for path.
handler is a function with an optional url.Values argument.

If the first result value of handler is a struct or struct pointer,
then the struct will be marshalled as JSON response.
If the first result value fo handler is a string,
then it will be used as response body with an auto-detected content type.
An optional second result value of type error will
create a 500 internal server error response if not nil.
All non error responses will use status code 200.

A single optional argument can be passed as object.
In that case handler is interpreted as a method and
object is the address of an object with such a method.

Format of GET handler:

	func([url.Values]) ([struct|*struct|string][, error]) {}

*/
func HandleGET(path string, handler interface{}, object ...interface{}) {
	handlerFunc, in, out := getHandlerFunc(handler, object)
	httpHandler := &httpHandler{
		method:      "GET",
		handlerFunc: handlerFunc,
	}
	// Check handler arguments and install getter
	switch len(in) {
	case 0:
		httpHandler.getArgs = func(request *http.Request) []reflect.Value {
			return nil
		}
	case 1:
		if in[0] != reflect.TypeOf(url.Values(nil)) {
			panic(fmt.Errorf("HandleGET(): handler argument must be url.Values, got %s", in[0]))
		}
		httpHandler.getArgs = func(request *http.Request) []reflect.Value {
			return []reflect.Value{reflect.ValueOf(request.URL.Query())}
		}
	default:
		panic(fmt.Errorf("HandleGET(): handler accepts zero or one arguments, got %d", len(in)))
	}
	httpHandler.writeResult = writeResultFunc(out)
	http.Handle(path, httpHandler)
}

/*
HandlePOST registers a HTTP POST handler for path.
handler is a function that takes a struct pointer or url.Values
as argument.

If the request content type is text/plain, then only a struct pointer
is allowed as handler argument and the request body will be interpreted
as JSON and unmarshalled to a new struct instance.

If the request content type multipart/form-data, then only a struct pointer
is allowed as handler argument and a file named JSON
will be unmarshalled to a new struct instance.

If the request content type is empty or application/x-www-form-urlencoded
and the handler argument is of type url.Values, then the form
values will be passed directly as url.Values.
If the handler argument is a struct pointer and the form contains
a single value named "JSON", then the value will be interpreted as
JSON and unmarshalled to a new struct instance.
If there are multiple form values, then they will be set at
struct fields with exact matching names.

If the first result value of handler is a struct or struct pointer,
then the struct will be marshalled as JSON response.
If the first result value fo handler is a string,
then it will be used as response body with an auto-detected content type.
An optional second result value of type error will
create a 500 internal server error response if not nil.
All non error responses will use status code 200.

A single optional argument can be passed as object.
In that case handler is interpreted as a method and
object is the address of an object with such a method.

Format of POST handler:

	func([*struct|url.Values]) ([struct|*struct|string][, error]) {}

*/
func HandlePOST(path string, handler interface{}, object ...interface{}) {
	handlerFunc, in, out := getHandlerFunc(handler, object)
	httpHandler := &httpHandler{
		method:      "POST",
		handlerFunc: handlerFunc,
	}
	// Check handler arguments and install getter
	switch len(in) {
	case 1:
		a := in[0]
		if a != urlValuesType && (a.Kind() != reflect.Ptr || a.Elem().Kind() != reflect.Struct) && a.Kind() != reflect.String {
			panic(fmt.Errorf("HandlePOST(): first handler argument must be a struct pointer, string, or url.Values. Got %s", a))
		}
		httpHandler.getArgs = func(request *http.Request) []reflect.Value {
			ct := request.Header.Get("Content-Type")
			switch ct {
			case "", "application/x-www-form-urlencoded":
				request.ParseForm()
				if a == urlValuesType {
					return []reflect.Value{reflect.ValueOf(request.Form)}
				}
				s := reflect.New(a.Elem())
				if len(request.Form) == 1 && request.Form.Get("JSON") != "" {
					err := json.Unmarshal([]byte(request.Form.Get("JSON")), s.Interface())
					if err != nil {
						panic(err)
					}
				} else {
					v := s.Elem()
					for key, value := range request.Form {
						if f := v.FieldByName(key); f.IsValid() && f.CanSet() {
							switch f.Kind() {
							case reflect.String:
								f.SetString(value[0])
							case reflect.Bool:
								if val, err := strconv.ParseBool(value[0]); err == nil {
									f.SetBool(val)
								}
							case reflect.Float32, reflect.Float64:
								if val, err := strconv.ParseFloat(value[0], 64); err == nil {
									f.SetFloat(val)
								}
							case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
								if val, err := strconv.ParseInt(value[0], 0, 64); err == nil {
									f.SetInt(val)
								}
							case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
								if val, err := strconv.ParseUint(value[0], 0, 64); err == nil {
									f.SetUint(val)
								}
							}
						}
					}
				}
				return []reflect.Value{s}

			case "text/plain":
				if a.Kind() != reflect.String {
					panic(fmt.Errorf("HandlePOST(): first handler argument must be a string when request Content-Type is text/plain, got %s", a))
				}
				defer request.Body.Close()
				body, err := ioutil.ReadAll(request.Body)
				if err != nil {
					panic(err)
				}
				return []reflect.Value{reflect.ValueOf(string(body))}

			case "application/xml":
				if a.Kind() != reflect.Ptr || a.Elem().Kind() != reflect.Struct {
					panic(fmt.Errorf("HandlePOST(): first handler argument must be a struct pointer when request Content-Type is application/xml, got %s", a))
				}
				s := reflect.New(a.Elem())
				defer request.Body.Close()
				body, err := ioutil.ReadAll(request.Body)
				if err != nil {
					panic(err)
				}
				err = xml.Unmarshal(body, s.Interface())
				if err != nil {
					panic(err)
				}
				return []reflect.Value{s}

			case "application/json":
				if a.Kind() != reflect.Ptr || a.Elem().Kind() != reflect.Struct {
					panic(fmt.Errorf("HandlePOST(): first handler argument must be a struct pointer when request Content-Type is application/json, got %s", a))
				}
				s := reflect.New(a.Elem())
				defer request.Body.Close()
				body, err := ioutil.ReadAll(request.Body)
				if err != nil {
					panic(err)
				}
				err = json.Unmarshal(body, s.Interface())
				if err != nil {
					panic(err)
				}
				return []reflect.Value{s}

			case "multipart/form-data":
				if a.Kind() != reflect.Ptr || a.Elem().Kind() != reflect.Struct {
					panic(fmt.Errorf("HandlePOST(): first handler argument must be a struct pointer when request Content-Type is multipart/form-data, got %s", a))
				}
				file, _, err := request.FormFile("JSON")
				if err != nil {
					panic(err)
				}
				s := reflect.New(a.Elem())
				defer file.Close()
				body, err := ioutil.ReadAll(request.Body)
				if err != nil {
					panic(err)
				}
				err = json.Unmarshal(body, s.Interface())
				if err != nil {
					panic(err)
				}
				return []reflect.Value{s}
			}
			panic("Unsupported POST Content-Type: " + ct)
		}
	default:
		panic(fmt.Errorf("HandlePOST(): handler accepts only one or thwo arguments, got %d", len(in)))
	}
	httpHandler.writeResult = writeResultFunc(out)
	http.Handle(path, httpHandler)
}

/*
RunServer starts an HTTP server with a given address
with the registered GET and POST handlers.
If stop is non nil then a send on the channel
will gracefully stop the server.
*/
func RunServer(addr string, stop chan struct{}) {
	server := &http.Server{Addr: addr}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	if stop != nil {
		go func() {
			<-stop
			err := listener.Close()
			if err != nil {
				os.Stderr.WriteString(err.Error())
			}
			return
		}()
	}
	Log("Server listening at", addr)
	err = server.Serve(listener)
	// I know, that's a ugly and depending on undocumented behavior.
	// But when the implementation changes, we'll see it immediately as panic.
	// To the keepers of the Go standard libraries:
	// It would be useful to return a documented error type
	// when the network connection is closed.
	if !strings.Contains(err.Error(), "use of closed network connection") {
		panic(err)
	}
	Log("Server stopped")
}

///////////////////////////////////////////////////////////////////////////////
// Internal stuff:

func getHandlerFunc(handler interface{}, object []interface{}) (f reflectionFunc, in, out []reflect.Type) {
	handlerValue := reflect.ValueOf(handler)
	if handlerValue.Kind() != reflect.Func {
		panic(fmt.Errorf("handler must be a function, got %T", handler))
	}
	handlerType := handlerValue.Type()
	out = make([]reflect.Type, handlerType.NumOut())
	for i := 0; i < handlerType.NumOut(); i++ {
		out[i] = handlerType.Out(i)
	}
	switch len(object) {
	case 0:
		f = func(args []reflect.Value) []reflect.Value {
			return handlerValue.Call(args)
		}
		in = make([]reflect.Type, handlerType.NumIn())
		for i := 0; i < handlerType.NumIn(); i++ {
			in[i] = handlerType.In(i)
		}
		return f, in, out
	case 1:
		objectValue := reflect.ValueOf(object[0])
		if objectValue.Kind() != reflect.Ptr {
			panic(fmt.Errorf("object must be a pointer, got %T", objectValue.Interface()))
		}
		f = func(args []reflect.Value) []reflect.Value {
			args = append([]reflect.Value{objectValue}, args...)
			return handlerValue.Call(args)
		}
		in = make([]reflect.Type, handlerType.NumIn()-1)
		for i := 1; i < handlerType.NumIn(); i++ {
			in[i] = handlerType.In(i)
		}
		return f, in, out
	}
	panic(fmt.Errorf("HandleGET(): only zero or one object allowed, got %d", len(object)))
}

var (
	urlValuesType = reflect.TypeOf((*url.Values)(nil)).Elem()
	errorType     = reflect.TypeOf((*error)(nil)).Elem()
)

type reflectionFunc func([]reflect.Value) []reflect.Value

type httpHandler struct {
	method      string
	getArgs     func(*http.Request) []reflect.Value
	handlerFunc reflectionFunc
	writeResult func([]reflect.Value, http.ResponseWriter)
}

func (handler *httpHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	Log(request.Method, request.URL)
	if !DontCheckRequestMethod && request.Method != handler.method {
		http.Error(writer, "405: Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	result := handler.handlerFunc(handler.getArgs(request))
	handler.writeResult(result, writer)
}

func writeError(writer http.ResponseWriter, err error) {
	Log("ERROR:", err)
	http.Error(writer, err.Error(), http.StatusInternalServerError)
}

func writeResultFunc(out []reflect.Type) func([]reflect.Value, http.ResponseWriter) {
	var returnError func(result []reflect.Value, writer http.ResponseWriter) bool
	switch len(out) {
	case 2:
		if out[1] == errorType {
			returnError = func(result []reflect.Value, writer http.ResponseWriter) (isError bool) {
				if isError = !result[1].IsNil(); isError {
					writeError(writer, result[1].Interface().(error))
				}
				return isError
			}
		} else {
			panic(fmt.Errorf("HandleGET(): second result value of handle must be of type error, got %s", out[1]))
		}
		fallthrough
	case 1:
		r := out[0]
		if r.Kind() == reflect.Struct || (r.Kind() == reflect.Ptr && r.Elem().Kind() == reflect.Struct) {
			return func(result []reflect.Value, writer http.ResponseWriter) {
				if returnError != nil && returnError(result, writer) {
					return
				}
				j, err := json.Marshal(result[0].Interface())
				if err != nil {
					writeError(writer, err)
					return
				}
				if IndentJSON != "" {
					var buf bytes.Buffer
					err = json.Indent(&buf, j, "", IndentJSON)
					if err != nil {
						writeError(writer, err)
						return
					}
					j = buf.Bytes()
				}
				writer.Header().Set("Content-Type", "application/json")
				writer.Write(j)
			}
		} else if r.Kind() == reflect.String {
			return func(result []reflect.Value, writer http.ResponseWriter) {
				if returnError != nil && returnError(result, writer) {
					return
				}
				bytes := []byte(result[0].String())
				ct := http.DetectContentType(bytes)
				writer.Header().Set("Content-Type", ct)
				writer.Write(bytes)
			}
		} else {
			panic(fmt.Errorf("first result value of handler must be of type string or struct(pointer), got %s", r))
		}
	case 0:
		return func(result []reflect.Value, writer http.ResponseWriter) {
			// do nothing, status code 200 will be returned
		}
	}
	panic(fmt.Errorf("zero to two return values allowed, got %d", len(out)))
}
