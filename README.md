## go-rest A minimalistic REST framework for Go

* Go structs and JSON marshalling FTW!
* Import: "github.com/ungerik/go-rest"
* Documentation: http://go.pkgdoc.org/github.com/ungerik/go-rest
* To Do: more testing

The framework consists of only three functions:
HandleGet, HandlePost, ListenAndServe.

Discussion:

This can be considered bad design because because
HandleGet and HandlePost use dynamic typing to hide 36 combinations
of handler function types to make the interface _easy_ to use.
36 static functions would have been more lines of code but
_simpler_ in their implementation than the dynamic solution.
See this great talk about easy vs. simple:
http://www.infoq.com/presentations/Simple-Made-Easy
Rob Pike may also dislike this approach:
https://groups.google.com/d/msg/golang-nuts/z4T_n4MHbXM/jT9PoYc6I1IJ
On the other side: Are all users of dynamic languages wrong?

So let's start with the dynamic fun:

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
