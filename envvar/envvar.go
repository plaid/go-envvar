// package envvar helps you manage environment variables. It maps environment variables
// to typed fields in a struct, and supports required and optional vars with defaults.
package envvar

import (
	"fmt"
	"reflect"
	"strconv"
	"syscall"
)

// Parse parses environment variables into v, which must be a pointer to a struct.
// For each field in v, Parse will get the environment variable with the same name
// as the field and set the field to the value of that environment variable, converting
// it to the appropriate type if needed.
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
		vString, found := syscall.Getenv(field.Name)
		if !found {
			return fmt.Errorf("Missing required environment variable: %s", field.Name)
		}
		if err := setFieldVal(structVal.Field(i), field.Name, vString); err != nil {
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
