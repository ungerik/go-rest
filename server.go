/*
## go-rest A minimalistic REST framework for Go

* Reflection, Go structs, and JSON marshalling FTW!
* Import: "github.com/ungerik/go-rest"
* Documentation: http://go.pkgdoc.org/github.com/ungerik/go-rest

The framework consists of only three functions:
HandleGet, HandlePost, ListenAndServe.

Discussion:

This can be considered bad design because because
HandleGet and HandlePost use dynamic typing to hide 36 combinations
of handler function types to make the interface _easy_ to use.
36 static functions would have been more lines of code but
dramatic _simpler_ in their implementation.
See this great talk about easy vs. simple:
http://www.infoq.com/presentations/Simple-Made-Easy
Rob Pike may also dislike this approach:
https://groups.google.com/d/msg/golang-nuts/z4T_n4MHbXM/jT9PoYc6I1IJ
On the other side: Are all users of dynamic languages wrong?

Now let's get started with this little madness,
maybe it's useful and fun after all:

HandleGet uses a handler function that returns a struct or string
to create the GET response. Structs will be marshalled als JSON,
strings will be used as body with auto-detected content type.

Format of GET handler:

	func([url.Values]) ([struct|*struct|string][, error]) {}

Example:

	type MyStruct struct {
		A in
		B string
	}

	json.HandleGet("/data.json", func() *MyStruct {
		return &MyStruct{A: 1, B: "Hello World"}
	})

	json.HandleGet("/index.html", func() string {
		return "<!doctype html><p>Hello World"
	})

The GET handler function can optionally accept an url.Values argument
and return an error as second result value that will be displayed as
500 internal server error if not nil.

Example:

	json.HandleGet("/data.json", func(params url.Values) (string, error) {
		v := params.Get("value")
		if v == "" {
			return nil, errors.New("Expecting GET parameter 'value'")
		}
		return "value = " + v, nil
	})

HandlePost maps POST form data or a JSON document to a struct that is passed
to the handler function. An error result from handler will be displayed
as 500 internal server error message. An optional first string result
will be displayed as a 200 response body with auto-detected content type.

Format of POST handler:

	func([*struct|url.Values]) ([struct|*struct|string],[error]) {}

Example:

	json.HandlePost("/change-data", func(data *MyStruct) (err error) {
		// save data
		return err
	})



*/
package rest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
)

/*
HandleGet registers a HTTP GET handler for path.
handler is a function with an optional url.Values argument.

If the first result value of handler is a struct or struct pointer,
then the struct will be marshalled as JSON response.
If the first result value fo handler is a string,
then it will be used as response body with an auto-detected content type.
An optional second result value of type error will 
create a 500 internal server error response if not nil.
All non error responses will use status code 200.

Format of GET handler:

	func([url.Values]) ([struct|*struct|string][, error]) {}

*/
func HandleGet(path string, handler interface{}) {
	t := reflect.TypeOf(handler)
	if t.Kind() != reflect.Func {
		panic(fmt.Errorf("HandleGet(): handler must be a function, got %T", handler))
	}
	httpHandler := &httpHandler{
		handler: handler,
	}
	// Check handler arguments and install getter
	switch t.NumIn() {
	case 0:
		httpHandler.getArgs = func(request *http.Request) []reflect.Value {
			return nil
		}
	case 1:
		if t.In(0) != reflect.TypeOf(url.Values(nil)) {
			panic(fmt.Errorf("HandleGet(): handler argument must be url.Values, got %s", t.In(0)))
		}
		httpHandler.getArgs = func(request *http.Request) []reflect.Value {
			return []reflect.Value{reflect.ValueOf(request.URL.Query())}
		}
	default:
		panic(fmt.Errorf("HandleGet(): handler accepts zero or one arguments, got %d", t.NumIn()))
	}
	httpHandler.writeResult = writeResultFunc(t)
	http.Handle(path, httpHandler)
}

/*
HandlePost registers a HTTP POST handler for path.
The POST request can have the Content-Type
application/x-www-form-urlencoded or text/plain.
handler is a function that takes a struct pointer or url.Values
as argument.

If the request content type is text/plain, then only a struct pointer
is allowed as handler argument and the request body will be interpreted
as JSON and unmarshalled to a new struct instance.

If the request content type multipart/form-data, then only a struct pointer
is allowed as handler argument and a file named JSON 
will be unmarshalled to a new struct instance.

If the request content type is application/x-www-form-urlencoded
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

Format of POST handler:

	func([*struct|url.Values]) ([struct|*struct|string][, error]) {}

*/
func HandlePost(path string, handler interface{}) {
	t := reflect.TypeOf(handler)
	if t.Kind() != reflect.Func {
		panic(fmt.Errorf("HandlePost(): handler must be a function, got %T", handler))
	}
	httpHandler := &httpHandler{
		handler: handler,
	}
	// Check handler arguments and install getter
	switch t.NumIn() {
	case 1:
		a := t.In(0)
		if a != urlValuesType && (a.Kind() != reflect.Ptr || a.Elem().Kind() != reflect.Struct) {
			panic(fmt.Errorf("HandlePost(): first handler argument must be a struct pointer or url.Values, got %s", a))
		}
		httpHandler.getArgs = func(request *http.Request) []reflect.Value {
			ct := request.Header.Get("Content-Type")
			switch ct {
			case "application/x-www-form-urlencoded":
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
				if a == urlValuesType {
					panic(fmt.Errorf("HandlePost(): first handler argument must be a struct pointer when request Content-Type is text/plain, got %s", a))
				}
				s := reflect.New(a.Elem())
				defer request.Body.Close()
				j, err := ioutil.ReadAll(request.Body)
				if err != nil {
					panic(err)
				}
				err = json.Unmarshal(j, s.Interface())
				if err != nil {
					panic(err)
				}
				return []reflect.Value{s}

			case "multipart/form-data":
				if a == urlValuesType {
					panic(fmt.Errorf("HandlePost(): first handler argument must be a struct pointer when request Content-Type is multipart/form-data, got %s", a))
				}
				file, _, err := request.FormFile("JSON")
				if err != nil {
					panic(err)
				}
				s := reflect.New(a.Elem())
				defer file.Close()
				j, err := ioutil.ReadAll(file)
				if err != nil {
					panic(err)
				}
				err = json.Unmarshal(j, s.Interface())
				if err != nil {
					panic(err)
				}
				return []reflect.Value{s}
			}
			panic("Unsupported POST Content-Type: " + ct)
		}
	default:
		panic(fmt.Errorf("HandlePost(): handler accepts only one or thwo arguments, got %d", t.NumIn()))
	}
	httpHandler.writeResult = writeResultFunc(t)
	http.Handle(path, httpHandler)
}

/*
ListenAndServe starts an HTTP server with a given address
with the registered GET and POST handlers.
*/
func ListenAndServe(addr string, close chan bool) {
	server := &http.Server{Addr: addr}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	if close != nil {
		go func() {
			for {
				if flag := <-close; flag {
					err := listener.Close()
					if err != nil {
						os.Stderr.WriteString(err.Error())
					}
					return
				}
			}
		}()
	}
	err = server.Serve(listener)
	// I know, that's a ugly and depending on undocumented behaviour.
	// But when the implementation changes, we'll see it immediatly as panic.
	// To the keepers of the Go standard libraries:
	// I would be useful to return a documented error type
	// when the network connection is closed.
	if !strings.Contains(err.Error(), "use of closed network connection") {
		panic(err)
	}
}

var urlValuesType reflect.Type = reflect.TypeOf((*url.Values)(nil)).Elem()
var errorType reflect.Type = reflect.TypeOf((*error)(nil)).Elem()

type httpHandler struct {
	getArgs     func(*http.Request) []reflect.Value
	handler     interface{}
	writeResult func([]reflect.Value, http.ResponseWriter)
}

func (self *httpHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	result := reflect.ValueOf(self.handler).Call(self.getArgs(request))
	self.writeResult(result, writer)
}

func writeResultFunc(t reflect.Type) func([]reflect.Value, http.ResponseWriter) {
	var returnError func(result []reflect.Value, writer http.ResponseWriter) bool
	switch t.NumOut() {
	case 2:
		if t.Out(1) == errorType {
			returnError = func(result []reflect.Value, writer http.ResponseWriter) (isError bool) {
				if isError = !result[1].IsNil(); isError {
					err := result[1].Interface().(error)
					http.Error(writer, err.Error(), http.StatusInternalServerError)
				}
				return isError
			}
		} else {
			panic(fmt.Errorf("HandleGet(): second result value of handle must be of type error, got %s", t.Out(1)))
		}
		fallthrough
	case 1:
		r := t.Out(0)
		if r.Kind() == reflect.Struct || (r.Kind() == reflect.Ptr && r.Elem().Kind() == reflect.Struct) {
			return func(result []reflect.Value, writer http.ResponseWriter) {
				if returnError != nil && returnError(result, writer) {
					return
				}
				j, err := json.Marshal(result[0].Interface())
				if err != nil {
					http.Error(writer, err.Error(), http.StatusInternalServerError)
					return
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
			panic(fmt.Errorf("HandleGet(): first result value of handler must be of type string or struct(pointer), got %s", r))
		}
	case 0:
		return func(result []reflect.Value, writer http.ResponseWriter) {
			// do nothing, status code 200 will be returned
		}
	}
	panic(fmt.Errorf("HandleGet(): zero to two return values allowed, got %d", t.NumIn()))
}
