package envvar

import (
	"os"
	"reflect"
	"regexp"
	"runtime"
	"testing"
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
	}
	testParse(t, vars, &typedVars{}, expected)
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
	}
	testParse(t, nil, &defaultVars{}, expected)
}

func TestParseCustomNameAndDefaultVal(t *testing.T) {
	expected := customNameAndDefaultVars{
		Foo: "biz",
	}
	testParse(t, nil, &customNameAndDefaultVars{}, expected)
}

func TestParseWithInvalidArgs(t *testing.T) {
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
}

type customNamedVars struct {
	Foo            string  `envvar:"FOO"`
	Bar            int     `envvar:"BAR"`
	MultiWord      float64 `envvar:"MULTI_WORD"`
	DifferentNames bool    `envvar:"COMPLETELY_DIFFERENT"`
}

type defaultVars struct {
	STRING  string  `default:"foo"`
	INT     int     `default:"272309480983"`
	INT8    int8    `default:"-4"`
	INT16   int16   `default:"15893"`
	INT32   int32   `default:"-230984"`
	INT64   int64   `default:"12"`
	UINT    uint    `default:"42"`
	UINT8   uint8   `default:"13"`
	UINT16  uint16  `default:"1337"`
	UINT32  uint32  `default:"348904"`
	UINT64  uint64  `default:"12093803"`
	FLOAT32 float32 `default:"0.001234"`
	FLOAT64 float64 `default:"23.7"`
	BOOL    bool    `default:"true"`
}

type customNameAndDefaultVars struct {
	Foo string `envvar:"BAR" default:"biz"`
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
