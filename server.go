package rest

import (
	"bytes"
	"encoding/json"
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

// IndentJSON is the string with which JSON output will be indented. 
var IndentJSON string

// Logger will be used for request and error logging in not nil
var Logger *log.Logger

// DontCheckRequestMethod disables checking for the correct
// request method for a handler, which would result in a
// 405 error if not correct.
var DontCheckRequestMethod bool

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

A single optional argument can be passed to methodName.
In that case handler is interpreted as an object and
methodName is the name of the handler method of that object.

Format of GET handler:

	func([url.Values]) ([struct|*struct|string][, error]) {}

*/
func HandleGet(path string, handler interface{}, methodName ...string) {
	handlerFunc := getHandlerFunc(handler, methodName)
	t := handlerFunc.Type()
	if t.Kind() != reflect.Func {
		panic(fmt.Errorf("HandleGet(): handler must be a function, got %T", handler))
	}
	httpHandler := &httpHandler{
		method:      "GET",
		handlerFunc: handlerFunc,
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

A single optional argument can be passed to methodName.
In that case handler is interpreted as an object and
methodName is the name of the handler method of that object.

Format of POST handler:

	func([*struct|url.Values]) ([struct|*struct|string][, error]) {}

*/
func HandlePost(path string, handler interface{}, methodName ...string) {
	handlerFunc := getHandlerFunc(handler, methodName)
	t := handlerFunc.Type()
	if t.Kind() != reflect.Func {
		panic(fmt.Errorf("HandlePost(): handler must be a function, got %T", handler))
	}
	httpHandler := &httpHandler{
		method:      "POST",
		handlerFunc: handlerFunc,
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
RunServer starts an HTTP server with a given address
with the registered GET and POST handlers.
If stop is non nil then the value true sent over the
channel will gracefully stop the server.
*/
func RunServer(addr string, stop chan bool) {
	server := &http.Server{Addr: addr}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	if stop != nil {
		go func() {
			for {
				if flag := <-stop; flag {
					err := listener.Close()
					if err != nil {
						os.Stderr.WriteString(err.Error())
					}
					return
				}
			}
		}()
	}
	if Logger != nil {
		Logger.Println("go-rest server listening at", listener.Addr())
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
	if Logger != nil {
		Logger.Println("go-rest server stopped")
	}
}

///////////////////////////////////////////////////////////////////////////////
// Internal stuff:

func getHandlerFunc(handler interface{}, methodName []string) reflect.Value {
	switch len(methodName) {
	case 0:
		return reflect.ValueOf(handler)
	case 1:
		handlerFunc := reflect.ValueOf(handler).MethodByName(methodName[0])
		if !handlerFunc.IsValid() {
			panic(fmt.Errorf("WrapMethod(): object of type %T has no method %s", handler, methodName[0]))
		}
		return handlerFunc
	}
	panic(fmt.Errorf("HandleGet(): only zero or one methodName allowed, got %d", len(methodName)))
}

var urlValuesType reflect.Type = reflect.TypeOf((*url.Values)(nil)).Elem()
var errorType reflect.Type = reflect.TypeOf((*error)(nil)).Elem()

type httpHandler struct {
	method      string
	getArgs     func(*http.Request) []reflect.Value
	handlerFunc reflect.Value
	writeResult func([]reflect.Value, http.ResponseWriter)
}

func (self *httpHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if Logger != nil {
		Logger.Println(request.Method, request.URL)
	}
	if !DontCheckRequestMethod && request.Method != self.method {
		http.Error(writer, "405: Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	result := self.handlerFunc.Call(self.getArgs(request))
	self.writeResult(result, writer)
}

func writeError(writer http.ResponseWriter, err error) {
	if Logger != nil {
		Logger.Println("ERROR", err)
	}
	http.Error(writer, err.Error(), http.StatusInternalServerError)
}

func writeResultFunc(t reflect.Type) func([]reflect.Value, http.ResponseWriter) {
	var returnError func(result []reflect.Value, writer http.ResponseWriter) bool
	switch t.NumOut() {
	case 2:
		if t.Out(1) == errorType {
			returnError = func(result []reflect.Value, writer http.ResponseWriter) (isError bool) {
				if isError = !result[1].IsNil(); isError {
					writeError(writer, result[1].Interface().(error))
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
			panic(fmt.Errorf("HandleGet(): first result value of handler must be of type string or struct(pointer), got %s", r))
		}
	case 0:
		return func(result []reflect.Value, writer http.ResponseWriter) {
			// do nothing, status code 200 will be returned
		}
	}
	panic(fmt.Errorf("HandleGet(): zero to two return values allowed, got %d", t.NumIn()))
}
