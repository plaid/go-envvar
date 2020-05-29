package envvar

import (
	"fmt"
	"strings"
)

// UnsetVariableError is returned by Parse whenever a required environment
// variable is not set.
type UnsetVariableError struct {
	// VarName is the name of the required environment variable that was not set
	VarName string
}

// InvalidFieldError is returned by Parse whenever a given struct field
// is unsupported.
type InvalidFieldError struct {
	Name    string
	Message string
}

// InvalidVariableError is returned when a given env var cannot be parsed to
// a given struct field.
type InvalidVariableError struct {
	VarName  string
	VarValue string
	parent   error // optional
}

// InvalidArgumentError is raised when an invalid argument passed.
type InvalidArgumentError struct {
	message string
}

// ErrorList is list of independent errors raised by Parse
type ErrorList struct {
	Errors []error
}

func (e InvalidArgumentError) Error() string {
	return "envvar: " + e.message
}

// Error satisfies the error interface
func (e UnsetVariableError) Error() string {
	return fmt.Sprintf("Missing required environment variable: %s", e.VarName)
}

// Error satisfies the error interface
func (e InvalidVariableError) Error() string {
	return fmt.Sprintf("Error parsing environment variable %s: %s (%s)", e.VarName, e.VarValue, errorOrUnknown(e.parent))
}

func (e InvalidFieldError) Error() string {
	return fmt.Sprintf("Unsupported struct field %s: %s", e.Name, e.Message)

}
func errorOrUnknown(err error) string {
	if err != nil {
		return err.Error()
	}
	return "unknown"
}

func (e ErrorList) Error() string {
	var allErrors []string
	for _, err := range e.Errors {
		allErrors = append(allErrors, "envvar: "+err.Error())
	}
	return fmt.Sprint(strings.Join(allErrors, "\n"))
}
