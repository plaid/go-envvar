// package envvar helps you manage environment variables. It maps environment
// variables to typed fields in a struct, and supports required and optional
// vars with defaults.
package envvar

import (
	"fmt"
	"reflect"
	"strconv"
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
func Parse(v interface{}) error {
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
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		varName := field.Name
		customName := field.Tag.Get("envvar")
		if customName != "" {
			varName = customName
		}
		defaultVal := field.Tag.Get("default")
		varVal, found := syscall.Getenv(varName)
		if !found && defaultVal == "" {
			return fmt.Errorf("Missing required environment variable: %s", varName)
		}
		if defaultVal != "" && varVal == "" {
			varVal = defaultVal
		}
		if err := setFieldVal(structVal.Field(i), varName, varVal); err != nil {
			return err
		}
	}
	return nil
}

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
