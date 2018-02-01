// package envvar helps you manage environment variables. It maps environment
// variables to typed fields in a struct, and supports required and optional
// vars with defaults.
package envvar

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Parse parses environment variables into v, which must be a pointer to a
// struct. Because of the way envvar uses reflection, the fields in v must be
// exported, i.e. start with a capital letter. For each field in v, Parse will
// get the environment variable with the same name as the field and set the
// field to the value of that environment variable, converting it to the
// appropriate type if needed.
//
// Parse supports two struct tags, which can be used together or separately. The
// struct tag `envvar` can be used to specify the name of the environment
// variable that corresponds to a field. If the `envvar` struct tag is not
// provided, the default is to look for an environment variable with the same
// name as the field. The struct tag `default` can be used to set the default
// value for a field. The default value must be a string, but will be converted
// to match the type of the field as needed. If the `default` struct tag is not
// provided, the corresponding environment variable is required, and Parse will
// return an error if it is not defined. When the `default` struct tag is
// provided, the environment variable is considered optional, and if set, the
// value of the environment variable will override the default value.
//
// Parse will return an UnsetVariableError if a required environment variable
// was not set. It will also return an error if there was a problem converting
// environment variable values to the proper type or setting the fields of v.
//
// If a field of v implements the encoding.TextUnmarshaler interface, Parse will
// call the UnmarshalText method on the field in order to set its value.
func Parse(v interface{}) error {
	// Make sure the type of v is what we expect.
	typ := reflect.TypeOf(v)
	if typ.Kind() != reflect.Ptr || typ.Elem().Kind() != reflect.Struct {
		return InvalidArgumentError{fmt.Sprintf("Error in Parse: type must be a pointer to a struct. Got: %T", v)}
	}
	structType := typ.Elem()
	val := reflect.ValueOf(v)
	if val.IsNil() {
		return InvalidArgumentError{"Error in Parse: argument cannot be nil"}
	}
	errors := []error{}
	structVal := val.Elem()
	// Iterate through the fields of v and set each field.
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		fieldVal := structVal.Field(i)
		if err := parse(field, fieldVal); err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return ErrorList{errors}
	}
	return nil
}

func parse(field reflect.StructField, fieldVal reflect.Value) error {
	varName := field.Name
	customName := field.Tag.Get("envvar")
	if customName != "" {
		varName = customName
	}
	var varVal string
	defaultVal, foundDefault := field.Tag.Lookup("default")
	envVal, foundEnv := syscall.Getenv(varName)
	if foundEnv {
		// If we found an environment variable corresponding to this field. Use
		// the value of the environment variable. This overrides the default
		// (if any).
		varVal = envVal
	} else {
		if foundDefault {
			// If we did not find an environment variable corresponding to this
			// field, but there is a default value, use the default value.
			varVal = defaultVal
		} else {
			// If we did not find an environment variable corresponding to this
			// field and there is not a default value, we are missing a required
			// environment variable. Return an error.
			return UnsetVariableError{VarName: varName}
		}
	}
	// Set the value of the field.
	return setFieldVal(fieldVal, varName, varVal)
}

// UnsetVariableError is returned by Parse whenever a required environment
// variable is not set.
type UnsetVariableError struct {
	// VarName is the name of the required environment variable that was not set
	VarName string
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

func errorOrUnknown(err error) string {
	if err != nil {
		return err.Error()
	}
	return "unknown"
}

func (e ErrorList) Error() string {
	allErrors := []string{}
	for _, err := range e.Errors {
		allErrors = append(allErrors, "envvar: "+err.Error())
	}
	return fmt.Sprintf(strings.Join(allErrors, "\n"))
}

// setFieldVal first converts v to the type of structField, then uses reflection
// to set the field to the converted value.
func setFieldVal(structField reflect.Value, name string, v string) error {

	// Check if the struct field type implements the encoding.TextUnmarshaler
	// interface.
	if structField.Type().Implements(reflect.TypeOf([]encoding.TextUnmarshaler{}).Elem()) {
		// Call the UnmarshalText method using reflection.
		results := structField.MethodByName("UnmarshalText").Call([]reflect.Value{reflect.ValueOf([]byte(v))})
		if !results[0].IsNil() {
			err := results[0].Interface().(error)
			return InvalidVariableError{name, v, err}
		}
		return nil
	}

	// Check if *a pointer to* the struct field type implements the
	// encoding.TextUnmarshaler interface. If it does and the struct value is
	// addressable, call the UnmarshalText method using reflection.
	if reflect.PtrTo(structField.Type()).Implements(reflect.TypeOf([]encoding.TextUnmarshaler{}).Elem()) {
		// CanAddr tells us if reflect is able to get a pointer to the struct field
		// value. This should always be true, because the Parse method is strict
		// about accepting a pointer to a struct type. However, if it's not true the
		// Addr() call will panic, so it is good practice to leave this check in
		// place. (In the reflect package, a struct field is considered addressable
		// if we originally received a pointer to the struct type).
		if structField.CanAddr() {
			results := structField.Addr().MethodByName("UnmarshalText").Call([]reflect.Value{reflect.ValueOf([]byte(v))})
			if !results[0].IsNil() {
				err := results[0].Interface().(error)
				return InvalidVariableError{name, v, err}
			}
			return nil
		}
	}

	// If the field type does not implement the encoding.TextUnmarshaler
	// interface, we can try decoding some basic primitive types and setting the
	// value of the struct field with reflection.
	switch structField.Kind() {
	case reflect.String:
		structField.SetString(v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if structField.Type() == reflect.TypeOf(time.Duration(0)) {
			// special handling for duration types.
			dur, err := time.ParseDuration(v)
			if err != nil {
				return InvalidVariableError{name, v, err}
			}
			structField.SetInt(int64(dur))
		} else {
			vInt, err := strconv.Atoi(v)
			if err != nil {
				return InvalidVariableError{name, v, err}
			}
			structField.SetInt(int64(vInt))
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		vUint, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return InvalidVariableError{name, v, err}
		}
		structField.SetUint(uint64(vUint))
	case reflect.Float32, reflect.Float64:
		vFloat, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return InvalidVariableError{name, v, err}
		}
		structField.SetFloat(vFloat)
	case reflect.Bool:
		vBool, err := strconv.ParseBool(v)
		if err != nil {
			return InvalidVariableError{name, v, err}
		}
		structField.SetBool(vBool)
	default:
		return InvalidArgumentError{fmt.Sprintf("Unsupported struct field type: %s", structField.Type().String())}
	}
	return nil
}
