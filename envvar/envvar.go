// package envvar helps you manage environment variables. It maps environment
// variables to typed fields in a struct, and supports required and optional
// vars with defaults.
package envvar

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"
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
	return ParseWithConfig(v, Config{Getenv: syscall.Getenv})
}

// Config is used to control the parsing behavior
// of the go-envvar package.
type Config struct {
	// Getenv is a custom function to retrieve envvars with.
	Getenv func(key string) (value string, found bool)
	// initial prefix to fetch envvars for.
	Prefix string
}

// GetenvFn is a custom function to retrieve envvars.
//
// given a key, it returns (value, true)
// if a given envvar exists, ("", false) otherwise.
// syscall.Getenv should satisfy this type signature.
type GetenvFn func(key string) (value string, found bool)

// ParseWithConfig is ...
func ParseWithConfig(v interface{}, config Config) error {
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
	structVal := val.Elem()
	if config.Getenv == nil {
		config.Getenv = syscall.Getenv
	}
	ss := structStack{config.Prefix, structType, structVal, &config}
	return ss.parseStruct()
}

// structStack represents the current instance of struct that the logic
// is injecting envvars into.
type structStack struct {
	envPrefix  string
	structType reflect.Type
	structVal  reflect.Value
	config     *Config
}

func (ss structStack) push(
	envPrefix string,
	structType reflect.Type,
	structVal reflect.Value,
) structStack {
	return structStack{
		envPrefix:  ss.envPrefix + envPrefix,
		structType: structType,
		structVal:  structVal,
		config:     ss.config,
	}
}

func (ss structStack) parseStruct() error {
	errors := []error{}
	// Iterate through the fields of v and set each field.
	for i := 0; i < ss.structType.NumField(); i++ {
		field := ss.structType.Field(i)
		fieldVal := ss.structVal.Field(i)
		if err := ss.parseField(field, fieldVal); err != nil {
			if suberrors, ok := err.(ErrorList); ok {
				errors = append(errors, suberrors.Errors...)
			} else {
				errors = append(errors, err)
			}
		}
	}
	if len(errors) > 0 {
		return ErrorList{errors}
	}
	return nil
}

func (ss structStack) parseField(field reflect.StructField, fieldVal reflect.Value) error {
	varName := field.Name
	customName := field.Tag.Get("envvar")
	if customName != "" {
		varName = customName
	}
	if success, _ := cleverMaybeTextUnmarshaler(fieldVal); !success {
		// subfield is a struct or pointer to a struct,
		// and does NOT implement TextUnmarshaller, so treat it
		// as a recursive inner struct.

		if fieldVal.Type().Kind() == reflect.Struct {
			newSS := ss.push(customName, field.Type, fieldVal)
			if err := foundDefaultTagError(field); err != nil {
				return err
			}
			return newSS.parseStruct()
		} else if fieldVal.Type().Kind() == reflect.Ptr && fieldVal.Type().Elem().Kind() == reflect.Struct {
			if fieldVal.IsNil() {
				fieldVal.Set(reflect.New(field.Type.Elem()))
			}
			if err := foundDefaultTagError(field); err != nil {
				return err
			}
			newSS := ss.push(customName, field.Type.Elem(), fieldVal.Elem())
			return newSS.parseStruct()
		}
	}

	var varVal string
	defaultVal, foundDefault := field.Tag.Lookup("default")
	derivedVarName := ss.envPrefix + varName
	envVal, foundEnv := ss.config.Getenv(derivedVarName)
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
			return UnsetVariableError{VarName: derivedVarName}
		}
	}
	// Set the value of the field.
	return setFieldVal(fieldVal, derivedVarName, varVal)
}

func foundDefaultTagError(field reflect.StructField) error {
	// struct fields do not support default tags.
	if _, foundDefault := field.Tag.Lookup("default"); foundDefault {
		return InvalidFieldError{
			Name:    field.Name,
			Message: "default tag is not supported for nested structs.",
		}
	}
	return nil
}

// determine whether a given reflect.Value is TextUnmarshaler, without
// doing something clever.
func maybeTextUnmarshaler(val reflect.Value) (bool, encoding.TextUnmarshaler) {
	if val.CanInterface() {
		casted, ok := val.Interface().(encoding.TextUnmarshaler)
		if !ok {
			return false, nil
		}
		return true, casted
	}
	return false, nil
}

// similar to maybeTextUnmarshaler, but attempt more clever things such as
// seeing the value as a pointer type.
func cleverMaybeTextUnmarshaler(structField reflect.Value) (bool, encoding.TextUnmarshaler) {
	// Check if the struct field type implements the encoding.TextUnmarshaler interface.
	if success, m := maybeTextUnmarshaler(structField); success {
		return true, m
	}
	// Check if *a pointer to* the struct field type implements the
	// encoding.TextUnmarshaler interface. If it does and the struct value is
	// addressable, call the UnmarshalText method using reflection.
	if structField.CanAddr() {
		if success, m := maybeTextUnmarshaler(structField.Addr()); success {
			return true, m
		}
	}
	return false, nil
}

// setUnmarshFieldVal sees whether a given field can be decoded via TextUnmarshaler interface.
// first bool determines whether the underlying type implements TextUnmarshaler
// and unmarshalling has been attempted.
func setUnmarshFieldVal(structField reflect.Value, name string, v string) (bool, error) {
	if success, m := cleverMaybeTextUnmarshaler(structField); success {
		err := m.UnmarshalText([]byte(v))
		if err != nil {
			return true, InvalidVariableError{name, v, err}
		}
		return true, nil
	}
	return false, nil
}

// setFieldVal first converts v to the type of structField, then uses reflection
// to set the field to the converted value.
func setFieldVal(structField reflect.Value, name string, v string) error {
	attempted, err := setUnmarshFieldVal(structField, name, v)
	if attempted {
		return err
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
		return InvalidFieldError{
			Name:    name,
			Message: fmt.Sprintf("Unsupported struct field type: %s", structField.Type().String()),
		}
	}
	return nil
}
