# go-envvar

[![GoDoc](https://godoc.org/github.com/plaid/go-envvar/envvar?status.svg)](https://godoc.org/github.com/plaid/go-envvar/envvar)

A go library for managing environment variables. It maps environment variables to
typed fields in a struct, and supports required and optional vars with defaults.

go-envvar is inspired by the javascript library https://github.com/plaid/envvar.

go-envvar supports fields of most primative types (e.g. int, string, bool,
float64) as well as any type which implements the
[encoding.TextUnmarshaler](https://golang.org/pkg/encoding/#TextUnmarshaler)
interface.

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
	// Time values are also supported. Parse uses the UnmarshalText method of
	// time.Time in order to set the value of the field. In this case, the
	// UnmarshalText method expects the string value to be in RFC 3339 format.
	StartTime time.Time `envvar:"START_TIME" default:"2017-10-31T14:18:00Z"`
	// Duration is supported via time.ParseDuration()
	Timeout time.Duration `envvar:"TIMEOUT" default:"30m"`
}

func main() {
	vars := serverEnvVars{}
	if err := envvar.Parse(&vars); err != nil {
		log.Fatal(err)
	}
	// Do something with the parsed environment variables...
}
```

### Nested structs

go-envvar also supports nested structs.

```go
type credential struct {
	Username string `envvar:"USERNAME"`
	Password string `envvar:"PASSWORD"`
}

type serverEnvVars struct {
	ServiceA credential `envvar: "SERVICE_A_`
	ServiceB credential `envvar: "SERVICE_B_`
}

func main() {
	vars := serverEnvVars{}
	err := envvar.Parse(&vars)
	// your application logic
}
```

In this example, envvar `SERVICE_A_USERNAME` will be mapped to the field `vars.ServiceA.Username`.
`envvar` struct tags on a struct field are interpreted as prefixes for envvar names
for the corresponding structs.

Inner struct fields can either be a struct, pointer to a struct, or an embedded field.