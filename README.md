# go-envvar

[![GoDoc](https://godoc.org/github.com/plaid/go-envvar/envvar?status.svg)](https://godoc.org/github.com/plaid/go-envvar/envvar)

A go library for managing environment variables. It maps environment variables to
typed fields in a struct, and supports required and optional vars with defaults.

go-envvar is inspired by the javascript library https://github.com/plaid/envvar.

## Example Usage

```go
package main

import (
	"log"
	"github.com/plaid/go-envvar/envvar"
)

type serverEnvVars struct {
	// Since we didn't provide a default, the environment variable GO_PORT is
	// required. Parse will set the Port field to the value of the environment
	// variable and return an error if the environment variable is not set.
	Port int `envvar:"GO_PORT"`
	// Since MaxConns has a default value, it is optional. The value of
	// the environment variable, if set, overrides the default value.
	MaxConns uint `envvar:"MAX_CONNECTIONS" default:"100"`
	// Similar to GO_PORT, HOST_NAME is required.
	HostName string `envvar:"HOST_NAME"`
}

func main() {
	vars := serverEnvVars{}
	if err := envvar.Parse(&vars); err != nil {
		log.Fatal(err)
	}
	// Do something with the parsed environment variables...
}
```
