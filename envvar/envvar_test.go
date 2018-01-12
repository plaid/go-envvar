package envvar

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	vars := map[string]string{
		"STRING":  "foo",
		"INT":     "272309480983",
		"INT8":    "-4",
		"INT16":   "15893",
		"INT32":   "-230984",
		"INT64":   "12",
		"UINT":    "42",
		"UINT8":   "13",
		"UINT16":  "1337",
		"UINT32":  "348904",
		"UINT64":  "12093803",
		"FLOAT32": "0.001234",
		"FLOAT64": "23.7",
		"BOOL":    "true",
		"TIME":    "2017-10-31T14:18:00Z",
		"CUSTOM":  "foo,bar,baz",
		"WRAPPER": "a,b,c",
	}
	expected := typedVars{
		STRING:  "foo",
		INT:     272309480983,
		INT8:    -4,
		INT16:   15893,
		INT32:   -230984,
		INT64:   12,
		UINT:    42,
		UINT8:   13,
		UINT16:  1337,
		UINT32:  348904,
		UINT64:  12093803,
		FLOAT32: 0.001234,
		FLOAT64: 23.7,
		BOOL:    true,
		TIME:    time.Date(2017, 10, 31, 14, 18, 0, 0, time.UTC),
		CUSTOM: customUnmarshaler{
			strings: []string{"foo", "bar", "baz"},
		},
		WRAPPER: customUnmarshalerWrapper{
			um: &customUnmarshaler{
				strings: []string{"a", "b", "c"},
			},
		},
	}
	// Note that we have to initialize the WRAPPER type so that its field is
	// non-nil. No other types need to be initialized.
	holder := &typedVars{
		WRAPPER: customUnmarshalerWrapper{
			um: &customUnmarshaler{},
		},
	}
	testParse(t, vars, holder, expected)
}

func TestParseCustomNames(t *testing.T) {
	vars := map[string]string{
		"FOO":                  "foo",
		"BAR":                  "42",
		"MULTI_WORD":           "6.28318",
		"COMPLETELY_DIFFERENT": "t",
	}
	expected := customNamedVars{
		Foo:            "foo",
		Bar:            42,
		MultiWord:      6.28318,
		DifferentNames: true,
	}
	testParse(t, vars, &customNamedVars{}, expected)
}

func TestParseDefaultVals(t *testing.T) {
	expected := defaultVars{
		STRING:  "foo",
		INT:     272309480983,
		INT8:    -4,
		INT16:   15893,
		INT32:   -230984,
		INT64:   12,
		UINT:    42,
		UINT8:   13,
		UINT16:  1337,
		UINT32:  348904,
		UINT64:  12093803,
		FLOAT32: 0.001234,
		FLOAT64: 23.7,
		BOOL:    true,
		TIME:    time.Date(1992, 9, 29, 0, 0, 0, 0, time.UTC),
		CUSTOM: customUnmarshaler{
			strings: []string{"one", "two", "three"},
		},
		WRAPPER: customUnmarshalerWrapper{
			um: &customUnmarshaler{
				strings: []string{"apple", "banana", "cranberry"},
			},
		},
	}
	// Note that we have to initialize the WRAPPER type so that its field is
	// non-nil. No other types need to be initialized.
	holder := &defaultVars{
		WRAPPER: customUnmarshalerWrapper{
			um: &customUnmarshaler{},
		},
	}
	testParse(t, nil, holder, expected)
}

func TestParseCustomNameAndDefaultVal(t *testing.T) {
	expected := customNameAndDefaultVars{
		Foo: "biz",
	}
	testParse(t, nil, &customNameAndDefaultVars{}, expected)
}

func TestParseDefaultEmptyString(t *testing.T) {
	expected := defaultEmptyStringVars{
		Foo: "",
	}
	testParse(t, nil, &defaultEmptyStringVars{}, expected)
}

func TestParseRequiredVars(t *testing.T) {
	vars := typedVars{}
	err := Parse(&vars)
	if err == nil {
		t.Errorf("Expected error because required environment variables were not set, but got none.")
		return
	}
	errorList, ok := err.(ErrorList)
	if !ok {
		t.Errorf("Expected error type to be ErrorList but got %T", err)
		return
	}
	if len(errorList.Errors) == 0 {
		t.Errorf("Got zero ErrorList")
		return
	}
	for _, err := range errorList.Errors {
		if err == nil {
			t.Errorf("Got nil error from ErrorList")
		}
		if _, ok := err.(UnsetVariableError); !ok {
			t.Errorf("Expected UnsetVariableError but got %T", err)
		}
	}
}

func TestParseErrors(t *testing.T) {
	testCases := []struct {
		holder        interface{}
		expectedError string
	}{
		{
			holder:        (*typedVars)(nil),
			expectedError: "cannot be nil",
		},
		{
			holder:        "notAStruct",
			expectedError: "type must be a pointer to a struct",
		},
		{
			holder:        typedVars{},
			expectedError: "type must be a pointer to a struct",
		},
	}
	for i, testCase := range testCases {
		defer func(i int) {
			// We want to make sure Parse does not panic. If it does, mark the test case as
			// a failure and keep going instead of crashing.
			if r := recover(); r != nil {
				_, file, line, _ := runtime.Caller(4)
				t.Errorf("Panic at %s:%d:\n%s\nFor test case %d.\nHolder was: (%T) %+v", file, line, r, i, testCases[i].holder, testCases[i].holder)
			}
		}(i)
		err := Parse(testCase.holder)
		if testCase.expectedError != "" {
			if err == nil {
				t.Errorf("Expected error for test case %d but got none.\nHolder was: (%T) %+v", i, testCase.holder, testCase.holder)
			} else if !regexp.MustCompile(testCase.expectedError).Match([]byte(err.Error())) {
				t.Errorf("Expected error message to match `%s`\nbut got: %s", testCase.expectedError, err.Error())
			}
		}
	}
}

func TestSetFieldValErrorInt(t *testing.T) {
	var x = 3
	var xptr = &x
	value := reflect.ValueOf(xptr).Elem()
	expectInvalidVariableError(t, setFieldVal(value, "name", "abc"))
	if err := setFieldVal(value, "name", "15"); err != nil {
		t.Errorf("Unexpected error on setFieldVal(): %T", err)
	} else if x != 15 {
		t.Errorf("Expected value to be changed, but did not.")
	}
}
func TestSetFieldValErrorUint(t *testing.T) {
	var x = uint(3)
	var xptr = &x
	value := reflect.ValueOf(xptr).Elem()
	expectInvalidVariableError(t, setFieldVal(value, "name", "-3"))
	if err := setFieldVal(value, "name", "15"); err != nil {
		t.Errorf("Unexpected error on setFieldVal(): %T", err)
	} else if x != 15 {
		t.Errorf("Expected value to be changed, but did not.")
	}
}

func TestSetFieldValErrorFloat(t *testing.T) {
	var x = 3.2
	var xptr = &x
	value := reflect.ValueOf(xptr).Elem()
	expectInvalidVariableError(t, setFieldVal(value, "name", "abc"))
	if err := setFieldVal(value, "name", "42.3"); err != nil {
		t.Errorf("Unexpected error on setFieldVal(): %T", err)
	} else if x != 42.3 {
		t.Errorf("Expected value to be changed, but did not.")
	}
}
func TestSetFieldValErrorBool(t *testing.T) {
	var x = false
	var xptr = &x
	value := reflect.ValueOf(xptr).Elem()
	expectInvalidVariableError(t, setFieldVal(value, "name", "not-bool"))
	if err := setFieldVal(value, "name", "true"); err != nil {
		t.Errorf("Unexpected error on setFieldVal(): %T", err)
	} else if !x {
		t.Errorf("Expected value to be changed, but did not.")
	}
}

func TestErrorList(t *testing.T) {
	errorList := ErrorList{
		[]error{
			fmt.Errorf("First Error"),
			fmt.Errorf("Second Error"),
			fmt.Errorf("Third Error"),
		},
	}
	if errorList.Error() != `envvar: First Error
envvar: Second Error
envvar: Third Error` {
		t.Errorf("Error list's string representation is incorrect.")
	}
}

func expectInvalidVariableError(t *testing.T, err error) {
	if err == nil {
		t.Errorf("Expected InvalidVariableError, but got nil error")
	} else if _, ok := err.(InvalidVariableError); !ok {
		t.Errorf("Expected InvalidVariableError, but got %s", err.Error())
	}
}

func TestUnmarshalTextError(t *testing.T) {
	holder := &alwaysErrorVars{}
	err := setFieldVal(reflect.ValueOf(holder).Elem().Field(0), "alwaysError", "")
	if err == nil {
		t.Errorf("Expected InvalidVariableError, but got nil error")
	} else if _, ok := err.(InvalidVariableError); !ok {
		t.Errorf("Expected InvalidVariableError, but got %s", err.Error())
	}
}

func TestUnmarshalTextErrorPtr(t *testing.T) {
	holder := &alwaysErrorVarsPtr{}
	err := setFieldVal(reflect.ValueOf(holder).Elem().Field(0), "alwaysErrorPtr", "")
	if err == nil {
		t.Errorf("Expected InvalidVariableError, but got nil error")
	} else if _, ok := err.(InvalidVariableError); !ok {
		t.Errorf("Expected InvalidVariableError, but got %s", err.Error())
	}
}

// customUnmarshaler implements the UnmarshalText method.
type customUnmarshaler struct {
	strings []string
}

// UnmarshalText simply splits the text by the separator: ",".
func (cu *customUnmarshaler) UnmarshalText(text []byte) error {
	cu.strings = strings.Split(string(text), ",")
	return nil
}

// customUnmarshalerWrapper also implements the UnmarshalText method by calling
// it on its own *customUnmarshaler.
type customUnmarshalerWrapper struct {
	um *customUnmarshaler
}

// UnmarshalText simply calls um.UnmarshalText. Note that here we use a
// non-pointer receiver. It still works because the um field is a pointer. We
// just need to be sure to check if um is nil first.
func (cuw customUnmarshalerWrapper) UnmarshalText(text []byte) error {
	if cuw.um == nil {
		return nil
	}
	return cuw.um.UnmarshalText(text)
}

// alwaysErrorUnmarshaler implements the UnmarshalText method by always
// returning an error.
type alwaysErrorUnmarshaler struct{}

func (aeu alwaysErrorUnmarshaler) UnmarshalText(text []byte) error {
	return errors.New("this function always returns an error")
}

type alwaysErrorVars struct {
	AlwaysError alwaysErrorUnmarshaler
}

// alwaysErrorUnmarshalerPtr is like alwaysErrorUnmarshaler but implements
// the UnmarshalText method with a pointer receiver.
type alwaysErrorUnmarshalerPtr struct{}

func (aue *alwaysErrorUnmarshalerPtr) UnmarshalText(text []byte) error {
	return errors.New("this function always returns an error")
}

type alwaysErrorVarsPtr struct {
	AlwaysErrorPtr alwaysErrorUnmarshalerPtr
}

type typedVars struct {
	STRING  string
	INT     int
	INT8    int8
	INT16   int16
	INT32   int32
	INT64   int64
	UINT    uint
	UINT8   uint8
	UINT16  uint16
	UINT32  uint32
	UINT64  uint64
	FLOAT32 float32
	FLOAT64 float64
	BOOL    bool
	TIME    time.Time
	CUSTOM  customUnmarshaler
	WRAPPER customUnmarshalerWrapper
}

type customNamedVars struct {
	Foo            string  `envvar:"FOO"`
	Bar            int     `envvar:"BAR"`
	MultiWord      float64 `envvar:"MULTI_WORD"`
	DifferentNames bool    `envvar:"COMPLETELY_DIFFERENT"`
}

type defaultVars struct {
	STRING  string                   `default:"foo"`
	INT     int                      `default:"272309480983"`
	INT8    int8                     `default:"-4"`
	INT16   int16                    `default:"15893"`
	INT32   int32                    `default:"-230984"`
	INT64   int64                    `default:"12"`
	UINT    uint                     `default:"42"`
	UINT8   uint8                    `default:"13"`
	UINT16  uint16                   `default:"1337"`
	UINT32  uint32                   `default:"348904"`
	UINT64  uint64                   `default:"12093803"`
	FLOAT32 float32                  `default:"0.001234"`
	FLOAT64 float64                  `default:"23.7"`
	BOOL    bool                     `default:"true"`
	TIME    time.Time                `default:"1992-09-29T00:00:00Z"`
	CUSTOM  customUnmarshaler        `default:"one,two,three"`
	WRAPPER customUnmarshalerWrapper `default:"apple,banana,cranberry"`
}

type customNameAndDefaultVars struct {
	Foo string `envvar:"BAR" default:"biz"`
}

type defaultEmptyStringVars struct {
	Foo string `default:""`
}

func testParse(t *testing.T, vars map[string]string, holder interface{}, expected interface{}) {
	for name, val := range vars {
		if err := os.Setenv(name, val); err != nil {
			t.Fatalf("Problem setting env var: %s", err.Error())
		}
		defer func(name string) {
			if err := os.Unsetenv(name); err != nil {
				t.Fatalf("Problem unsetting env var: %s", err.Error())
			}
		}(name)
	}
	if err := Parse(holder); err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(reflect.ValueOf(holder).Elem().Interface(), expected) {
		t.Errorf("Parsed struct was incorrect.\nExpected: %+v\nBut got:  %+v", expected, holder)
	}
}
