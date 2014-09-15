// The MIT License (MIT)

// Copyright (c) 2014 Tony Walker

// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package gower

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"time"

	"github.com/flosch/pongo2"
)

const (
	RED   = "\033[1;31m%s\033[0m"
	GREEN = "\033[1;32m%s\033[0m"
)

// ------
// Types
// ------

// Server Configuration
type Config struct {
	Port        int
	Host        string
	StaticDir   string
	TemplateDir string
	Csrf        bool
	Routes      []*Route
	ColoredLog  bool
	Debug       bool
}

// Context contains the `Request`, `Response` and `Matches` vars
type Context struct {
	Res     http.ResponseWriter
	Req     *http.Request
	Matches []string
}

// Route contains the compiled pattern for the url, the method and the handler
type Route struct {
	re      *regexp.Regexp // url pattern to match
	method  string         // request method
	handler func(*Context) // func to run
}

var templateCache = make(map[string]pongo2.Template)
var ServerConfig = &Config{}
var ServerStat = NewStat()

// ----------------
// Context Methods
// ----------------

// Write plain text response
func (c *Context) Write(data ...interface{}) {
	c.Res.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(c.Res, data...)
}

//
func renderTemplate(filepath string, data map[string]interface{}) []byte {

	var out string
	var err error
	var template pongo2.Template

	// Read the template from the disk every time
	if ServerConfig.Debug {
		newTemplate, err := pongo2.FromFile(filepath)
		if err != nil {
			panic(err)
		}
		template = *newTemplate

	} else {
		// Read the template and cache it
		cached, ok := templateCache[filepath]
		if ok == false {
			newTemplate, err := pongo2.FromFile(filepath)
			if err != nil {
				panic(err)
			}
			templateCache[filepath] = *newTemplate
			cached = *newTemplate
		}
		template = cached
	}

	out, err = template.Execute(data)
	if err != nil {
		panic(err)
	}
	return []byte(out)
}

// Render template and write it to Response
func (c *Context) WriteTemplate(filename string, data map[string]interface{}) {
	c.Res.Header().Set("Content-Type", "text/html; charset=utf-8")
	filepath := ServerConfig.TemplateDir + "/" + filename
	str := renderTemplate(filepath, data)
	c.Res.Write(str)
}

// Write json response
func (c *Context) WriteJson(data interface{}) {
	c.Res.Header().Set("Content-Type", "application/json; charset=utf-8")
	out, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	c.Res.Write(out)
}

// --------
// Methods
// --------

// Create a Config instance
func NewConfig() *Config {
	return &Config{}
}

// Create a new Route
func NewRoute(pattern string, method string, handler func(*Context)) *Route {
	re, err := regexp.Compile("^" + pattern + "$")
	if err != nil {
		log.Fatal(err)
	}
	return &Route{re, method, handler}
}

// Load templates from path
func NewTemplates(path string) *template.Template {
	pattern := filepath.Join(path, "*.html")
	return template.Must(template.ParseGlob(pattern))
}

// Handler to server static content
func ServeStatic() http.Handler {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	staticDir := filepath.Join(cwd, ServerConfig.StaticDir)
	return http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir)))

}

// Register new route
func RegisterRoute(path string, method string, fun func(c *Context)) {
	route := NewRoute(path, method, fun)
	ServerConfig.Routes = append(ServerConfig.Routes, route)
}

// -------------
// HTTP Methods
// -------------

// Create and register a new GET route
func Get(path string, fun func(c *Context)) {
	RegisterRoute(path, "GET", fun)
}

// Create and register a new POST route
func Post(path string, fun func(c *Context)) {
	RegisterRoute(path, "POST", fun)
}

// Create and register a new DELETE route
func Delete(path string, fun func(c *Context)) {
	RegisterRoute(path, "DELETE", fun)
}

// Create and register a new PUT route
func Put(path string, fun func(c *Context)) {
	RegisterRoute(path, "PUT", fun)
}

// ----------
// OTHER
// ----------

// Safely process a request and recover if necessary
func Process(w http.ResponseWriter, r *http.Request) {
	reqStarted := time.Now()
	// Recover if `process` errors
	defer func() {
		if err := recover(); err != nil {
			// Append the error to the method for logging purposes
			log.Println(err)
			method := fmt.Sprintf("%s (%s)", r.Method, err)
			logRequest(r.URL.Path, method, r.RemoteAddr, false, time.Since(reqStarted))
			http.Error(w, "Server Error", 500)
			ServerStat.Increment(500, time.Since(reqStarted))
		}
	}()
	process(w, r)
}

// Process a request
func process(w http.ResponseWriter, r *http.Request) {

	reqStarted := time.Now()

	var foundPattern bool = false
	var foundMethod bool = false
	var routeToExecute *Route
	var foundMatches []string

	// Find the Route to serve this request
	for _, route := range ServerConfig.Routes {
		if matches := route.re.FindStringSubmatch(r.URL.Path); matches != nil {
			foundPattern = true
			if foundPattern {
				foundMethod = (route.method == r.Method)
			}
			if foundPattern && foundMethod {
				routeToExecute = route
				foundMatches = matches
				break
			}
		}
	}

	statusCode := 200
	// Not found
	if !foundPattern {
		http.Error(w, "Not Found", 404)
		statusCode = 404
	} else if !foundMethod { // Invalid Method
		http.Error(w, "Method not allowed", 405)
		statusCode = 405
	}

	// If all is good
	if foundMethod && foundPattern {
		// Execute the routes handler
		context := &Context{Res: w, Req: r, Matches: foundMatches}
		routeToExecute.handler(context)
	}

	duration := time.Since(reqStarted)

	if ServerConfig.Debug {
		logRequest(r.URL.Path, r.Method, r.RemoteAddr, statusCode == 200, duration)
	}

	ServerStat.Increment(statusCode, duration)

}

// Log an incoming request (with colors if requested)
func logRequest(path string, method string, ip string, success bool, duration time.Duration) {
	if ServerConfig.ColoredLog {
		if success {
			method = fmt.Sprintf(GREEN, string(method))
		} else {
			method = fmt.Sprintf(RED, string(method))
		}
	}

	log.Println(method, path, ip, duration)
}

// Display system information
func showInfo() {
	fmt.Printf("\nRuntime\n\n")
	fmt.Printf("* %-13v: %v\n", "PID", os.Getpid())
	fmt.Printf("* %-13v: %v\n", "Host", ServerConfig.Host)
	fmt.Printf("* %-13v: %v\n", "CPUs", runtime.NumCPU())
	fmt.Printf("* %-13v: %v\n", "Arch", runtime.GOARCH)
	fmt.Printf("* %-13v: %v\n", "OS", runtime.GOOS)
	fmt.Printf("* %-13v: %v\n", "Go version", runtime.Version())
	fmt.Printf("* %-13v: %v\n", "Goroutines", runtime.NumGoroutine())
	fmt.Print("\nServing\n\n")
}

// Start the server
func Start() {
	showInfo()
	// Listen and serve
	http.Handle("/static/", ServeStatic())
	http.HandleFunc("/", Process)
	log.Fatal(http.ListenAndServe(ServerConfig.Host, nil))
}

// Default command line options
var (
	port        = flag.Int("port", 8000, "Port number for server")
	staticDir   = flag.String("static-dir", "www/static", "Static directory")
	templateDir = flag.String("template-dir", "www/templates", "Templates directory")
	enableCsrf  = flag.Bool("enable-csrf", false, "Enable cross site request forgery")
	debug       = flag.Bool("debug", false, "Enable debugging")
)

func init() {

	flag.Parse() // read command line flags
	ServerConfig.Port = *port
	ServerConfig.Host = ":" + strconv.Itoa(ServerConfig.Port)
	ServerConfig.StaticDir = *staticDir
	ServerConfig.TemplateDir = *templateDir
	ServerConfig.Routes = []*Route{}
	ServerConfig.Csrf = *enableCsrf
	ServerConfig.ColoredLog = true
	ServerConfig.Debug = *debug
}
