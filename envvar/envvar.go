// package envvar helps you manage environment variables. It maps environment
// variables to typed fields in a struct, and supports required and optional
// vars with defaults.
package envvar

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"syscall"
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
func Parse(v interface{}) error {
	// Make sure the type of v is what we expect.
	typ := reflect.TypeOf(v)
	if typ.Kind() != reflect.Ptr || typ.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("envvar: Error in Parse: type must be a pointer to a struct. Got: %T", v)
	}
	structType := typ.Elem()
	val := reflect.ValueOf(v)
	if val.IsNil() {
		return fmt.Errorf("envvar: Error in Parse: argument cannot be nil.")
	}
	structVal := val.Elem()
	// Iterate through the fields of v and set each field.
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		varName := field.Name
		customName := field.Tag.Get("envvar")
		if customName != "" {
			varName = customName
		}
		var varVal string
		defaultVal, foundDefault, err := getStructTag(field, "default")
		if err != nil {
			return err
		}
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
		if err := setFieldVal(structVal.Field(i), varName, varVal); err != nil {
			return err
		}
	}
	return nil
}

// UnsetVariableError is returned by Parse whenever a required environment
// variable is not set.
type UnsetVariableError struct {
	// VarName is the name of the required environment variable that was not set
	VarName string
}

// Error satisfies the error interface
func (e UnsetVariableError) Error() string {
	return fmt.Sprintf("envvar: Missing required environment variable: %s", e.VarName)
}

// getStructTag gets struct tag value with the given key. It differs from
// Tag.Get in that it has an additional return value that indicates whether or
// not the key was found in the struct tag. This is required to differentiate
// between a struct tag which specifies a default value of an empty string and
// a struct tag which does not specify a default value.
func getStructTag(field reflect.StructField, key string) (value string, found bool, err error) {
	buf := bytes.NewBufferString(string(field.Tag))
	for {
		// Read until we reach a colon. Whatever we scan is the key (plus the
		// colon).
		k, err := buf.ReadString(':')
		if err != nil {
			if err == io.EOF {
				return "", false, nil
			}
			return "", false, err
		}
		// Read until we reach the opening quotation mark. This is the start of
		// the value.
		if _, err := buf.ReadString('"'); err != nil {
			return "", false, fmt.Errorf("envvar: Invalid struct tag for field named %s. %s", field.Name, err.Error())
		}
		// Read until we reach the closing quotation mark. Whatever we scan is
		// the value (plus the closing quotation mark).
		val, err := buf.ReadString('"')
		if err != nil {
			return "", false, fmt.Errorf("envvar: Invalid struct tag for field named %s. %s", field.Name, err.Error())
		}
		// If k is equal to the key we were looking for, we have found the value.
		// Return it.
		if strings.TrimSpace(k[:len(k)-1]) == key {
			return val[:len(val)-1], true, nil
		}
	}
}

// setFieldVal first converts v to the type of structField, then uses reflection
// to set the field to the converted value.
func setFieldVal(structField reflect.Value, name string, v string) error {
	switch structField.Kind() {
	case reflect.String:
		structField.SetString(v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		vInt, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("envvar: Error parsing environment variable %s: %s\n%s", name, v, err.Error())
		}
		structField.SetInt(int64(vInt))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		vUint, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return fmt.Errorf("envvar: Error parsing environment variable %s: %s\n%s", name, v, err.Error())
		}
		structField.SetUint(uint64(vUint))
	case reflect.Float32, reflect.Float64:
		vFloat, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("envvar: Error parsing environment variable %s: %s\n%s", name, v, err.Error())
		}
		structField.SetFloat(vFloat)
	case reflect.Bool:
		vBool, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("envvar: Error parsing environment variable %s: %s\n%s", name, v, err.Error())
		}
		structField.SetBool(vBool)
	default:
		return fmt.Errorf("envvar: Unsupported struct field type: %s", structField.Type().String())
	}
	return nil
}
