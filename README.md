gower
=====
a tiny Go framework


Usage
-----
```go

package main

import (
    "github.com/walkr/gower"
)

// Simple Hello World
func hello(c *gower.Context){
    c.Write("Hello World")
}

// Read the name from url and output a template
func sayHi(c *gower.Context){
    data := map[string]interface{}{"name":c.Matches[1]}
    // Template location "www/templates/say-hi.html"
    c.WriteTemplate("say-hi.html", data)
}

// Display some server stats
func serverStats(c *gower.Context){
	c.WriteJson(gower.ServerStat)
}

func main(){

    // Register the routes
    gower.Get("/hello", hello)
    gower.Get("/say-hi/([a-zA-Z]+)", sayHi)
	 gower.Get("/stats", serverStats)
    // Start the server
    gower.Start()

}

```

```bash

# Start app
./bin/app

# Start on a different port and enable debug
./bin/app --port=9000 --debug=true

```

MIT License
