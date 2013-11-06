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