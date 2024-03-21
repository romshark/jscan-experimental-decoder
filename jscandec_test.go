package jscandec_test

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"unsafe"

	jscandec "github.com/romshark/jscan-experimental-decoder"

	jsoniter "github.com/json-iterator/go"
	"github.com/romshark/jscan/v2"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/array_2d_bool_6m.json
var array2DBool6M string

//go:embed testdata/array_str_1024_639k.json
var arrayStr1024 string

//go:embed testdata/array_dec_1024_10k.json
var arrayFloat1024 string

type DecoderIface[S []byte | string, T any] interface {
	Decode(S, *T, *jscandec.DecodeOptions) (errIndex int, err error)
}

type testSetup[T any] struct {
	decodeOptions *jscandec.DecodeOptions
	decoderString DecoderIface[string, T]
	decoderBytes  DecoderIface[[]byte, T]
}

func newTestSetup[T any](
	t *testing.T, decodeOptions jscandec.DecodeOptions,
) testSetup[T] {
	tokenizerString := jscan.NewTokenizer[string](16, 1024)
	dStr, err := jscandec.NewDecoder[string, T](
		tokenizerString, jscandec.DefaultInitOptions,
	)
	require.NoError(t, err)
	tokenizerBytes := jscan.NewTokenizer[[]byte](16, 1024)
	dBytes, err := jscandec.NewDecoder[[]byte, T](
		tokenizerBytes, jscandec.DefaultInitOptions,
	)
	require.NoError(t, err)
	return testSetup[T]{
		decodeOptions: &decodeOptions,
		decoderString: dStr,
		decoderBytes:  dBytes,
	}
}

// TestOK makes sure that input can be decoded to T successfully,
// equals expect (if any) and yields the same results as encoding/json.Unmarshal.
// expect is optional.
func (s testSetup[T]) TestOK(t *testing.T, name, input string, expect ...T) {
	t.Helper()
	s.TestOKPrepare(t, name, input, Test[T]{})
}

type Test[T any] struct {
	// OptionsJscanInit is jscandec.DefaultInitOptions by default
	OptionsJscanInit *jscandec.InitOptions

	// OptionsJscanDecode is jscandec.DefaultOptions by default
	OptionsJscanDecode *jscandec.DecodeOptions

	// PrepareJscan is optional and produces the value passed to
	// `jscandec.(*Decoder).Decode`.
	PrepareJscan func() T

	// PrepareEncodingjson is optional and produces the value passed to
	// `encoding/json.(*Decoder).Decode` if not nil, otherwise the value of PrepareJscan
	// is used if PrepareEncodingjson == nil and PrepareJscan != nil.
	// If borh are nil, then the value passed will be a pointer to the zero value of T.
	PrepareEncodingjson func() any

	// Expect is optional. If Expect != nil then the jscan result is compared against it.
	Expect any

	// Check is optional and used to check the result of jscandec and compare its
	// result with the result of encoding/json. If Check is nil, then
	// vJscan and vEncodingJson are compared directly.
	Check func(t *testing.T, vJscan T, vEncodingJson any)
}

// TestOKPrepare is similar to TestOK but invokes prepareJscan on input variables
// before unmarshalling. If
func (s testSetup[T]) TestOKPrepare(t *testing.T, name, input string, test Test[T]) {
	t.Helper()
	check := func(t *testing.T, resultEncodingjson any, resultJscan *T) {
		t.Helper()
		{
			// Make sure that the result data for jscan and encoding/json
			// resides at different addresses because otherwise the comparison
			// could yield false positive results. The only exception are types
			// of size zero, such as empty struct and its derivatives.
			type emptyIface struct{ _, Data unsafe.Pointer }
			if (*emptyIface)(unsafe.Pointer(&resultEncodingjson)).Data == unsafe.Pointer(resultJscan) &&
				reflect.TypeOf(resultJscan).Elem().Size() != 0 {
				t.Fatal("the encoding/json and jscan variables " +
					"must reside at different addresses")
			}

		}

		// Make sure GC is happy
		runtime.GC()
		runtime.GC()

		if test.Expect != nil {
			require.Equal(t, test.Expect, *resultJscan, "unexpected jscan result")
		}
		if test.Check == nil {
			require.Equal(t, resultEncodingjson, resultJscan,
				"deviation between encoding/json and jscan results")
		} else {
			test.Check(t, *resultJscan, resultEncodingjson)
		}
	}
	prepare := func() (vJscan T, vEncodingJson any) {
		if test.PrepareJscan != nil {
			vJscan = test.PrepareJscan()
		}
		if test.PrepareEncodingjson != nil {
			vEncodingJson = test.PrepareEncodingjson()
		} else if test.PrepareJscan != nil {
			v := test.PrepareJscan()
			vEncodingJson = &v
		} else {
			vEncodingJson = &vJscan
		}
		return vJscan, vEncodingJson
	}
	t.Run(name+"/bytes", func(t *testing.T) {
		t.Helper()
		vJscan, vEncodingJson := prepare()

		dEncodingJson := json.NewDecoder(strings.NewReader(input))
		if s.decodeOptions.DisallowUnknownFields {
			dEncodingJson.DisallowUnknownFields()
		}
		require.NoError(t, dEncodingJson.Decode(vEncodingJson), "error in encoding/json")
		errIndex, err := s.decoderBytes.Decode([]byte(input), &vJscan, s.decodeOptions)
		if err != nil {
			t.Fatal(fmt.Errorf("ERR at %d: %s", errIndex, err.Error()))
		}
		check(t, vEncodingJson, &vJscan)
	})
	t.Run(name+"/string", func(t *testing.T) {
		t.Helper()
		vJscan, vEncodingJson := prepare()

		dEncodingJson := json.NewDecoder(strings.NewReader(input))
		if s.decodeOptions.DisallowUnknownFields {
			dEncodingJson.DisallowUnknownFields()
		}
		require.NoError(t, dEncodingJson.Decode(vEncodingJson), "error in encoding/json")
		errIndex, err := s.decoderString.Decode(input, &vJscan, s.decodeOptions)
		if err != nil {
			t.Fatal(fmt.Errorf("ERR at %d: %s", errIndex, err.Error()))
		}
		check(t, vEncodingJson, &vJscan)
	})
}

// testErr makes sure that input fails to parse for both jscan and encoding/json
// and that the returned error equals expect.
func (s testSetup[T]) testErr(
	t *testing.T, name, input string, expectIndex int, expect error,
) {
	t.Helper()
	s.testErrCheck(t, name, input, func(t *testing.T, errIndex int, err error) {
		t.Helper()
		require.Equal(t, expect, err)
		require.Equal(t, expectIndex, errIndex)
	})
}

// testErrCheck makes sure that input fails to parse for both jscan and encoding/json
// and calls check with the error returned.
func (s testSetup[T]) testErrCheck(
	t *testing.T, name, input string,
	check func(t *testing.T, errIndex int, err error),
) {
	t.Helper()
	t.Run(name+"/bytes", func(t *testing.T) {
		t.Helper()
		var std, v T
		stdDec := json.NewDecoder(strings.NewReader(input))
		if s.decodeOptions.DisallowUnknownFields {
			stdDec.DisallowUnknownFields()
		}
		require.Error(t, stdDec.Decode(&std), "no error in encoding/json")
		errIndex, err := s.decoderBytes.Decode([]byte(input), &v, s.decodeOptions)
		check(t, errIndex, err)
		runtime.GC() // Make sure GC is happy
	})
	t.Run(name+"/string", func(t *testing.T) {
		t.Helper()
		var std, v T
		stdDec := json.NewDecoder(strings.NewReader(input))
		if s.decodeOptions.DisallowUnknownFields {
			stdDec.DisallowUnknownFields()
		}
		require.Error(t, stdDec.Decode(&std), "no error in encoding/json")
		errIndex, err := s.decoderString.Decode(input, &v, s.decodeOptions)
		check(t, errIndex, err)
		runtime.GC() // Make sure GC is happy
	})
}

func TestDecodeNil(t *testing.T) {
	tokenizer := jscan.NewTokenizer[string](64, 1024)
	d, err := jscandec.NewDecoder[string, [][]bool](
		tokenizer, jscandec.DefaultInitOptions,
	)
	require.NoError(t, err)
	errIndex, err := d.Decode(`"foo"`, nil, jscandec.DefaultOptions)
	require.Error(t, err)
	require.Equal(t, jscandec.ErrNilDest, err)
	require.Equal(t, 0, errIndex)
}

func TestDecodeBool(t *testing.T) {
	s := newTestSetup[bool](t, *jscandec.DefaultOptions)
	s.TestOK(t, "true", `true`, true)
	s.TestOK(t, "false", `false`, false)

	s.testErr(t, "int", `1`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodeAny(t *testing.T) {
	s := newTestSetup[any](t, *jscandec.DefaultOptions)
	s.TestOK(t, "int_0", `0`, float64(0))
	s.TestOK(t, "int_42", `42`, float64(42))
	s.TestOK(t, "number", `3.1415`, float64(3.1415))
	s.TestOK(t, "true", `true`, true)
	s.TestOK(t, "false", `false`, false)
	s.TestOK(t, "string", `"string"`, "string")
	s.TestOK(t, "string_escaped", `"\"\u30C4\""`, `"ツ"`)
	s.TestOK(t, "null", `null`, nil)
	s.TestOK(t, "array_empty", `[]`, []any{})
	s.TestOK(t, "array_int", `[0,1,2]`, []any{float64(0), float64(1), float64(2)})
	s.TestOK(t, "array_string", `["a", "b", "\t"]`, []any{"a", "b", "\t"})
	s.TestOK(t, "array_bool", `[true, false]`, []any{true, false})
	s.TestOK(t, "array_null", `[null, null, null]`, []any{nil, nil, nil})
	s.TestOK(t, "object_empty", `{}`, map[string]any{})
	s.TestOK(t, "array_mix", `[null, false, 42, "x", {}, true]`,
		[]any{nil, false, float64(42), "x", map[string]any{}, true})
	s.TestOK(t, "object_multi", `{
		"num":         42,
		"str":         "\"text\u30C4\u044B\"",
		"bool_true":   true,
		"bool_false":  false,
		"array_empty": [],
		"array_mix":   [null, false, 42, "\/\r\n", {}, true],
		"null":        null
	}`, map[string]any{
		"num":         float64(42),
		"str":         `"textツы"`,
		"bool_true":   true,
		"bool_false":  false,
		"array_empty": []any{},
		"array_mix":   []any{nil, false, float64(42), "/\r\n", map[string]any{}, true},
		"null":        nil,
	})

	s.TestOKPrepare(t, "overwrite_int->bool", `true`, Test[any]{
		PrepareJscan: func() any { return int(42) },
		Expect:       true,
	})
	s.TestOKPrepare(t, "overwrite_bool->float64", `42`, Test[any]{
		PrepareJscan: func() any { return true },
		Expect:       float64(42),
	})

	{
		type S struct {
			ID   int
			Name string
		}
		s.TestOKPrepare(t, "overwrite_S->map", `{"id":42,"name":"John"}`, Test[any]{
			PrepareJscan: func() any { return S{ID: 1, Name: "Alice"} },
			Expect:       map[string]any{"id": float64(42), "name": "John"},
		})
		s.TestOKPrepare(t, "overwrite_slice", `42`, Test[any]{
			PrepareJscan: func() any {
				return []S{
					{ID: 1, Name: "Alice"}, {ID: 2, Name: "Bob"},
				}
			},
			Expect: float64(42),
		})

		s.TestOKPrepare(t, "overwrite_sliceS->map", `[{"name":"John"}]`, Test[any]{
			PrepareJscan: func() any {
				return []S{{ID: 1, Name: "Alice"}, {ID: 2, Name: "Bob"}}
			},
			Expect: []any{map[string]any{"name": "John"}},
		})
		s.TestOKPrepare(t, "no_overwrite_null", `null`, Test[any]{
			PrepareJscan: func() any {
				return []S{{ID: 1, Name: "Alice"}, {ID: 2, Name: "Bob"}}
			},
			// Check explicitly for any(nil) because otherwise
			// Test.Expect will be ignored.
			Check: func(t *testing.T, vJscan, vEncodingJson any) {
				require.Nil(t, vJscan)
				require.Equal(t, any(nil), *vEncodingJson.(*any))
			},
		})
	}

	s.testErrCheck(t, "float_range_hi", `1e309`,
		func(t *testing.T, errIndex int, err error) {
			require.ErrorIs(t, err, strconv.ErrRange)
			require.Equal(t, 0, errIndex)
		})
}

func TestDecodeUint(t *testing.T) {
	skipIfNot64bitSystem(t)
	s := newTestSetup[uint](t, *jscandec.DefaultOptions)
	s.TestOK(t, "0", `0`, 0)
	s.TestOK(t, "1", `1`, 1)
	s.TestOK(t, "int32_max", `2147483647`, math.MaxInt32)
	s.TestOK(t, "int64_max", `18446744073709551615`, math.MaxUint64)

	s.testErr(t, "overflow_hi", `18446744073709551616`, 0, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_l21", `111111111111111111111`, 0, jscandec.ErrIntegerOverflow)

	s.testErr(t, "negative", `-1`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "-1", `-1`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `0.1`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `1e2`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodeInt(t *testing.T) {
	skipIfNot64bitSystem(t)
	s := newTestSetup[int](t, *jscandec.DefaultOptions)
	s.TestOK(t, "0", `0`, 0)
	s.TestOK(t, "1", `1`, 1)
	s.TestOK(t, "-1", `-1`, -1)
	s.TestOK(t, "int32_min", `-2147483648`, math.MinInt32)
	s.TestOK(t, "int32_max", `2147483647`, math.MaxInt32)
	s.TestOK(t, "int64_min", `-9223372036854775808`, math.MinInt64)
	s.TestOK(t, "int64_max", `9223372036854775807`, math.MaxInt64)

	s.testErr(t, "overflow_hi", `9223372036854775808`, 0, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_lo", `-9223372036854775809`, 0, jscandec.ErrIntegerOverflow)

	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`, 0, jscandec.ErrUnexpectedValue)
	// s.testErr(t, "null", `null`, 0, jscandec.ErrUnexpectedValue)
	// s.testErr(t, "float", `0.1`, 0, jscandec.ErrUnexpectedValue)
	// s.testErr(t, "exponent", `1e2`, 0, ErrUnexpectedValue, Inde)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodeFloat32(t *testing.T) {
	s := newTestSetup[float32](t, *jscandec.DefaultOptions)
	s.TestOK(t, "0", `0`, 0)
	s.TestOK(t, "1", `1`, 1)
	s.TestOK(t, "-1", `-1`, -1)
	s.TestOK(t, "min_int", `-16777215`, -16_777_215)
	s.TestOK(t, "max_int", `16777215`, 16_777_215)
	s.TestOK(t, "pi7", `3.1415927`, 3.1415927)
	s.TestOK(t, "-pi7", `-3.1415927`, -3.1415927)
	s.TestOK(t, "3.4028235e38", `3.4028235e38`, 3.4028235e38)
	s.TestOK(t, "min_pos", `1.4e-45`, 1.4e-45)
	s.TestOK(t, "3.4e38", `3.4e38`, 3.4e38)
	s.TestOK(t, "-3.4e38", `-3.4e38`, -3.4e38)
	s.TestOK(t, "avogadros_num", `6.022e23`, 6.022e23)

	s.testErrCheck(t, "range_hi", `3.5e38`,
		func(t *testing.T, errIndex int, err error) {
			require.ErrorIs(t, err, strconv.ErrRange)
			require.Equal(t, 0, errIndex)
		})

	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodeFloat64(t *testing.T) {
	s := newTestSetup[float64](t, *jscandec.DefaultOptions)
	s.TestOK(t, "0", `0`, 0)
	s.TestOK(t, "1", `1`, 1)
	s.TestOK(t, "-1", `-1`, -1)
	s.TestOK(t, "1.0", `1.0`, 1)
	s.TestOK(t, "1.000000003", `1.000000003`, 1.000000003)
	s.TestOK(t, "max_int", `9007199254740991`, 9_007_199_254_740_991)
	s.TestOK(t, "min_int", `-9007199254740991`, -9_007_199_254_740_991)
	s.TestOK(t, "pi",
		`3.141592653589793238462643383279502884197`,
		3.141592653589793238462643383279502884197)
	s.TestOK(t, "pi_neg",
		`-3.141592653589793238462643383279502884197`,
		-3.141592653589793238462643383279502884197)
	s.TestOK(t, "3.4028235e38", `3.4028235e38`, 3.4028235e38)
	s.TestOK(t, "exponent", `1.7976931348623157e308`, 1.7976931348623157e308)
	s.TestOK(t, "neg_exponent", `1.7976931348623157e-308`, 1.7976931348623157e-308)
	s.TestOK(t, "1.4e-45", `1.4e-45`, 1.4e-45)
	s.TestOK(t, "neg_exponent", `-1.7976931348623157e308`, -1.7976931348623157e308)
	s.TestOK(t, "3.4e38", `3.4e38`, 3.4e38)
	s.TestOK(t, "-3.4e38", `-3.4e38`, -3.4e38)
	s.TestOK(t, "avogadros_num", `6.022e23`, 6.022e23)

	s.testErrCheck(t, "range_hi", `1e309`,
		func(t *testing.T, errIndex int, err error) {
			require.ErrorIs(t, err, strconv.ErrRange)
			require.Equal(t, 0, errIndex)
		})

	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodeUint64(t *testing.T) {
	s := newTestSetup[uint64](t, *jscandec.DefaultOptions)
	s.TestOK(t, "0", `0`, 0)
	s.TestOK(t, "1", `1`, 1)
	s.TestOK(t, "int32_max", `2147483647`, math.MaxInt32)
	s.TestOK(t, "uint32_max", `4294967295`, math.MaxUint32)
	s.TestOK(t, "max", `18446744073709551615`, math.MaxUint64)

	s.testErr(t, "overflow_hi", `18446744073709551616`, 0, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_hi2", `19000000000000000000`, 0, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_l21", `111111111111111111111`, 0, jscandec.ErrIntegerOverflow)

	s.testErr(t, "negative", `-1`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodeInt64(t *testing.T) {
	s := newTestSetup[int64](t, *jscandec.DefaultOptions)
	s.TestOK(t, "0", `0`, 0)
	s.TestOK(t, "1", `1`, 1)
	s.TestOK(t, "-1", `-1`, -1)
	s.TestOK(t, "int32_min", `-2147483648`, math.MinInt32)
	s.TestOK(t, "int32_max", `2147483647`, math.MaxInt32)
	s.TestOK(t, "min", `-9223372036854775808`, math.MinInt64)
	s.TestOK(t, "max", `9223372036854775807`, math.MaxInt64)

	s.testErr(t, "overflow_hi", `9223372036854775808`, 0, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_lo", `-9223372036854775809`, 0, jscandec.ErrIntegerOverflow)

	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodeUint32(t *testing.T) {
	s := newTestSetup[uint32](t, *jscandec.DefaultOptions)
	s.TestOK(t, "0", `0`, 0)
	s.TestOK(t, "1", `1`, 1)
	s.TestOK(t, "max", `4294967295`, math.MaxUint32)

	s.testErr(t, "overflow_hi", `4294967296`,
		0, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_hi2", `5000000000`, 0, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_l11", `11111111111`, 0, jscandec.ErrIntegerOverflow)

	s.testErr(t, "negative", `-1`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodeInt32(t *testing.T) {
	s := newTestSetup[int32](t, *jscandec.DefaultOptions)
	s.TestOK(t, "0", `0`, 0)
	s.TestOK(t, "1", `1`, 1)
	s.TestOK(t, "-1", `-1`, -1)
	s.TestOK(t, "min", `-2147483648`, math.MinInt32)
	s.TestOK(t, "max", `2147483647`, math.MaxInt32)

	s.testErr(t, "overflow_hi", `2147483648`, 0, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_lo", `-2147483649`, 0, jscandec.ErrIntegerOverflow)

	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodeUint16(t *testing.T) {
	s := newTestSetup[uint16](t, *jscandec.DefaultOptions)
	s.TestOK(t, "0", `0`, 0)
	s.TestOK(t, "1", `1`, 1)
	s.TestOK(t, "max", `65535`, math.MaxUint16)

	s.testErr(t, "overflow_hi", `65536`, 0, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_hi2", `70000`, 0, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_l6", `111111`, 0, jscandec.ErrIntegerOverflow)

	s.testErr(t, "negative", `-1`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodeInt16(t *testing.T) {
	s := newTestSetup[int16](t, *jscandec.DefaultOptions)
	s.TestOK(t, "0", `0`, 0)
	s.TestOK(t, "1", `1`, 1)
	s.TestOK(t, "-1", `-1`, -1)
	s.TestOK(t, "min", `-32768`, math.MinInt16)
	s.TestOK(t, "max", `32767`, math.MaxInt16)

	s.testErr(t, "overflow_hi", `32768`, 0, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_lo", `-32769`, 0, jscandec.ErrIntegerOverflow)

	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodeUint8(t *testing.T) {
	s := newTestSetup[uint8](t, *jscandec.DefaultOptions)
	s.TestOK(t, "0", `0`, 0)
	s.TestOK(t, "1", `1`, 1)
	s.TestOK(t, "max", `255`, math.MaxUint8)

	s.testErr(t, "overflow_hi", `256`, 0, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_hi2", `300`, 0, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_l4", `1111`, 0, jscandec.ErrIntegerOverflow)

	s.testErr(t, "negative", `-1`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodeInt8(t *testing.T) {
	s := newTestSetup[int8](t, *jscandec.DefaultOptions)
	s.TestOK(t, "0", `0`, 0)
	s.TestOK(t, "1", `1`, 1)
	s.TestOK(t, "-1", `-1`, -1)
	s.TestOK(t, "min", `-128`, math.MinInt8)
	s.TestOK(t, "max", `127`, math.MaxInt8)

	s.testErr(t, "overflow_hi", `128`, 0, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_lo", `-129`, 0, jscandec.ErrIntegerOverflow)

	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodeNull(t *testing.T) {
	// Primitive types
	newTestSetup[string](t, *jscandec.DefaultOptions).TestOK(t, "string", `null`, "")
	newTestSetup[bool](t, *jscandec.DefaultOptions).TestOK(t, "bool", `null`, false)
	newTestSetup[int](t, *jscandec.DefaultOptions).TestOK(t, "int", `null`, 0)
	newTestSetup[int8](t, *jscandec.DefaultOptions).TestOK(t, "int8", `null`, 0)
	newTestSetup[int16](t, *jscandec.DefaultOptions).TestOK(t, "int16", `null`, 0)
	newTestSetup[int32](t, *jscandec.DefaultOptions).TestOK(t, "int32", `null`, 0)
	newTestSetup[int64](t, *jscandec.DefaultOptions).TestOK(t, "int64", `null`, 0)
	newTestSetup[uint](t, *jscandec.DefaultOptions).TestOK(t, "uint", `null`, 0)
	newTestSetup[uint8](t, *jscandec.DefaultOptions).TestOK(t, "uint8", `null`, 0)
	newTestSetup[uint16](t, *jscandec.DefaultOptions).TestOK(t, "uint16", `null`, 0)
	newTestSetup[uint32](t, *jscandec.DefaultOptions).TestOK(t, "uint32", `null`, 0)
	newTestSetup[uint64](t, *jscandec.DefaultOptions).TestOK(t, "uint64", `null`, 0)
	newTestSetup[float32](t, *jscandec.DefaultOptions).TestOK(t, "float32", `null`, 0)
	newTestSetup[float64](t, *jscandec.DefaultOptions).TestOK(t, "float64", `null`, 0)

	// Slices
	newTestSetup[[]bool](t, *jscandec.DefaultOptions).
		TestOK(t, "slice_bool", `null`, nil)
	newTestSetup[[]string](t, *jscandec.DefaultOptions).
		TestOK(t, "slice_string", `null`, nil)

	// Primitives in array
	newTestSetup[[]bool](t, *jscandec.DefaultOptions).
		TestOK(t, "array_bool", `[null]`, []bool{false})
	newTestSetup[[]string](t, *jscandec.DefaultOptions).
		TestOK(t, "array_string", `[null]`, []string{""})
	newTestSetup[[]int](t, *jscandec.DefaultOptions).
		TestOK(t, "array_int", `[null]`, []int{0})
	newTestSetup[[]int8](t, *jscandec.DefaultOptions).
		TestOK(t, "array_int8", `[null]`, []int8{0})
	newTestSetup[[]int16](t, *jscandec.DefaultOptions).
		TestOK(t, "array_int16", `[null]`, []int16{0})
	newTestSetup[[]int32](t, *jscandec.DefaultOptions).
		TestOK(t, "array_int32", `[null]`, []int32{0})
	newTestSetup[[]int64](t, *jscandec.DefaultOptions).
		TestOK(t, "array_int64", `[null]`, []int64{0})
	newTestSetup[[]uint](t, *jscandec.DefaultOptions).
		TestOK(t, "array_int", `[null]`, []uint{0})
	newTestSetup[[]uint8](t, *jscandec.DefaultOptions).
		TestOK(t, "array_int8", `[null]`, []uint8{0})
	newTestSetup[[]uint16](t, *jscandec.DefaultOptions).
		TestOK(t, "array_int16", `[null]`, []uint16{0})
	newTestSetup[[]uint32](t, *jscandec.DefaultOptions).
		TestOK(t, "array_int32", `[null]`, []uint32{0})
	newTestSetup[[]uint64](t, *jscandec.DefaultOptions).
		TestOK(t, "array_int64", `[null]`, []uint64{0})
	newTestSetup[[]float32](t, *jscandec.DefaultOptions).
		TestOK(t, "array_float32", `[null]`, []float32{0})
	newTestSetup[[]float64](t, *jscandec.DefaultOptions).
		TestOK(t, "array_float64", `[null]`, []float64{0})
}

func TestDecodeString(t *testing.T) {
	s := newTestSetup[string](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `""`, "")
	s.TestOK(t, "spaces", `"   "`, "   ")
	s.TestOK(t, "hello_world", `"Hello World!"`, "Hello World!")
	s.TestOK(t, "unicode", `"юникод-жеж"`, "юникод-жеж")
	s.TestOK(t, "escaped", `"\"\\\""`, `"\"`)
	s.TestOK(t, "escaped_unicode", `"\u0436\u0448\u0444\u30C4"`, `жшфツ`)
}

func TestDecode2DSliceBool(t *testing.T) {
	type T = [][]bool
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "3_2", `[[true],[false, true],[ ]]`, T{{true}, {false, true}, {}})
	s.TestOK(t, "2_1", `[[],[false]]`, T{{}, {false}})
	s.TestOK(t, "2_0", `[[],[]]`, T{{}, {}})
	s.TestOK(t, "1", `[]`, T{})
	s.TestOK(t, "array_2d_bool_6m", array2DBool6M)
}

func TestDecodeSliceInt(t *testing.T) {
	skipIfNot64bitSystem(t)

	type T = []int
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "three_items", `[ 1, -23, 456 ]`, T{1, -23, 456})
	s.TestOK(t, "max_int", `[9223372036854775807]`, T{math.MaxInt})
	s.TestOK(t, "min_int", `[-9223372036854775808]`, T{math.MinInt})
	s.TestOK(t, "one_item", `[ 1 ]`, T{1})
	s.TestOK(t, "empty", `[]`, T{})
	s.TestOK(t, "null", `null`, T(nil))
	s.TestOK(t, "null_element", `[ null ]`, T{0})
	s.TestOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.TestOKPrepare(t, "var_overwrite", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int { return []int{10, 20, 30} },
		Expect:       []int{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_nil_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int { return []int(nil) },
		Expect:       []int{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int { return []int{10, 20} },
		Expect:       []int{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_shrink", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int { return []int{10, 20, 30, 40} },
		Expect:       []int{1, 2, 3},
	})

	s.testErr(t, "overflow_hi", `[9223372036854775808]`,
		1, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_lo", `[-9223372036854775809]`,
		1, jscandec.ErrIntegerOverflow)
	s.testErr(t, "wrong_type_object", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		3, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		3, jscandec.ErrUnexpectedValue)
}

func TestDecodeSliceInt8(t *testing.T) {
	type T = []int8
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "three_items", `[ 1, -23, 123 ]`, T{1, -23, 123})
	s.TestOK(t, "max_int8", `[127]`, T{math.MaxInt8})
	s.TestOK(t, "min_int8", `[-128]`, T{math.MinInt8})
	s.TestOK(t, "one_item", `[ 1 ]`, T{1})
	s.TestOK(t, "empty", `[]`, T{})
	s.TestOK(t, "null", `null`, T(nil))
	s.TestOK(t, "null_element", `[ null ]`, T{0})
	s.TestOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.TestOKPrepare(t, "var_overwrite", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int8 { return []int8{10, 20, 30} },
		Expect:       []int8{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_nil_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int8 { return []int8(nil) },
		Expect:       []int8{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int8 { return []int8{10, 20} },
		Expect:       []int8{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_shrink", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int8 { return []int8{10, 20, 30, 40} },
		Expect:       []int8{1, 2, 3},
	})

	s.testErr(t, "overflow_hi", `[128]`,
		1, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_lo", `[-129]`,
		1, jscandec.ErrIntegerOverflow)
	s.testErr(t, "wrong_type_object", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		3, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		3, jscandec.ErrUnexpectedValue)
}

func TestDecodeSliceInt16(t *testing.T) {
	type T = []int16
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "three_items", `[ 1, -23, 123 ]`, T{1, -23, 123})
	s.TestOK(t, "max_int16", `[32767]`, T{math.MaxInt16})
	s.TestOK(t, "min_int16", `[-32768]`, T{math.MinInt16})
	s.TestOK(t, "one_item", `[ 1 ]`, T{1})
	s.TestOK(t, "empty", `[]`, T{})
	s.TestOK(t, "null", `null`, T(nil))
	s.TestOK(t, "null_element", `[ null ]`, T{0})
	s.TestOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.TestOKPrepare(t, "var_overwrite", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int16 { return []int16{10, 20, 30} },
		Expect:       []int16{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_nil_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int16 { return []int16(nil) },
		Expect:       []int16{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int16 { return []int16{10, 20} },
		Expect:       []int16{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_shrink", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int16 { return []int16{10, 20, 30, 40} },
		Expect:       []int16{1, 2, 3},
	})

	s.testErr(t, "overflow_hi", `[32768]`,
		1, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_lo", `[-32769]`,
		1, jscandec.ErrIntegerOverflow)
	s.testErr(t, "wrong_type_object", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		3, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		3, jscandec.ErrUnexpectedValue)
}

func TestDecodeSliceInt32(t *testing.T) {
	type T = []int32
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "three_items", `[ 1, -23, 123 ]`, T{1, -23, 123})
	s.TestOK(t, "max_int32", `[2147483647]`, T{math.MaxInt32})
	s.TestOK(t, "min_int32", `[-2147483648]`, T{math.MinInt32})
	s.TestOK(t, "one_item", `[ 1 ]`, T{1})
	s.TestOK(t, "empty", `[]`, T{})
	s.TestOK(t, "null", `null`, T(nil))
	s.TestOK(t, "null_element", `[ null ]`, T{0})
	s.TestOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.TestOKPrepare(t, "var_overwrite", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int32 { return []int32{10, 20, 30} },
		Expect:       []int32{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_nil_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int32 { return []int32(nil) },
		Expect:       []int32{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int32 { return []int32{10, 20} },
		Expect:       []int32{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_shrink", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int32 { return []int32{10, 20, 30, 40} },
		Expect:       []int32{1, 2, 3},
	})

	s.testErr(t, "overflow_hi", `[2147483648]`,
		1, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_lo", `[-2147483649]`,
		1, jscandec.ErrIntegerOverflow)
	s.testErr(t, "wrong_type_object", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		3, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		3, jscandec.ErrUnexpectedValue)
}

func TestDecodeSliceInt64(t *testing.T) {
	type T = []int64
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "three_items", `[ 1, -23, 123 ]`, T{1, -23, 123})
	s.TestOK(t, "max_int64", `[9223372036854775807]`, T{math.MaxInt64})
	s.TestOK(t, "min_int64", `[-9223372036854775808]`, T{math.MinInt64})
	s.TestOK(t, "one_item", `[ 1 ]`, T{1})
	s.TestOK(t, "empty", `[]`, T{})
	s.TestOK(t, "null", `null`, T(nil))
	s.TestOK(t, "null_element", `[ null ]`, T{0})
	s.TestOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.TestOKPrepare(t, "var_overwrite", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int64 { return []int64{10, 20, 30} },
		Expect:       []int64{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_nil_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int64 { return []int64(nil) },
		Expect:       []int64{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int64 { return []int64{10, 20} },
		Expect:       []int64{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_shrink", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []int64 { return []int64{10, 20, 30, 40} },
		Expect:       []int64{1, 2, 3},
	})

	s.testErr(t, "overflow_hi", `[9223372036854775808]`,
		1, jscandec.ErrIntegerOverflow)
	s.testErr(t, "overflow_lo", `[-9223372036854775809]`,
		1, jscandec.ErrIntegerOverflow)
	s.testErr(t, "wrong_type_object", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		3, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		3, jscandec.ErrUnexpectedValue)
}

func TestDecodeSliceUint(t *testing.T) {
	skipIfNot64bitSystem(t)

	type T = []uint
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "three_items", `[ 1, 23, 456 ]`, T{1, 23, 456})
	s.TestOK(t, "max_uint", `[18446744073709551615]`, T{math.MaxUint})
	s.TestOK(t, "one_item", `[ 1 ]`, T{1})
	s.TestOK(t, "empty", `[]`, T{})
	s.TestOK(t, "null", `null`, T(nil))
	s.TestOK(t, "null_element", `[ null ]`, T{0})
	s.TestOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.TestOKPrepare(t, "var_overwrite", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint { return []uint{10, 20, 30} },
		Expect:       []uint{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_nil_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint { return []uint(nil) },
		Expect:       []uint{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint { return []uint{10, 20} },
		Expect:       []uint{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_shrink", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint { return []uint{10, 20, 30, 40} },
		Expect:       []uint{1, 2, 3},
	})

	s.testErr(t, "negative", `[-1]`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "overflow_hi", `[18446744073709551616]`,
		1, jscandec.ErrIntegerOverflow)
	s.testErr(t, "wrong_type_object", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		3, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		3, jscandec.ErrUnexpectedValue)
}

func TestDecodeSliceUint8(t *testing.T) {
	skipIfNot64bitSystem(t)

	type T = []uint8
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "three_items", `[ 1, 23, 123 ]`, T{1, 23, 123})
	s.TestOK(t, "max_uint", `[255]`, T{math.MaxUint8})
	s.TestOK(t, "one_item", `[ 1 ]`, T{1})
	s.TestOK(t, "empty", `[]`, T{})
	s.TestOK(t, "null", `null`, T(nil))
	s.TestOK(t, "null_element", `[ null ]`, T{0})
	s.TestOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.TestOKPrepare(t, "var_overwrite", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint8 { return []uint8{10, 20, 30} },
		Expect:       []uint8{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_nil_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint8 { return []uint8(nil) },
		Expect:       []uint8{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint8 { return []uint8{10, 20} },
		Expect:       []uint8{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_shrink", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint8 { return []uint8{10, 20, 30, 40} },
		Expect:       []uint8{1, 2, 3},
	})

	s.testErr(t, "negative", `[-1]`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "overflow_hi", `[256]`,
		1, jscandec.ErrIntegerOverflow)
	s.testErr(t, "wrong_type_object", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		3, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		3, jscandec.ErrUnexpectedValue)
}

func TestDecodeSliceUint16(t *testing.T) {
	skipIfNot64bitSystem(t)

	type T = []uint16
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "three_items", `[ 1, 23, 123 ]`, T{1, 23, 123})
	s.TestOK(t, "max_uint", `[65535]`, T{math.MaxUint16})
	s.TestOK(t, "one_item", `[ 1 ]`, T{1})
	s.TestOK(t, "empty", `[]`, T{})
	s.TestOK(t, "null", `null`, T(nil))
	s.TestOK(t, "null_element", `[ null ]`, T{0})
	s.TestOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.TestOKPrepare(t, "var_overwrite", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint16 { return []uint16{10, 20, 30} },
		Expect:       []uint16{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_nil_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint16 { return []uint16(nil) },
		Expect:       []uint16{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_nil_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint16 { return []uint16{10, 20} },
		Expect:       []uint16{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_shrink", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint16 { return []uint16{10, 20, 30, 40} },
		Expect:       []uint16{1, 2, 3},
	})

	s.testErr(t, "negative", `[-1]`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "overflow_hi", `[65536]`,
		1, jscandec.ErrIntegerOverflow)
	s.testErr(t, "wrong_type_object", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		3, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		3, jscandec.ErrUnexpectedValue)
}

func TestDecodeSliceUint32(t *testing.T) {
	skipIfNot64bitSystem(t)

	type T = []uint32
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "three_items", `[ 1, 23, 123 ]`, T{1, 23, 123})
	s.TestOK(t, "max_uint", `[4294967295]`, T{math.MaxUint32})
	s.TestOK(t, "one_item", `[ 1 ]`, T{1})
	s.TestOK(t, "empty", `[]`, T{})
	s.TestOK(t, "null", `null`, T(nil))
	s.TestOK(t, "null_element", `[ null ]`, T{0})
	s.TestOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.TestOKPrepare(t, "var_overwrite", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint32 { return []uint32{10, 20, 30} },
		Expect:       []uint32{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_nil_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint32 { return []uint32(nil) },
		Expect:       []uint32{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint32 { return []uint32{10, 20} },
		Expect:       []uint32{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_shrink", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint32 { return []uint32{10, 20, 30, 40} },
		Expect:       []uint32{1, 2, 3},
	})

	s.testErr(t, "negative", `[-1]`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "overflow_hi", `[4294967296]`,
		1, jscandec.ErrIntegerOverflow)
	s.testErr(t, "wrong_type_object", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		3, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		3, jscandec.ErrUnexpectedValue)
}

func TestDecodeSliceUint64(t *testing.T) {
	skipIfNot64bitSystem(t)

	type T = []uint64
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "three_items", `[ 1, 23, 123 ]`, T{1, 23, 123})
	s.TestOK(t, "max_uint", `[18446744073709551615]`, T{math.MaxUint64})
	s.TestOK(t, "one_item", `[ 1 ]`, T{1})
	s.TestOK(t, "empty", `[]`, T{})
	s.TestOK(t, "null", `null`, T(nil))
	s.TestOK(t, "null_element", `[ null ]`, T{0})
	s.TestOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.TestOKPrepare(t, "var_overwrite", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint64 { return []uint64{10, 20, 30} },
		Expect:       []uint64{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_nil_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint64 { return []uint64(nil) },
		Expect:       []uint64{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_realloc", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint64 { return []uint64{10, 20} },
		Expect:       []uint64{1, 2, 3},
	})
	s.TestOKPrepare(t, "var_shrink", `[1,2,3]`, Test[T]{
		PrepareJscan: func() []uint64 { return []uint64{10, 20, 30, 40} },
		Expect:       []uint64{1, 2, 3},
	})

	s.testErr(t, "negative", `[-1]`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "overflow_hi", `[18446744073709551616]`,
		1, jscandec.ErrIntegerOverflow)
	s.testErr(t, "wrong_type_object", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		3, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		3, jscandec.ErrUnexpectedValue)
}

func TestDecodeSliceString(t *testing.T) {
	type T = []string
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "three_items", `[ "a", "", "cde" ]`, T{"a", "", "cde"})
	s.TestOK(t, "one_items", `[ "abc" ]`, T{"abc"})
	s.TestOK(t, "escaped", `[ "\"abc\tdef\"" ]`, T{"\"abc\tdef\""})
	s.TestOK(t, "unicode", `["жзш","ツ!"]`, T{`жзш`, `ツ!`})
	s.TestOK(t, "empty", `[]`, T{})
	s.TestOK(t, "null", `null`, T(nil))
	s.TestOK(t, "null_element", `[null]`, T{""})
	s.TestOK(t, "null_element_multi", `[null,"okay",null]`, T{"", "okay", ""})
	s.TestOK(t, "array_str_1024_639k", arrayStr1024)

	s.TestOKPrepare(t, "var_overwrite", `["x","y","z"]`, Test[T]{
		PrepareJscan: func() []string { return []string{"a", "b", "c"} },
		Expect:       []string{"x", "y", "z"},
	})
	s.TestOKPrepare(t, "var_nil_realloc", `["x","y","z"]`, Test[T]{
		PrepareJscan: func() []string { return []string(nil) },
		Expect:       []string{"x", "y", "z"},
	})
	s.TestOKPrepare(t, "var_realloc", `["x","y","z"]`, Test[T]{
		PrepareJscan: func() []string { return []string{"a", "b"} },
		Expect:       []string{"x", "y", "z"},
	})
	s.TestOKPrepare(t, "var_overwrite", `["x","y","z"]`, Test[T]{
		PrepareJscan: func() []string { return []string{"a", "b", "c", "d"} },
		Expect:       []string{"x", "y", "z"},
	})

	s.testErr(t, "wrong_type_string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_true", `true`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_false", `false`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_object", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_array", `["okay",[]]`,
		8, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_float", `["okay",3.14]`,
		8, jscandec.ErrUnexpectedValue)
}

func TestDecodeSliceBool(t *testing.T) {
	type T = []bool
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "three_items", `[ true, false, true ]`, T{true, false, true})
	s.TestOK(t, "one_items", `[ true ]`, T{true})
	s.TestOK(t, "empty", `[]`, T{})
	s.TestOK(t, "null", `null`, T(nil))
	s.TestOK(t, "null_element", `[null]`, T{false})
	s.TestOK(t, "null_element_multi", `[null,true,null]`, T{false, true, false})

	s.TestOKPrepare(t, "var_overwrite", `[true,false,true]`, Test[T]{
		PrepareJscan: func() []bool { return []bool{false, true, false} },
		Expect:       []bool{true, false, true},
	})
	s.TestOKPrepare(t, "var_nil_realloc", `[true, false, true]`, Test[T]{
		PrepareJscan: func() []bool { return []bool(nil) },
		Expect:       []bool{true, false, true},
	})
	s.TestOKPrepare(t, "var_realloc", `[true, false, true]`, Test[T]{
		PrepareJscan: func() []bool { return []bool{false, false} },
		Expect:       []bool{true, false, true},
	})
	s.TestOKPrepare(t, "var_shrink", `[true, false, true]`, Test[T]{
		PrepareJscan: func() []bool { return []bool{false, true, true, true} },
		Expect:       []bool{true, false, true},
	})

	s.testErr(t, "wrong_type_string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_true", `true`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_false", `false`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_object", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_array", `[true,[]]`,
		6, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_float", `[true,3.14]`,
		6, jscandec.ErrUnexpectedValue)
}

func TestDecodeSliceFloat32(t *testing.T) {
	type T = []float32
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "three_items", `[0, 3.14, 2.5]`, T{0, 3.14, 2.5})
	s.TestOK(t, "null", `null`, T(nil))
	s.TestOK(t, "empty", `[]`, T{})
	s.TestOK(t, "zero", `[0]`, T{0})
	s.TestOK(t, "1", `[1]`, T{1})
	s.TestOK(t, "-1", `[-1]`, T{-1})
	s.TestOK(t, "min_int", `[-16777215]`, T{-16_777_215})
	s.TestOK(t, "max_int", `[16777215]`, T{16_777_215})
	s.TestOK(t, "pi7", `[3.1415927]`, T{3.1415927})
	s.TestOK(t, "-pi7", `[-3.1415927]`, T{-3.1415927})
	s.TestOK(t, "3.4028235e38", `[3.4028235e38]`, T{3.4028235e38})
	s.TestOK(t, "min_pos", `[1.4e-45]`, T{1.4e-45})
	s.TestOK(t, "3.4e38", `[3.4e38]`, T{3.4e38})
	s.TestOK(t, "-3.4e38", `[-3.4e38]`, T{-3.4e38})
	s.TestOK(t, "avogadros_num", `[6.022e23]`, T{6.022e23})

	s.TestOKPrepare(t, "var_overwrite", `[1.1, 2.2, 3.3]`, Test[T]{
		PrepareJscan: func() []float32 { return []float32{10.1, 20.2, 30.3} },
		Expect:       []float32{1.1, 2.2, 3.3},
	})
	s.TestOKPrepare(t, "var_nil_realloc", `[1.1, 2.2, 3.3]`, Test[T]{
		PrepareJscan: func() []float32 { return []float32(nil) },
		Expect:       []float32{1.1, 2.2, 3.3},
	})
	s.TestOKPrepare(t, "var_realloc", `[1.1, 2.2, 3.3]`, Test[T]{
		PrepareJscan: func() []float32 { return []float32{10.1, 20.2} },
		Expect:       []float32{1.1, 2.2, 3.3},
	})
	s.TestOKPrepare(t, "var_shrink", `[1.1, 2.2, 3.3]`, Test[T]{
		PrepareJscan: func() []float32 { return []float32{10.1, 20.2, 30.3, 40.4} },
		Expect:       []float32{1.1, 2.2, 3.3},
	})

	s.testErrCheck(t, "range_hi", `[3.5e38]`,
		func(t *testing.T, errIndex int, err error) {
			require.ErrorIs(t, err, strconv.ErrRange)
			require.Equal(t, 1, errIndex)
		})

	s.testErr(t, "wrong_type_string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_true", `true`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_false", `false`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_object", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_array", `[1,[]]`,
		3, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		3, jscandec.ErrUnexpectedValue)
}

func TestDecodeSliceFloat64(t *testing.T) {
	type T = []float64
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "three_items", `[0, 3.14, 2.5]`, T{0, 3.14, 2.5})
	s.TestOK(t, "null", `null`, T(nil))
	s.TestOK(t, "empty", `[]`, T{})
	s.TestOK(t, "0", `[0]`, T{0})
	s.TestOK(t, "1", `[1]`, T{1})
	s.TestOK(t, "-1", `[-1]`, T{-1})
	s.TestOK(t, "1.0", `[1.0]`, T{1})
	s.TestOK(t, "1.000000003", `[1.000000003]`, T{1.000000003})
	s.TestOK(t, "max_int", `[9007199254740991]`, T{9_007_199_254_740_991})
	s.TestOK(t, "min_int", `[-9007199254740991]`, T{-9_007_199_254_740_991})
	s.TestOK(t, "pi",
		`[3.141592653589793238462643383279502884197]`,
		T{3.141592653589793238462643383279502884197})
	s.TestOK(t, "pi_neg",
		`[-3.141592653589793238462643383279502884197]`,
		T{-3.141592653589793238462643383279502884197})
	s.TestOK(t, "3.4028235e38", `[3.4028235e38]`, T{3.4028235e38})
	s.TestOK(t, "exponent", `[1.7976931348623157e308]`, T{1.7976931348623157e308})
	s.TestOK(t, "neg_exponent", `[1.7976931348623157e-308]`, T{1.7976931348623157e-308})
	s.TestOK(t, "1.4e-45", `[1.4e-45]`, T{1.4e-45})
	s.TestOK(t, "neg_exponent", `[-1.7976931348623157e308]`, T{-1.7976931348623157e308})
	s.TestOK(t, "3.4e38", `[3.4e38]`, T{3.4e38})
	s.TestOK(t, "-3.4e38", `[-3.4e38]`, T{-3.4e38})
	s.TestOK(t, "avogadros_num", `[6.022e23]`, T{6.022e23})
	s.TestOK(t, "array_float_1024", arrayFloat1024)

	s.TestOKPrepare(t, "var_overwrite", `[1.1, 2.2, 3.3]`, Test[T]{
		PrepareJscan: func() []float64 { return []float64{10.1, 20.2, 30.3} },
		Expect:       []float64{1.1, 2.2, 3.3},
	})
	s.TestOKPrepare(t, "var_nil_realloc", `[1.1, 2.2, 3.3]`, Test[T]{
		PrepareJscan: func() []float64 { return []float64(nil) },
		Expect:       []float64{1.1, 2.2, 3.3},
	})
	s.TestOKPrepare(t, "var_realloc", `[1.1, 2.2, 3.3]`, Test[T]{
		PrepareJscan: func() []float64 { return []float64{10.1, 20.2} },
		Expect:       []float64{1.1, 2.2, 3.3},
	})
	s.TestOKPrepare(t, "var_shrink", `[1.1, 2.2, 3.3]`, Test[T]{
		PrepareJscan: func() []float64 { return []float64{10.1, 20.2, 30.3, 40.4} },
		Expect:       []float64{1.1, 2.2, 3.3},
	})

	s.testErrCheck(t, "range_hi", `[1e309]`,
		func(t *testing.T, errIndex int, err error) {
			require.ErrorIs(t, err, strconv.ErrRange)
			require.Equal(t, 1, errIndex)
		})

	s.testErr(t, "wrong_type_string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_true", `true`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_false", `false`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_object", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_array", `[1,[]]`,
		3, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		3, jscandec.ErrUnexpectedValue)
}

func TestDecode2DSliceInt(t *testing.T) {
	type T = [][]int
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "3_2", `[[0],[12, 123],[ ]]`, T{{0}, {12, 123}, {}})
	s.TestOK(t, "2_1", `[[],[-12345678]]`, T{{}, {-12_345_678}})
	s.TestOK(t, "2_0", `[[],[]]`, T{{}, {}})
	s.TestOK(t, "1", `[]`, T{})

	s.TestOKPrepare(t, "var_overwrite", `[ [1], [], [3] ]`, Test[T]{
		PrepareJscan: func() [][]int { return [][]int{{10}, {20}, {30}} },
		Expect:       [][]int{{1}, {}, {3}},
	})
	s.TestOKPrepare(t, "var_nil_realloc", `[ [1], [], [3] ]`, Test[T]{
		PrepareJscan: func() [][]int { return [][]int(nil) },
		Expect:       [][]int{{1}, {}, {3}},
	})
	s.TestOKPrepare(t, "var_realloc", `[ [1], [], [3] ]`, Test[T]{
		PrepareJscan: func() [][]int { return [][]int{{10}, {20}} },
		Expect:       [][]int{{1}, {}, {3}},
	})
	s.TestOKPrepare(t, "var_shrink", `[ [1], [], [3] ]`, Test[T]{
		PrepareJscan: func() [][]int { return [][]int{{10}, {20}, {30}, {40}} },
		Expect:       [][]int{{1}, {}, {3}},
	})
}

func TestDecodeMatrix2Int(t *testing.T) {
	type T = [2][2]int
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "complete",
		`[[0,1],[2,3]]`,
		T{{0, 1}, {2, 3}})
	s.TestOK(t, "empty",
		`[]`,
		T{{0, 0}, {0, 0}})
	s.TestOK(t, "sub_arrays_empty",
		`[[],[]]`,
		T{{0, 0}, {0, 0}})
	s.TestOK(t, "incomplete",
		`[[1],[2]]`,
		T{{1, 0}, {2, 0}})
	s.TestOK(t, "partially_incomplete",
		`[[1,2],[3]]`,
		T{{1, 2}, {3, 0}})
	s.TestOK(t, "overflow_subarray_ignore",
		`[[1,2,  3,4],[5,6,  7,8,9,10]]`,
		T{{1, 2}, {5, 6}})
	s.TestOK(t, "overflow_ignore",
		`[[1,2],[3,4],  [5,6]]`,
		T{{1, 2}, {3, 4}})
}

func TestDecodeMatrix4Int(t *testing.T) {
	type T = [4][4]int
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "complete",
		`[[0,1,2,3],[4,5,6,7],[8,9,10,11],[12,13,14,15]]`,
		T{{0, 1, 2, 3}, {4, 5, 6, 7}, {8, 9, 10, 11}, {12, 13, 14, 15}})
	s.TestOK(t, "incomplete",
		`[[1],[2,3],[4,5,6],[]]`,
		T{{1, 0, 0, 0}, {2, 3, 0, 0}, {4, 5, 6, 0}, {0, 0, 0, 0}})
	s.TestOK(t, "empty",
		`[]`,
		T{{0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}})
	s.TestOK(t, "sub_arrays_empty_incomplete",
		`[[],[]]`,
		T{{0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}})
	s.TestOK(t, "overflow_subarray_ignore",
		`[[1,2,3,4,  5],[6,7,8,9,  10,11,12,13,14]]`,
		T{{1, 2, 3, 4}, {6, 7, 8, 9}})
	s.TestOK(t, "overflow_ignore",
		`[[1,2,3,4],[5,6,7,8],[9,10,11,12],[13,14,15,16],  [17]]`,
		T{{1, 2, 3, 4}, {5, 6, 7, 8}, {9, 10, 11, 12}, {13, 14, 15, 16}})
}

func TestDecodeEmptyStruct(t *testing.T) {
	type S struct{}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "null", `null`, S{})
	s.TestOK(t, "empty_object", `{}`, S{})
	s.TestOK(t, "object", `{"x":"y"}`, S{})
	s.TestOK(t, "object_multikey",
		`{"x":"y","abc":[{"x":"y","2":42}, null, {}]}`, S{})

	s.testErr(t, "true", `true`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array_empty", `[]`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[{},{}]`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeSliceEmptyStruct(t *testing.T) {
	type S struct{}
	type T = []S
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "null", `null`, []S(nil))
	s.TestOK(t, "empty_array", `[]`, []S{})
	s.TestOK(t, "array_one", `[{}]`, []S{{}})
	s.TestOK(t, "array_multiple", `[{},{},{}]`, []S{{}, {}, {}})

	s.TestOKPrepare(t, "var_overwrite", `[{}, null, {}]`, Test[T]{
		PrepareJscan: func() []S { return []S{{}, {}, {}} },
		Expect:       []S{{}, {}, {}},
	})
	s.TestOKPrepare(t, "var_nil_realloc", `[{}, null, {}]`, Test[T]{
		PrepareJscan: func() []S { return []S(nil) },
		Expect:       []S{{}, {}, {}},
	})
	s.TestOKPrepare(t, "var_realloc", `[{}, null, {}]`, Test[T]{
		PrepareJscan: func() []S { return []S{{}, {}} },
		Expect:       []S{{}, {}, {}},
	})
	s.TestOKPrepare(t, "var_shrink", `[{}, null, {}]`, Test[T]{
		PrepareJscan: func() []S { return []S{{}, {}, {}, {}} },
		Expect:       []S{{}, {}, {}},
	})

	s.testErr(t, "true", `true`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object_empty", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object", `{"x":0}`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeSliceStruct(t *testing.T) {
	type S struct{ A, B int }
	type T = []S
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "null", `null`, []S(nil))
	s.TestOK(t, "empty_array", `[]`, []S{})
	s.TestOK(t, "item_empty", `[{}]`, []S{{}})
	s.TestOK(t, "item_null", `[null]`, []S{{}})
	s.TestOK(t, "item", `[{"A":1,"B":2}]`, []S{{A: 1, B: 2}})
	s.TestOK(t, "item_partial", `[{"B":2}]`, []S{{B: 2}})
	s.TestOK(t, "item_extra_ignore", `[{"A":1,"X":99,"B":2}]`, []S{{A: 1, B: 2}})
	s.TestOK(t, "items_empty", `[{},null,{}]`, []S{{}, {}, {}})
	s.TestOK(t, "items_partial", `[{"B":2},null,{"A":3}]`, []S{{B: 2}, {}, {A: 3}})

	s.TestOKPrepare(t, "var_overwrite", `[{"A":1}, {"B":2}, {"C":3,"B":3}]`, Test[T]{
		PrepareJscan: func() []S { return []S{{A: 9, B: 9}, {A: 9, B: 9}, {A: 9, B: 9}} },
		Expect:       []S{{A: 1, B: 9}, {A: 9, B: 2}, {A: 9, B: 3}},
	})
	s.TestOKPrepare(t, "var_nil_realloc", `[{}, null, {"B":1}]`, Test[T]{
		PrepareJscan: func() []S { return []S(nil) },
		Expect:       []S{{}, {}, {B: 1}},
	})
	s.TestOKPrepare(t, "var_realloc", `[ {"B":1}, null, {"A":1, "B":2} ]`, Test[T]{
		PrepareJscan: func() []S { return []S{{A: 9, B: 9}, {A: 9, B: 9}} },
		Expect:       []S{{A: 9, B: 1}, {A: 9, B: 9}, {A: 1, B: 2}},
	})
	s.TestOKPrepare(t, "var_shrink", `[{}, null, {"A":42}, {"A":1,"B":2}]`, Test[T]{
		PrepareJscan: func() []S {
			return []S{
				{A: 9, B: 9}, {A: 9, B: 9}, {A: 9, B: 9}, {A: 9, B: 9}, {A: 9, B: 9},
			}
		},
		Expect: []S{{A: 9, B: 9}, {A: 9, B: 9}, {A: 42, B: 9}, {A: 1, B: 2}},
	})

	s.testErr(t, "true", `true`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object_empty", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object", `{"x":0}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array_int", `[123]`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array_string", `["x"]`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_A_float", `[{"A": 2.2}]`,
		7, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_B_exponent", `[{"B": 1e10}]`,
		7, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_B_string", `[{"B": "123"}]`,
		7, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_B_true", `[{"B": true}]`,
		7, jscandec.ErrUnexpectedValue)
}

func TestDecodeArray(t *testing.T) {
	s := newTestSetup[[3]int](t, *jscandec.DefaultOptions)
	s.TestOK(t, "null", `null`, [3]int{})
	s.TestOK(t, "empty_array", `[]`, [3]int{})
	s.TestOK(t, "array_one", `[1]`, [3]int{1, 0, 0})
	s.TestOK(t, "array_full", `[1,2,3]`, [3]int{1, 2, 3})
	s.TestOK(t, "array_overflow",
		`[1,2,3,false,true,{},{"x":"y"},[],null,42,3.14]`, [3]int{1, 2, 3})

	s.testErr(t, "true", `true`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `3.14`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object_empty", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object", `{"x":0}`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeArray2D(t *testing.T) {
	type A [2][2]int
	s := newTestSetup[A](t, *jscandec.DefaultOptions)
	s.TestOK(t, "null", `null`, A{})
	s.TestOK(t, "empty_array", `[]`, A{})
	s.TestOK(t, "array_one", `[[]]`, A{})
	s.TestOK(t, "array_full", `[[1,2],[3,4]]`, A{{1, 2}, {3, 4}})
	s.TestOK(t, "array_overflow",
		`[[1,2],[3,4],false,true,{},{"x":"y"},[],null,42,3.14]`, A{{1, 2}, {3, 4}})
	s.TestOK(t, "array_overflow_in_subarray",
		`[[1,2, 3,[],{}],[4,5, 6,[],{}],false,true,{},{"x":"y"},[],null,42,3.14]`,
		A{{1, 2}, {4, 5}})

	s.testErr(t, "true", `true`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `3.14`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object_empty", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object", `{"x":0}`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeArrayLen0(t *testing.T) {
	s := newTestSetup[[0]int](t, *jscandec.DefaultOptions)
	s.TestOK(t, "null", `null`, [0]int{})
	s.TestOK(t, "empty_array", `[]`, [0]int{})
	s.TestOK(t, "array_one", `[1]`, [0]int{})
	s.TestOK(t, "array_one_empty_object", `[{}]`, [0]int{})
	s.TestOK(t, "array_multiple",
		`[false,true,{},{"x":"y"},[],null,42,3.14]`, [0]int{})

	s.testErr(t, "true", `true`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `3.14`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object_empty", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object", `{"x":0}`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeArrayLen02D(t *testing.T) {
	type A [0][0]int
	s := newTestSetup[A](t, *jscandec.DefaultOptions)
	s.TestOK(t, "null", `null`, A{})
	s.TestOK(t, "empty_array", `[]`, A{})
	s.TestOK(t, "array_one", `[1]`, A{})
	s.TestOK(t, "array_one_empty_object", `[{}]`, A{})
	s.TestOK(t, "array_multiple",
		`[false,true,{},{"x":"y"},[],null,42,3.14]`, A{})

	s.testErr(t, "true", `true`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `3.14`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object_empty", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object", `{"x":0}`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeArrayArrayLen0(t *testing.T) {
	type A [2][0]int
	s := newTestSetup[A](t, *jscandec.DefaultOptions)
	s.TestOK(t, "null", `null`, A{})
	s.TestOK(t, "empty_array", `[]`, A{})
	s.TestOK(t, "array_overflow",
		`[[],[],false,true,{},{"x":"y"},[],null,42,3.14]`, A{})
	s.TestOK(t, "array_overflow_in_subarray",
		`[["foo",1.2],["bar",3.4],false,true,{},{"x":"y"},[],null,42,3.14]`, A{})

	s.testErr(t, "array_int", `[1,2]`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array_one_empty_object", `[{}]`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `3.14`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object_empty", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object", `{"x":0}`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeArrayEmptyStruct(t *testing.T) {
	type S struct{}
	s := newTestSetup[[3]S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "null", `null`, [3]S{})
	s.TestOK(t, "empty_array", `[]`, [3]S{})
	s.TestOK(t, "array_one", `[{}]`, [3]S{})
	s.TestOK(t, "array_full", `[{},{},{}]`, [3]S{})
	s.TestOK(t, "array_overflow", `[{},{},{},{},{},{},{},{}, {}]`, [3]S{})

	s.testErr(t, "true", `true`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object_empty", `{}`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object", `{"x":0}`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeStruct(t *testing.T) {
	type S struct {
		Foo int    `json:"foo"`
		Bar string `json:"bar"`
	}
	type T = S
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.TestOK(t, "regular_field_order",
		`{"foo":42,"bar":"bazz"}`, S{Foo: 42, Bar: "bazz"})
	s.TestOK(t, "reversed_field_order",
		`{"bar":"abc","foo":1234}`, S{Foo: 1234, Bar: "abc"})
	s.TestOK(t, "case_insensitive_match1",
		`{"FOO":42,"BAR":"bazz"}`, S{Foo: 42, Bar: "bazz"})
	s.TestOK(t, "case_insensitive_match2",
		`{"Foo":42,"Bar":"bazz"}`, S{Foo: 42, Bar: "bazz"})
	s.TestOK(t, "null_fields", `{"foo":null,"bar":null}`, S{Foo: 0, Bar: ""})

	s.TestOK(t, "missing_field_foo",
		`{"bar":"bar"}`, S{Bar: "bar"})
	s.TestOK(t, "missing_field_bar",
		`{"foo":12345}`, S{Foo: 12345})
	s.TestOK(t, "unknown_field",
		`{"bar":"bar","unknown":42,"foo":102}`, S{Foo: 102, Bar: "bar"})
	s.TestOK(t, "unknown_fields_only",
		`{"unknown":42, "unknown2": "bad"}`, S{})

	s.TestOK(t, "empty", `{}`, S{})
	s.TestOK(t, "name_mismatch", `{"faz":42,"baz":"bazz"}`, S{})

	s.TestOKPrepare(t, "overwrite", `{"foo":42,"bar":"bazz"}`, Test[T]{
		PrepareJscan: func() S { return S{Foo: 99, Bar: "predefined"} },
		Expect:       S{Foo: 42, Bar: "bazz"},
	})
	s.TestOKPrepare(t, "overwrite_foo", `{"foo":42}`, Test[T]{
		PrepareJscan: func() S { return S{Foo: 99, Bar: "predefined"} },
		Expect:       S{Foo: 42, Bar: "predefined"},
	})
	s.TestOKPrepare(t, "overwrite_bar", `{"bar":"newval"}`, Test[T]{
		PrepareJscan: func() S { return S{Foo: 99, Bar: "predefined"} },
		Expect:       S{Foo: 99, Bar: "newval"},
	})
	s.TestOKPrepare(t, "unknown_field_overwrite", `{"fazz":false}`, Test[T]{
		PrepareJscan: func() S { return S{Foo: 99, Bar: "predefined"} },
		Expect:       S{Foo: 99, Bar: "predefined"},
	})

	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeStructRecursivePtr(t *testing.T) {
	type S struct {
		ID      string
		Name    string
		Recurse *S
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "null",
		`null`, S{})
	s.TestOK(t, "empty",
		`{}`, S{})
	s.TestOK(t, "root",
		`{"id":"root"}`, S{ID: "root"})
	s.TestOK(t, "2_level",
		`{"id":"root","recurse":{"id":"level2", "name": "Level 2"}}`,
		S{ID: "root", Recurse: &S{ID: "level2", Name: "Level 2"}})
	s.TestOK(t, "3_level",
		`{
			"id": "root",
			"recurse": {
				"id": "level2",
				"recurse": {
					"id": "level3"
				}
			}
		}`,
		S{ID: "root", Recurse: &S{ID: "level2", Recurse: &S{ID: "level3"}}})
	s.TestOK(t, "3_level_reversed_field_order",
		`{
			"recurse": {
				"recurse": {
					"id": "level3"
				},
				"id": "level2"
			},
			"id": "root"
		}`,
		S{ID: "root", Recurse: &S{ID: "level2", Recurse: &S{ID: "level3"}}})
	s.TestOK(t, "3_level_missing_field",
		`{
			"recurse": {
				"recurse": {}
			}
		}`,
		S{Recurse: &S{Recurse: &S{}}})
	s.TestOK(t, "null_level2",
		`{
			"id": "root",
			"recurse": null
		}`,
		S{ID: "root", Recurse: nil})
	s.TestOK(t, "empty_level2",
		`{
			"id": "root",
			"recurse": {}
		}`,
		S{ID: "root", Recurse: &S{}})
	s.TestOK(t, "null_level3",
		`{
			"id": "root",
			"recurse": {
				"id":"level2",
				"recurse": null
			}
		}`,
		S{ID: "root", Recurse: &S{ID: "level2", Recurse: nil}})
	s.TestOK(t, "empty_level3",
		`{
			"id": "root",
			"recurse": {
				"id":"level2",
				"recurse": {}
			}
		}`,
		S{ID: "root", Recurse: &S{ID: "level2", Recurse: &S{}}})
	s.TestOK(t, "3_level_unknown_field",
		`{
			"recurse": {
				"recurse": {
					"unknown":["okay"]
				}
			}
		}`,
		S{Recurse: &S{Recurse: &S{}}})
	s.TestOK(t, "3_level_upper_case_field_names",
		`{
			"RECURSE": {
				"RECURSE": {
					"ID": "level3"
				},
				"ID": "level2"
			},
			"ID": "root"
		}`,
		S{ID: "root", Recurse: &S{ID: "level2", Recurse: &S{ID: "level3"}}})

	s.TestOKPrepare(t, "overwrite",
		`{"name":"Root", "recurse":{"name": "L2"}}`,
		Test[S]{
			PrepareJscan: func() S { return S{ID: "root", Recurse: &S{ID: "level2"}} },
			Expect: S{
				ID: "root", Name: "Root", Recurse: &S{ID: "level2", Name: "L2"},
			},
		})

	s.TestOKPrepare(t, "overwrite_null_level2",
		`{"name":"Root", "recurse":null}`,
		Test[S]{
			PrepareJscan: func() S { return S{ID: "root", Recurse: &S{ID: "level2"}} },
			Expect:       S{ID: "root", Name: "Root", Recurse: nil},
		})

	s.TestOKPrepare(t, "overwrite_empty_level2",
		`{"name":"Root", "recurse":{}}`,
		Test[S]{
			PrepareJscan: func() S { return S{ID: "root", Recurse: &S{ID: "level2"}} },
			Expect:       S{ID: "root", Name: "Root", Recurse: &S{ID: "level2"}},
		})
	s.TestOKPrepare(t, "overwrite_unknown_field_level2",
		`{"name":"Root", "recurse":{"fuzz":"inexistent"}}`, Test[S]{
			PrepareJscan: func() S { return S{ID: "root", Recurse: &S{ID: "level2"}} },
			Expect:       S{ID: "root", Name: "Root", Recurse: &S{ID: "level2"}},
		})

	s.testErr(t, "wrong_type_level2", `{"id":"root","recurse":[]}`,
		23, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_level3", `{"id":"root","recurse":{"id":"2","recurse":"x"}}`,
		43, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeStructRecursiveSlice(t *testing.T) {
	type S struct {
		ID      string
		Name    string
		Recurse []S
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "null",
		`null`, S{})
	s.TestOK(t, "empty",
		`{}`, S{})
	s.TestOK(t, "root",
		`{"id":"root"}`, S{ID: "root"})
	s.TestOK(t, "2_level",
		`{"id":"root","recurse":[{"id":"level2", "name": "Level 2"}]}`,
		S{ID: "root", Recurse: []S{{ID: "level2", Name: "Level 2"}}})
	s.TestOK(t, "3_level",
		`{
			"id": "root",
			"recurse": [{
				"id": "level2",
				"recurse": [{
					"id": "level3"
				}]
			}]
		}`,
		S{ID: "root", Recurse: []S{{ID: "level2", Recurse: []S{{ID: "level3"}}}}})
	s.TestOK(t, "3_level_reversed_field_order",
		`{
			"recurse": [{
				"recurse": [{
					"id": "level3"
				}],
				"id": "level2"
			}],
			"id": "root"
		}`,
		S{ID: "root", Recurse: []S{{ID: "level2", Recurse: []S{{ID: "level3"}}}}})
	s.TestOK(t, "3_level_missing_field",
		`{
			"recurse": [{
				"recurse": [{}]
			}]
		}`,
		S{Recurse: []S{{Recurse: []S{{ /* empty */ }}}}})
	s.TestOK(t, "null_level2",
		`{
			"id": "root",
			"recurse": null
		}`,
		S{ID: "root", Recurse: nil})
	s.TestOK(t, "empty_level2",
		`{
			"id": "root",
			"recurse": []
		}`,
		S{ID: "root", Recurse: []S{}})
	s.TestOK(t, "null_level3",
		`{
			"id": "root",
			"recurse": [{
				"id":"level2",
				"recurse": null
			}]
		}`,
		S{ID: "root", Recurse: []S{{ID: "level2", Recurse: nil}}})
	s.TestOK(t, "empty_level3",
		`{
			"id": "root",
			"recurse": [{
				"id":"level2",
				"recurse": [{}]
			}]
		}`,
		S{ID: "root", Recurse: []S{{ID: "level2", Recurse: []S{{ /* empty */ }}}}})
	s.TestOK(t, "3_level_unknown_field",
		`{
			"recurse": [{
				"recurse": [{
					"unknown":["okay"]
				}]
			}]
		}`,
		S{Recurse: []S{{Recurse: []S{{ /* empty */ }}}}})
	s.TestOK(t, "3_level_upper_case_field_names",
		`{
			"RECURSE": [{
				"RECURSE": [{
					"ID": "level3"
				}],
				"ID": "level2"
			}],
			"ID": "root"
		}`,
		S{ID: "root", Recurse: []S{{ID: "level2", Recurse: []S{{ID: "level3"}}}}})

	s.TestOKPrepare(t, "overwrite",
		`{"name":"Root", "recurse":[{"name": "L2"}]}`,
		Test[S]{
			PrepareJscan: func() S { return S{ID: "root", Recurse: []S{{ID: "level2"}}} },
			Expect: S{
				ID: "root", Name: "Root", Recurse: []S{{ID: "level2", Name: "L2"}},
			},
		})

	s.TestOKPrepare(t, "overwrite_null_level2",
		`{"name":"Root", "recurse":null}`,
		Test[S]{
			PrepareJscan: func() S { return S{ID: "root", Recurse: []S{{ID: "level2"}}} },
			Expect:       S{ID: "root", Name: "Root", Recurse: nil},
		})

	s.TestOKPrepare(t, "overwrite_empty_level2",
		`{"name":"Root", "recurse":[{}]}`,
		Test[S]{
			PrepareJscan: func() S { return S{ID: "root", Recurse: []S{{ID: "level2"}}} },
			Expect:       S{ID: "root", Name: "Root", Recurse: []S{{ID: "level2"}}},
		})
	s.TestOKPrepare(t, "overwrite_unknown_field_level2",
		`{"name":"Root", "recurse":[{"fuzz":"inexistent"}]}`,
		Test[S]{
			PrepareJscan: func() S { return S{ID: "root", Recurse: []S{{ID: "level2"}}} },
			Expect:       S{ID: "root", Name: "Root", Recurse: []S{{ID: "level2"}}},
		})

	s.testErr(t, "wrong_type_level2", `{"id":"root","recurse":{}}`,
		23, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_level3", `{"id":"root","recurse":[{"id":"2","recurse":"x"}]}`,
		44, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeStructRecursiveMap(t *testing.T) {
	type S struct {
		ID      string
		Name    string
		Recurse map[string]S
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "null",
		`null`, S{})
	s.TestOK(t, "empty",
		`{}`, S{})
	s.TestOK(t, "root",
		`{"id":"root"}`, S{ID: "root"})
	s.TestOK(t, "2_level",
		`{"id":"root","recurse":{"L2": {"id":"level2", "name": "Level 2"}}}`,
		S{
			ID: "root", Recurse: map[string]S{
				"L2": {
					ID: "level2", Name: "Level 2",
				},
			},
		})
	s.TestOK(t, "3_level",
		`{
			"id": "root",
			"recurse": {"L2": {
				"id": "level2",
				"recurse": {"L3": {
					"id": "level3"
				}}
			}}
		}`,
		S{
			ID: "root", Recurse: map[string]S{
				"L2": {
					ID: "level2", Recurse: map[string]S{
						"L3": {
							ID: "level3",
						},
					},
				},
			},
		})
	s.TestOK(t, "3_level_reversed_field_order",
		`{
			"recurse": {"L2": {
				"recurse": {"L3": {
					"id": "level3"
				}},
				"id": "level2"
			}},
			"id": "root"
		}`,
		S{ID: "root", Recurse: map[string]S{
			"L2": {
				ID: "level2", Recurse: map[string]S{
					"L3": {ID: "level3"},
				},
			},
		}})
	s.TestOK(t, "3_level_missing_field",
		`{
			"recurse": {"E": {
				"recurse": {"E2": {}}
			}}
		}`,
		S{Recurse: map[string]S{"E": {
			Recurse: map[string]S{
				"E2": { /* empty */ },
			},
		}}})
	s.TestOK(t, "null_level2",
		`{
			"id": "root",
			"recurse": null
		}`,
		S{ID: "root", Recurse: nil})
	s.TestOK(t, "empty_level2",
		`{
			"id": "root",
			"recurse": {}
		}`,
		S{ID: "root", Recurse: map[string]S{}})
	s.TestOK(t, "null_level3",
		`{
			"id": "root",
			"recurse": {"L2": {
				"id":"level2",
				"recurse": null
			}}
		}`,
		S{ID: "root", Recurse: map[string]S{
			"L2": {
				ID: "level2", Recurse: nil,
			},
		}})
	s.TestOK(t, "empty_level3",
		`{
			"id": "root",
			"recurse": {"L2": {
				"id":"level2",
				"recurse": {"L3": {}}
			}}
		}`,
		S{ID: "root", Recurse: map[string]S{
			"L2": {
				ID: "level2", Recurse: map[string]S{
					"L3": { /* empty */ },
				},
			},
		}})
	s.TestOK(t, "3_level_unknown_field",
		`{
			"recurse": {"L2": {
				"recurse": {"L3": {
					"unknown":["okay"]
				}}
			}}
		}`,
		S{Recurse: map[string]S{
			"L2": {
				Recurse: map[string]S{
					"L3": { /* empty */ },
				},
			},
		}})
	s.TestOK(t, "3_level_upper_case_field_names",
		`{
			"RECURSE": {"L2": {
				"RECURSE": {"L3": {
					"ID": "level3"
				}},
				"ID": "level2"
			}},
			"ID": "root"
		}`,
		S{ID: "root", Recurse: map[string]S{
			"L2": {
				ID: "level2", Recurse: map[string]S{
					"L3": {ID: "level3"},
				},
			},
		}})

	s.TestOKPrepare(t, "overwrite",
		`{"name":"Root", "recurse":{"L2": {"name": "L2"}}}`,
		Test[S]{
			PrepareJscan: func() S {
				return S{ID: "root", Recurse: map[string]S{"L2": {ID: "level2"}}}
			},
			Expect: S{
				ID: "root", Name: "Root", Recurse: map[string]S{
					"L2": {
						Name: "L2", Recurse: map[string]S(nil),
					},
				},
			},
		})

	s.TestOKPrepare(t, "overwrite_null_level2",
		`{"name":"Root", "recurse":null}`,
		Test[S]{
			PrepareJscan: func() S {
				return S{ID: "root", Recurse: map[string]S{"L2": {ID: "level2"}}}
			},
			Expect: S{ID: "root", Name: "Root", Recurse: nil},
		})

	s.TestOKPrepare(t, "overwrite_empty_level2",
		`{"name":"Root", "recurse":{"L2": {}}}`,
		Test[S]{
			PrepareJscan: func() S {
				return S{ID: "root", Recurse: map[string]S{"L2": {ID: "level2"}}}
			},
			Expect: S{
				ID: "root", Name: "Root", Recurse: map[string]S{"L2": { /* empty */ }},
			},
		})

	s.TestOKPrepare(t, "overwrite_unknown_field_level2",
		`{"name":"Root", "recurse":{"L2": {"fuzz":"inexistent"}}}`,
		Test[S]{
			PrepareJscan: func() S {
				return S{ID: "root", Recurse: map[string]S{"L2": {ID: "level2"}}}
			},
			Expect: S{
				ID: "root", Name: "Root", Recurse: map[string]S{"L2": { /* empty */ }},
			},
		})

	s.testErr(t, "wrong_type_level2", `{"id":"root","recurse":[]}`,
		23, jscandec.ErrUnexpectedValue)
	s.testErr(t, "wrong_type_level3",
		`{"id":"root","recurse":{"L2": {"id":"2","recurse":"x"}}}`,
		50, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeStructErrUknownField(t *testing.T) {
	type S struct {
		Foo int    `json:"foo"`
		Bar string `json:"bar"`
	}
	s := newTestSetup[S](t, jscandec.DecodeOptions{DisallowUnknownFields: true})
	s.TestOK(t, "regular_field_order",
		`{"foo":42,"bar":"bazz"}`, S{Foo: 42, Bar: "bazz"})
	s.TestOK(t, "reversed_field_order",
		`{"bar":"abc","foo":1234}`, S{Foo: 1234, Bar: "abc"})
	s.TestOK(t, "case_insensitive_match1",
		`{"FOO":42,"BAR":"bazz"}`, S{Foo: 42, Bar: "bazz"})
	s.TestOK(t, "case_insensitive_match2",
		`{"Foo":42,"Bar":"bazz"}`, S{Foo: 42, Bar: "bazz"})
	s.TestOK(t, "null_fields", `{"foo":null,"bar":null}`, S{Foo: 0, Bar: ""})

	s.TestOK(t, "missing_field_foo",
		`{"bar":"bar"}`, S{Bar: "bar"})
	s.TestOK(t, "missing_field_bar",
		`{"foo":12345}`, S{Foo: 12345})

	s.TestOK(t, "empty", `{}`, S{})

	s.testErr(t, "unknown_field",
		`{"bar":"bar","unknown":42,"foo":102}`,
		13, jscandec.ErrUnknownField)
	s.testErr(t, "unknown_fields_only",
		`{"unknown":42, "unknown2": "bad"}`,
		1, jscandec.ErrUnknownField)
	s.testErr(t, "name_mismatch", `{"faz":42,"baz":"bazz"}`,
		1, jscandec.ErrUnknownField)

	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeStructFields(t *testing.T) {
	type S struct {
		Any   any
		Map   map[string]any
		Slice []any
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "case_insensitive_match",
		`{"any":42,"Map":{"foo":"bar"},"SLICE":[1,false,"x"]}`,
		S{
			Any:   float64(42),
			Map:   map[string]any{"foo": "bar"},
			Slice: []any{float64(1), false, "x"},
		})
	s.TestOK(t, "different_order_and_types",
		`{"map":{},"slice":[{"1":"2", "3":4},null,[]],"any":{"x":"y"}}`,
		S{
			Map:   map[string]any{},
			Slice: []any{map[string]any{"1": "2", "3": float64(4)}, nil, []any{}},
			Any:   map[string]any{"x": "y"},
		})
	s.TestOK(t, "null_fields",
		`{"map":null,"slice":null,"any":null}`,
		S{})
	s.TestOK(t, "partial_one_field",
		`{"slice":[{"x":false,"y":42},{"Имя":"foo"},{"x":{}}]}`,
		S{Slice: []any{
			map[string]any{"x": false, "y": float64(42)},
			map[string]any{"Имя": "foo"},
			map[string]any{"x": map[string]any{}},
		}})

	s.TestOK(t, "null", `null`, S{})
	s.TestOK(t, "empty", `{}`, S{})
	s.TestOK(t, "name_mismatch", `{"faz":42,"baz":"bazz"}`, S{})

	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeStringTagBool(t *testing.T) {
	type S struct {
		Bool bool `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty_string",
		`{"bool":null}`, S{Bool: false})
	s.TestOK(t, "false",
		`{"bool":"false"}`, S{Bool: false})
	s.TestOK(t, "true",
		`{"bool":"true"}`, S{Bool: true})

	s.TestOK(t, "empty", `{}`, S{})
	s.TestOK(t, "null", `null`, S{})

	s.testErr(t, "empty", `{"bool":""}`,
		8, jscandec.ErrUnexpectedValue)
	s.testErr(t, "text", `{"bool":"text"}`,
		8, jscandec.ErrUnexpectedValue)
	s.testErr(t, "one", `{"bool":"1"}`,
		8, jscandec.ErrUnexpectedValue)
	s.testErr(t, "zero", `{"bool":"0"}`,
		8, jscandec.ErrUnexpectedValue)
	s.testErr(t, "space_prefix", `{"bool":" true"}`,
		8, jscandec.ErrUnexpectedValue)
	s.testErr(t, "space_suffix", `{"bool":"true "}`,
		8, jscandec.ErrUnexpectedValue)
	s.testErr(t, "multiple_values", `{"bool":"true true"}`,
		8, jscandec.ErrUnexpectedValue)
	s.testErr(t, "suffix_new_line", `{"bool":"true\n"}`,
		8, jscandec.ErrUnexpectedValue)
	s.testErr(t, "suffix_text", `{"bool":"trueabc"}`,
		8, jscandec.ErrUnexpectedValue)
	s.testErr(t, "suffix_false", `{"bool":"truefalse"}`,
		8, jscandec.ErrUnexpectedValue)

	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeStringTagString(t *testing.T) {
	type S struct {
		String string `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty_string",
		`{"string":"\"\""}`, S{String: ""})
	s.TestOK(t, "space",
		`{"string":"\" \""}`, S{String: " "})
	s.TestOK(t, "text",
		`{"string":"\"text\""}`, S{String: "text"})

	s.TestOK(t, "empty", `{}`, S{})
	s.TestOK(t, "null", `null`, S{})

	s.testErr(t, "empty", `{"string":""}`,
		10, jscandec.ErrUnexpectedValue)
	s.testErr(t, "space_prefix", `{"string":" \"\""}`,
		10, jscandec.ErrUnexpectedValue)
	s.testErr(t, "space_suffix", `{"string":"\"\" "}`,
		10, jscandec.ErrUnexpectedValue)
	s.testErr(t, "multiple_strings", `{"string":"\"first\"\"second\""}`,
		10, jscandec.ErrUnexpectedValue)
	s.testErr(t, "suffix_new_line", `{"string":"\"okay\"\n"}`,
		10, jscandec.ErrUnexpectedValue)
	s.testErr(t, "suffix_text", `{"string":"\"okay\"abc"}`,
		10, jscandec.ErrUnexpectedValue)
	s.testErr(t, "suffix_text", `{"string":"\"ok\"\"ay\""}`,
		10, jscandec.ErrUnexpectedValue)

	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeStringTagInt(t *testing.T) {
	type S struct {
		Int int `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, S{})
	s.TestOK(t, "null", `null`, S{})
	s.TestOK(t, "min", `{"int":"-9223372036854775808"}`, S{Int: -9223372036854775808})
	s.TestOK(t, "max", `{"int":"9223372036854775807"}`, S{Int: 9223372036854775807})

	s.testErr(t, "overflow_hi", `{"int":"9223372036854775808"}`,
		7, jscandec.ErrUnexpectedValue)
	s.testErr(t, "overflow_lo", `{"int":"-9223372036854775809"}`,
		7, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"int":"3.14"}`,
		7, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"int":"3e2"}`,
		7, jscandec.ErrUnexpectedValue)
}

func TestDecodeStringTagInt8(t *testing.T) {
	type S struct {
		Int8 int8 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, S{})
	s.TestOK(t, "null", `null`, S{})
	s.TestOK(t, "min", `{"int8":"-128"}`, S{Int8: -128})
	s.TestOK(t, "max", `{"int8":"127"}`, S{Int8: 127})

	s.testErr(t, "overflow_hi", `{"int8":"128"}`,
		8, jscandec.ErrUnexpectedValue)
	s.testErr(t, "overflow_lo", `{"int8":"-129"}`,
		8, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"int8":"3.14"}`,
		8, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"int8":"3e2"}`,
		8, jscandec.ErrUnexpectedValue)
}

func TestDecodeStringTagInt16(t *testing.T) {
	type S struct {
		Int16 int16 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, S{})
	s.TestOK(t, "null", `null`, S{})
	s.TestOK(t, "min", `{"int16":"-32768"}`, S{Int16: -32768})
	s.TestOK(t, "max", `{"int16":"32767"}`, S{Int16: 32767})

	s.testErr(t, "overflow_hi", `{"int16":"32768"}`,
		9, jscandec.ErrUnexpectedValue)
	s.testErr(t, "overflow_lo", `{"int16":"-32769"}`,
		9, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"int16":"3.14"}`,
		9, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"int16":"3e2"}`,
		9, jscandec.ErrUnexpectedValue)
}

func TestDecodeStringTagInt32(t *testing.T) {
	type S struct {
		Int32 int32 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, S{})
	s.TestOK(t, "null", `null`, S{})
	s.TestOK(t, "min", `{"int32":"-2147483648"}`, S{Int32: -2147483648})
	s.TestOK(t, "max", `{"int32":"2147483647"}`, S{Int32: 2147483647})

	s.testErr(t, "overflow_hi", `{"int32":"2147483648"}`,
		9, jscandec.ErrUnexpectedValue)
	s.testErr(t, "overflow_lo", `{"int32":"-2147483649"}`,
		9, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"int32":"3.14"}`,
		9, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"int32":"3e2"}`,
		9, jscandec.ErrUnexpectedValue)
}

func TestDecodeStringTagInt64(t *testing.T) {
	type S struct {
		Int64 int64 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, S{})
	s.TestOK(t, "null", `null`, S{})
	s.TestOK(t, "min", `{"int64":"-9223372036854775808"}`,
		S{Int64: -9223372036854775808})
	s.TestOK(t, "max", `{"int64":"9223372036854775807"}`,
		S{Int64: 9223372036854775807})

	s.testErr(t, "overflow_hi", `{"int64":"9223372036854775808"}`,
		9, jscandec.ErrUnexpectedValue)
	s.testErr(t, "overflow_lo", `{"int64":"-9223372036854775809"}`,
		9, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"int64":"3.14"}`,
		9, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"int64":"3e2"}`,
		9, jscandec.ErrUnexpectedValue)
}

func TestDecodeStringTagUint(t *testing.T) {
	type S struct {
		Uint uint `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, S{})
	s.TestOK(t, "null", `null`, S{})
	s.TestOK(t, "min", `{"uint":"0"}`, S{Uint: 0})
	s.TestOK(t, "max", `{"uint":"18446744073709551615"}`, S{Uint: 18446744073709551615})

	s.testErr(t, "overflow_hi", `{"uint":"18446744073709551616"}`,
		8, jscandec.ErrUnexpectedValue)
	s.testErr(t, "negative", `{"uint":"-1"}`,
		8, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"uint":"3.14"}`,
		8, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"uint":"3e2"}`,
		8, jscandec.ErrUnexpectedValue)
}

func TestDecodeStringTagUint8(t *testing.T) {
	type S struct {
		Uint8 uint8 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, S{})
	s.TestOK(t, "null", `null`, S{})
	s.TestOK(t, "min", `{"uint8":"0"}`, S{Uint8: 0})
	s.TestOK(t, "max", `{"uint8":"255"}`, S{Uint8: 255})

	s.testErr(t, "overflow_hi", `{"uint8":"256"}`,
		9, jscandec.ErrUnexpectedValue)
	s.testErr(t, "negative", `{"uint8":"-1"}`,
		9, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"uint8":"3.14"}`,
		9, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"uint8":"3e2"}`,
		9, jscandec.ErrUnexpectedValue)
}

func TestDecodeStringTagUint16(t *testing.T) {
	type S struct {
		Uint16 uint16 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, S{})
	s.TestOK(t, "null", `null`, S{})
	s.TestOK(t, "min", `{"uint16":"0"}`, S{Uint16: 0})
	s.TestOK(t, "max", `{"uint16":"65535"}`, S{Uint16: 65535})

	s.testErr(t, "overflow_hi", `{"uint16":"65536"}`,
		10, jscandec.ErrUnexpectedValue)
	s.testErr(t, "negative", `{"uint16":"-1"}`,
		10, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"uint16":"3.14"}`,
		10, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"uint16":"3e2"}`,
		10, jscandec.ErrUnexpectedValue)
}

func TestDecodeStringTagUint32(t *testing.T) {
	type S struct {
		Uint32 uint32 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, S{})
	s.TestOK(t, "null", `null`, S{})
	s.TestOK(t, "min", `{"uint32":"0"}`, S{Uint32: 0})
	s.TestOK(t, "max", `{"uint32":"4294967295"}`,
		S{Uint32: 4294967295})

	s.testErr(t, "overflow_hi", `{"uint32":"4294967296"}`,
		10, jscandec.ErrUnexpectedValue)
	s.testErr(t, "negative", `{"uint32":"-1"}`,
		10, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"uint32":"3.14"}`,
		10, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"uint32":"3e2"}`,
		10, jscandec.ErrUnexpectedValue)
}

func TestDecodeStringTagUint64(t *testing.T) {
	type S struct {
		Uint64 uint64 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, S{})
	s.TestOK(t, "null", `null`, S{})
	s.TestOK(t, "min", `{"uint64":"0"}`, S{Uint64: 0})
	s.TestOK(t, "max", `{"uint64":"18446744073709551615"}`,
		S{Uint64: 18446744073709551615})

	s.testErr(t, "overflow_hi", `{"uint64":"18446744073709551616"}`,
		10, jscandec.ErrUnexpectedValue)
	s.testErr(t, "negative", `{"uint64":"-1"}`,
		10, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"uint64":"3.14"}`,
		10, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"uint64":"3e2"}`,
		10, jscandec.ErrUnexpectedValue)
}

func TestDecodeStringTagFloat32(t *testing.T) {
	type S struct {
		Float32 float32 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, S{})
	s.TestOK(t, "null", `null`, S{})
	s.TestOK(t, "zero", `{"float32":"0"}`, S{Float32: 0})
	s.TestOK(t, "integer", `{"float32":"123"}`, S{Float32: 123})
	s.TestOK(t, "-pi7", `{"float32":"-3.1415927"}`, S{Float32: -3.1415927})
	s.TestOK(t, "avogadros_num", `{"float32":"6.022e23"}`, S{Float32: 6.022e23})

	s.testErrCheck(t, "range_hi", `{"float32":"3.5e38"}`,
		func(t *testing.T, errIndex int, err error) {
			require.ErrorIs(t, err, strconv.ErrRange)
			require.Equal(t, 11, errIndex)
		})
}

func TestDecodeStringTagFloat64(t *testing.T) {
	type S struct {
		Float64 float64 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, S{})
	s.TestOK(t, "null", `null`, S{})
	s.TestOK(t, "zero", `{"float64":"0"}`, S{Float64: 0})
	s.TestOK(t, "integer", `{"float64":"123"}`, S{Float64: 123})
	s.TestOK(t, "1.000000003", `{"float64":"1.000000003"}`,
		S{Float64: 1.000000003})
	s.TestOK(t, "max_int", `{"float64":"9007199254740991"}`,
		S{Float64: 9_007_199_254_740_991})
	s.TestOK(t, "min_int", `{"float64":"-9007199254740991"}`,
		S{Float64: -9_007_199_254_740_991})
	s.TestOK(t, "pi",
		`{"float64":"3.141592653589793238462643383279502884197"}`,
		S{Float64: 3.141592653589793238462643383279502884197})
	s.TestOK(t, "pi_neg",
		`{"float64":"-3.141592653589793238462643383279502884197"}`,
		S{Float64: -3.141592653589793238462643383279502884197})
	s.TestOK(t, "3.4028235e38", `{"float64":"3.4028235e38"}`,
		S{Float64: 3.4028235e38})
	s.TestOK(t, "exponent", `{"float64":"1.7976931348623157e308"}`,
		S{Float64: 1.7976931348623157e308})
	s.TestOK(t, "neg_exponent", `{"float64":"1.7976931348623157e-308"}`,
		S{Float64: 1.7976931348623157e-308})
	s.TestOK(t, "1.4e-45", `{"float64":"1.4e-45"}`,
		S{Float64: 1.4e-45})
	s.TestOK(t, "neg_exponent", `{"float64":"-1.7976931348623157e308"}`,
		S{Float64: -1.7976931348623157e308})
	s.TestOK(t, "3.4e38", `{"float64":"3.4e38"}`,
		S{Float64: 3.4e38})
	s.TestOK(t, "-3.4e38", `{"float64":"-3.4e38"}`,
		S{Float64: -3.4e38})
	s.TestOK(t, "avogadros_num", `{"float64":"6.022e23"}`,
		S{Float64: 6.022e23})

	s.testErrCheck(t, "range_hi", `{"float64":"1e309"}`,
		func(t *testing.T, errIndex int, err error) {
			require.ErrorIs(t, err, strconv.ErrRange)
			require.Equal(t, 11, errIndex)
		})
}

func TestDecodePointerInt(t *testing.T) {
	s := newTestSetup[*int](t, *jscandec.DefaultOptions)
	s.TestOK(t, "valid", `42`, Ptr(int(42)))
	s.TestOK(t, "null", `null`, (*int)(nil))

	s.testErr(t, "float", `1.1`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object", `{}`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodePointerBool(t *testing.T) {
	s := newTestSetup[*bool](t, *jscandec.DefaultOptions)
	s.TestOK(t, "valid_true", `true`, Ptr(bool(true)))
	s.TestOK(t, "valid_false", `false`, Ptr(bool(false)))
	s.TestOK(t, "null", `null`, (*bool)(nil))

	s.testErr(t, "float", `1.1`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object", `{}`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `42`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodePointerFloat64(t *testing.T) {
	s := newTestSetup[*float64](t, *jscandec.DefaultOptions)
	s.TestOK(t, "valid_int", `42`, Ptr(float64(42)))
	s.TestOK(t, "valid_num", `3.1415`, Ptr(float64(3.1415)))
	s.TestOK(t, "null", `null`, (*float64)(nil))

	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object", `{}`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodePointerString(t *testing.T) {
	s := newTestSetup[*string](t, *jscandec.DefaultOptions)
	s.TestOK(t, "valid", `"text"`, Ptr(string("text")))
	s.TestOK(t, "empty", `""`, Ptr(string("")))
	s.TestOK(t, "null", `null`, (*string)(nil))

	s.testErr(t, "float", `1.1`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object", `{}`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `42`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodePointerSlice(t *testing.T) {
	s := newTestSetup[*[]int](t, *jscandec.DefaultOptions)
	s.TestOK(t, "valid", `[1]`, Ptr([]int{1}))
	s.TestOK(t, "empty", `[]`, Ptr([]int{}))
	s.TestOK(t, "null", `null`, (*[]int)(nil))

	s.testErr(t, "float", `1.1`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object", `{}`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `42`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodePointerStruct(t *testing.T) {
	type S struct {
		Foo string `json:"foo"`
		Bar any    `json:"bar"`
	}
	s := newTestSetup[*S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "valid", `{"foo":"™","bar":[1,true]}`, &S{
		Foo: "™",
		Bar: []any{float64(1), true},
	})
	s.TestOK(t, "partial", `{"bar":[1,true]}`, &S{Bar: []any{float64(1), true}})
	s.TestOK(t, "empty", `{}`, &S{})
	s.TestOK(t, "null", `null`, (*S)(nil))

	s.testErr(t, "float", `1.1`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `42`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodePointerAny(t *testing.T) {
	s := newTestSetup[*any](t, *jscandec.DefaultOptions)
	s.TestOK(t, "int", `[1]`, Ptr(any([]any{float64(1)})))
	s.TestOK(t, "string", `"text"`, Ptr(any("text")))
	s.TestOK(t, "array_int", `[1]`, Ptr(any([]any{float64(1)})))
	s.TestOK(t, "array_int", `{"foo":1}`, Ptr(any(map[string]any{"foo": float64(1)})))
}

func TestDecodePointer3DInt(t *testing.T) {
	s := newTestSetup[***int](t, *jscandec.DefaultOptions)
	s.TestOK(t, "valid", `42`, Ptr(Ptr(Ptr(int(42)))))

	s.testErr(t, "float", `1.1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
}

func Ptr[T any](v T) *T { return &v }

func TestDecodeStructSlice(t *testing.T) {
	type S struct {
		Foo int    `json:"foo"`
		Bar string `json:"bar"`
	}
	s := newTestSetup[[]S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty_array",
		`[]`, []S{})
	s.TestOK(t, "regular_field_order",
		`[{"foo":42,"bar":"bazz"}]`, []S{{Foo: 42, Bar: "bazz"}})
	s.TestOK(t, "multiple",
		`[
			{"foo": 1, "bar": "a"},
			{"foo": 2, "bar": "ab"},
			{"foo": 3, "bar": "abc"},
			{"foo": 4, "bar": "abcd"},
			{"foo": 5, "bar": "abcde"},
			{"foo": 6, "bar": "abcdef"},
			{"foo": 7, "bar": "abcdefg"}
		]`, []S{
			{Foo: 1, Bar: "a"},
			{Foo: 2, Bar: "ab"},
			{Foo: 3, Bar: "abc"},
			{Foo: 4, Bar: "abcd"},
			{Foo: 5, Bar: "abcde"},
			{Foo: 6, Bar: "abcdef"},
			{Foo: 7, Bar: "abcdefg"},
		})
	s.TestOK(t, "empty_null_and_unknown_fields",
		`[
			{ },
			null,
			{"faz":42,"baz":"bazz"}
		]`, []S{{}, {}, {}})
	s.TestOK(t, "mixed",
		`[
			{"bar":"abc","foo":1234},
			{"FOO":42,"BAR":"bazz"},
			{"Foo":42,"Bar":"bazz"},
			{"foo":null,"bar":null},
			{"bar":"bar"},
			{"foo":12345},
			{"bar":"bar","unknown":42,"foo":102},
			{"unknown":42, "unknown2": "bad"},
			{},
			{"faz":42,"baz":"bazz"}
		]`, []S{
			{Foo: 1234, Bar: "abc"},
			{Foo: 42, Bar: "bazz"},
			{Foo: 42, Bar: "bazz"},
			{Foo: 0, Bar: ""},
			{Bar: "bar"},
			{Foo: 12345},
			{Foo: 102, Bar: "bar"},
			{},
			{},
			{},
		})

	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeMapStringToString(t *testing.T) {
	type M map[string]string
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, M{})
	s.TestOK(t, "2_pairs",
		`{"foo":"42","bar":"bazz"}`, M{"foo": "42", "bar": "bazz"})
	s.TestOK(t, "empty_strings",
		`{"":""}`, M{"": ""})
	s.TestOK(t, "multiple_empty_strings",
		`{"":"", "":""}`, M{"": ""})
	s.TestOK(t, "null_value",
		`{"":null}`, M{"": ""})
	s.TestOK(t, "duplicate_values",
		`{"a":"1","a":"2"}`, M{"a": "2"}) // Take last
	s.TestOK(t, "multiple_overrides",
		`{"":"1", "":"12", "":"123"}`, M{"": "123"}) // Take last
	s.TestOK(t, "many",
		`{
			"foo": "1", "bar": "a", "baz": "2", "muzz": "",
			"longer_key": "longer test text"
		}`, M{
			"foo": "1", "bar": "a", "baz": "2", "muzz": "",
			"longer_key": "longer test text",
		})
	s.TestOK(t, "escaped",
		`{"\"key\"":"\"value\"\t\u0042"}`, M{"\"key\"": "\"value\"\tB"})

	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeMapIntToString(t *testing.T) {
	type M map[int]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, M{})
	s.TestOK(t, "null", `null`, M(nil))
	s.TestOK(t, "positive_and_negative", `{"0":0, "42":42, "-123456789":123456789}`,
		M{0: 0, 42: 42, -123456789: 123456789})

	s.TestOKPrepare(t, "overwrite_1", `{"1": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 20},
	})
	s.TestOKPrepare(t, "overwrite_2", `{"2": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 10, 2: 42},
	})
	s.TestOKPrepare(t, "overwrite_all", `{"2": 42, "1": 42, "3": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 42, 3: 42},
	})

	s.testErr(t, "overflow_hi", `{"9223372036854775808":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "overflow_lo", `{"-9223372036854775809":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"3.14":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"3e2":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeMapInt8ToString(t *testing.T) {
	type M map[int8]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, M{})
	s.TestOK(t, "null", `null`, M(nil))
	s.TestOK(t, "min_and_max", `{"0":0, "-128":-128, "127":127}`,
		M{0: 0, -128: -128, 127: 127})

	s.TestOKPrepare(t, "overwrite_1", `{"1": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 20},
	})
	s.TestOKPrepare(t, "overwrite_2", `{"2": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 10, 2: 42},
	})
	s.TestOKPrepare(t, "overwrite_all", `{"2": 42, "1": 42, "3": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 42, 3: 42},
	})

	s.testErr(t, "overflow_hi", `{"128":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "overflow_lo", `{"-129":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"3.14":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"3e2":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeMapInt16ToString(t *testing.T) {
	type M map[int16]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, M{})
	s.TestOK(t, "null", `null`, M(nil))
	s.TestOK(t, "min_and_max", `{"0":0, "-32768":-32768, "32767":32767}`,
		M{0: 0, -32768: -32768, 32767: 32767})

	s.TestOKPrepare(t, "overwrite_1", `{"1": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 20},
	})
	s.TestOKPrepare(t, "overwrite_2", `{"2": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 10, 2: 42},
	})
	s.TestOKPrepare(t, "overwrite_all", `{"2": 42, "1": 42, "3": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 42, 3: 42},
	})

	s.testErr(t, "overflow_hi", `{"32768":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "overflow_lo", `{"-32769":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"3.14":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"3e2":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeMapInt32ToString(t *testing.T) {
	type M map[int32]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, M{})
	s.TestOK(t, "null", `null`, M(nil))
	s.TestOK(t, "min_and_max", `{"0":0,
		"-2147483648":-2147483648, "2147483647":2147483647}`,
		M{0: 0, -2147483648: -2147483648, 2147483647: 2147483647})

	s.TestOKPrepare(t, "overwrite_1", `{"1": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 20},
	})
	s.TestOKPrepare(t, "overwrite_2", `{"2": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 10, 2: 42},
	})
	s.TestOKPrepare(t, "overwrite_all", `{"2": 42, "1": 42, "3": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 42, 3: 42},
	})

	s.testErr(t, "overflow_hi", `{"2147483648":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "overflow_lo", `{"-2147483649":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"3.14":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"3e2":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeMapInt64ToString(t *testing.T) {
	type M map[int64]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, M{})
	s.TestOK(t, "null", `null`, M(nil))
	s.TestOK(t, "min_and_max", `{"0":0,
		"-9223372036854775808":-9223372036854775808,
		"9223372036854775807":9223372036854775807}`,
		M{
			0:                    0,
			-9223372036854775808: -9223372036854775808,
			9223372036854775807:  9223372036854775807,
		})

	s.TestOKPrepare(t, "overwrite_1", `{"1": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 20},
	})
	s.TestOKPrepare(t, "overwrite_2", `{"2": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 10, 2: 42},
	})
	s.TestOKPrepare(t, "overwrite_all", `{"2": 42, "1": 42, "3": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 42, 3: 42},
	})

	s.testErr(t, "overflow_hi", `{"9223372036854775808":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "overflow_lo", `{"-9223372036854775809":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"3.14":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"3e2":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeMapUintToString(t *testing.T) {
	type M map[uint]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, M{})
	s.TestOK(t, "null", `null`, M(nil))
	s.TestOK(t, "positive_and_negative", `{"0":0, "42":42, "18446744073709551615":1}`,
		M{0: 0, 42: 42, 18446744073709551615: 1})

	s.TestOKPrepare(t, "overwrite_1", `{"1": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 20},
	})
	s.TestOKPrepare(t, "overwrite_2", `{"2": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 10, 2: 42},
	})
	s.TestOKPrepare(t, "overwrite_all", `{"2": 42, "1": 42, "3": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 42, 3: 42},
	})

	s.testErr(t, "overflow", `{"18446744073709551616":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "negative", `{"-1":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"3.14":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"3e2":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeMapUint8ToString(t *testing.T) {
	type M map[uint8]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, M{})
	s.TestOK(t, "null", `null`, M(nil))
	s.TestOK(t, "positive_and_negative", `{"0":0, "42":42, "255":1}`,
		M{0: 0, 42: 42, 255: 1})

	s.TestOKPrepare(t, "overwrite_1", `{"1": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 20},
	})
	s.TestOKPrepare(t, "overwrite_2", `{"2": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 10, 2: 42},
	})
	s.TestOKPrepare(t, "overwrite_all", `{"2": 42, "1": 42, "3": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 42, 3: 42},
	})

	s.testErr(t, "overflow", `{"256":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "negative", `{"-1":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"3.14":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"3e2":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeMapUint16ToString(t *testing.T) {
	type M map[uint16]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, M{})
	s.TestOK(t, "null", `null`, M(nil))
	s.TestOK(t, "positive_and_negative", `{"0":0, "42":42, "65535":1}`,
		M{0: 0, 42: 42, 65535: 1})

	s.TestOKPrepare(t, "overwrite_1", `{"1": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 20},
	})
	s.TestOKPrepare(t, "overwrite_2", `{"2": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 10, 2: 42},
	})
	s.TestOKPrepare(t, "overwrite_all", `{"2": 42, "1": 42, "3": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 42, 3: 42},
	})

	s.testErr(t, "overflow", `{"65536":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "negative", `{"-1":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"3.14":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"3e2":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeMapUint32ToString(t *testing.T) {
	type M map[uint32]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, M{})
	s.TestOK(t, "null", `null`, M(nil))
	s.TestOK(t, "positive_and_negative", `{"0":0, "42":42, "4294967295":1}`,
		M{0: 0, 42: 42, 4294967295: 1})

	s.TestOKPrepare(t, "overwrite_1", `{"1": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 20},
	})
	s.TestOKPrepare(t, "overwrite_2", `{"2": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 10, 2: 42},
	})
	s.TestOKPrepare(t, "overwrite_all", `{"2": 42, "1": 42, "3": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 42, 3: 42},
	})

	s.testErr(t, "overflow", `{"4294967296":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "negative", `{"-1":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"3.14":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"3e2":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeMapUint64ToString(t *testing.T) {
	type M map[uint64]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, M{})
	s.TestOK(t, "null", `null`, M(nil))
	s.TestOK(t, "positive_and_negative", `{"0":0, "42":42, "18446744073709551615":1}`,
		M{0: 0, 42: 42, 18446744073709551615: 1})

	s.TestOKPrepare(t, "overwrite_1", `{"1": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 20},
	})
	s.TestOKPrepare(t, "overwrite_2", `{"2": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 10, 2: 42},
	})
	s.TestOKPrepare(t, "overwrite_all", `{"2": 42, "1": 42, "3": 42}`, Test[M]{
		PrepareJscan: func() M { return M{1: 10, 2: 20} },
		Expect:       M{1: 42, 2: 42, 3: 42},
	})

	s.testErr(t, "overflow", `{"18446744073709551616":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "negative", `{"-1":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `{"3.14":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "exponent", `{"3e2":0}`,
		1, jscandec.ErrUnexpectedValue)
	s.testErr(t, "int", `1`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
}

func TestDecodeMapStringToMapStringToString(t *testing.T) {
	type M2 map[string]string
	type M map[string]M2
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, M{})
	s.TestOK(t, "2_pairs",
		`{
			"a":{"a1":"a1val","a2":"a2val"},
			"b":{"b1":"b1val","b2":"b2val"}
		}`,
		M{
			"a": M2{"a1": "a1val", "a2": "a2val"},
			"b": M2{"b1": "b1val", "b2": "b2val"},
		})
	s.TestOK(t, "empty_strings",
		`{"":{"":""}}`, M{"": M2{"": ""}})
	s.TestOK(t, "multiple_empty_strings",
		`{"":{"":"", "":""}, "":{"":"", "":""}}`,
		M{"": M2{"": ""}})
	s.TestOK(t, "null_value",
		`{"n":null,"x":{"x":null}}`, M{"n": nil, "x": M2{"x": ""}})
	s.TestOK(t, "duplicate_values",
		`{"a":{"foo":"bar"},"a":{"baz":"faz"}}`,
		M{"a": M2{"baz": "faz"}}) // Take last
	s.TestOK(t, "multiple_overrides",
		`{"":{"a":"b"}, "":{"c":"d"}, "":{"e":"f"}}`,
		M{"": {"e": "f"}}) // Take last
	s.TestOK(t, "mixed",
		`{
			"":{},
			"first_key":{"f1":"first1_value","f2":"first2_value"},
			"second_key":null
		}`, M{
			"":           M2{},
			"first_key":  M2{"f1": "first1_value", "f2": "first2_value"},
			"second_key": nil,
		})
	s.TestOK(t, "escaped",
		`{" \b " : {" \" ":" \u30C4 "} }`, M{" \b ": M2{` " `: ` ツ `}})

	s.TestOKPrepare(t, "overwrite_a_empty", `{"a": {}}`, Test[M]{
		PrepareJscan: func() M {
			return M{"a": {"a1": "a1_b", "a2": "a2_v"}, "b": {"b1": "b1_v"}}
		},
		Expect: M{"a": {}, "b": {"b1": "b1_v"}},
	})
	s.TestOKPrepare(t, "overwrite_a_partially", `{"a": {"a2":"NEWVAL"}}`, Test[M]{
		PrepareJscan: func() M {
			return M{"a": {"a1": "a1_b", "a2": "a2_v"}, "b": {"b1": "b1_v"}}
		},
		Expect: M{"a": {"a2": "NEWVAL"}, "b": {"b1": "b1_v"}},
	})
	s.TestOKPrepare(t, "overwrite_all",
		`{"a": {"a2":"NEWVAL1","a1":"NEWVAL2"}, "b": {"b1": "NEWVAL3", "bNEW": "NV3"} }`,
		Test[M]{
			PrepareJscan: func() M {
				return M{"a": {"a1": "a1_b", "a2": "a2_v"}, "b": {"b1": "b1_v"}}
			},
			Expect: M{
				"a": {"a1": "NEWVAL2", "a2": "NEWVAL1"},
				"b": {"b1": "NEWVAL3", "bNEW": "NV3"},
			},
		})

	s.testErr(t, "int", `1`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "map_string_string", `{"foo":"bar"}`, 7, jscandec.ErrUnexpectedValue)
	s.testErr(t, "map_string_map_string_int", `{"foo":{"bar":42}}`,
		14, jscandec.ErrUnexpectedValue)
}

func TestDecodeMapStringToStruct(t *testing.T) {
	type S struct {
		Name string `json:"name"`
		ID   int    `json:"id"`
	}
	type M map[string]S
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, M{})
	s.TestOK(t, "one",
		`{"x":{"name":"first","id":1}}`, M{"x": S{Name: "first", ID: 1}})
	s.TestOK(t, "empty_struct",
		`{"x":{}}`, M{"x": {}})
	s.TestOK(t, "null_value",
		`{"":null}`, M{"": {}})
	s.TestOK(t, "escaped_key",
		`{"\u30c4":{}}`, M{`ツ`: {}})
	s.TestOK(t, "multiple",
		`{
			"x":{"name":"first","id":1},
			"y":{"name":"second","id":2}
		}`, M{
			"x": S{Name: "first", ID: 1},
			"y": S{Name: "second", ID: 2},
		})

	s.TestOKPrepare(t, "overwrite_null", `null`, Test[M]{
		PrepareJscan: func() M {
			return M{"a": {Name: "a1", ID: 1}, "b": {Name: "b1", ID: 2}}
		},
		Expect: M(nil),
	})
	s.TestOKPrepare(t, "no_overwrite_empty", `{}`, Test[M]{
		PrepareJscan: func() M {
			return M{
				"a": {Name: "a1", ID: 1},
				"b": {Name: "b1", ID: 2},
			}
		},
		Expect: M{
			"a": {Name: "a1", ID: 1},
			"b": {Name: "b1", ID: 2},
		},
	})
	s.TestOKPrepare(t, "no_overwrite_new", `{"c":{"id":3, "name":"Bob"}}`, Test[M]{
		PrepareJscan: func() M {
			return M{
				"a": {Name: "a1", ID: 1},
				"b": {Name: "b1", ID: 2},
			}
		},
		Expect: M{
			"a": {Name: "a1", ID: 1},
			"b": {Name: "b1", ID: 2},
			"c": {Name: "Bob", ID: 3},
		},
	})
	s.TestOKPrepare(t, "overwrite_a_empty", `{"a": {}}`, Test[M]{
		PrepareJscan: func() M {
			return M{"a": {Name: "a1", ID: 1}, "b": {Name: "b1", ID: 2}}
		},
		Expect: M{"a": {}, "b": {Name: "b1", ID: 2}},
	})
	s.TestOKPrepare(t, "overwrite_a_partially", `{"a": {"id":42}}`, Test[M]{
		PrepareJscan: func() M {
			return M{"a": {Name: "a1", ID: 1}, "b": {Name: "b1", ID: 2}}
		},
		Expect: M{"a": {ID: 42}, "b": {Name: "b1", ID: 2}},
	})

	s.testErr(t, "int", `1`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "non_object_element", `{"x":42}`, 5, jscandec.ErrUnexpectedValue)
}

func TestDecodeMapStringStruct512(t *testing.T) {
	type D [512]byte
	type S struct{ Data D }
	type M map[string]S
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, M{})
	s.TestOK(t, "null", `null`, M(nil))
	s.TestOK(t, "one",
		`{"x":{"data":[1,2,3]}}`, M{"x": S{Data: D{0: 1, 1: 2, 2: 3}}})
	s.TestOK(t, "empty_struct",
		`{"x":{}}`, M{"x": {}})
	s.TestOK(t, "null_value",
		`{"":null}`, M{"": {}})
	s.TestOK(t, "escaped_key",
		`{"\u30c4":{}}`, M{`ツ`: {}})
	s.TestOK(t, "multiple",
		`{
			"x":{"data":[0,1,2]},
			"y":{"data":[3,4]}
		}`, M{
			"x": S{Data: D{0: 0, 1: 1, 2: 2}},
			"y": S{Data: D{0: 3, 1: 4}},
		})

	s.TestOKPrepare(t, "overwrite_null", `null`, Test[M]{
		PrepareJscan: func() M { return M{"a": S{Data: D{0}}, "b": {Data: D{0}}} },
		Expect:       M(nil),
	})
	s.TestOKPrepare(t, "no_overwrite_empty", `{}`, Test[M]{
		PrepareJscan: func() M { return M{"a": {Data: D{0}}, "b": {Data: D{1}}} },
		Expect:       M{"a": {Data: D{0}}, "b": {Data: D{1}}},
	})
	s.TestOKPrepare(t, "no_overwrite_new", `{"c":{"data":[1]}}`, Test[M]{
		PrepareJscan: func() M { return M{"a": {Data: D{0}}, "b": {Data: D{0}}} },
		Expect:       M{"a": {Data: D{0}}, "b": {Data: D{0}}, "c": {Data: D{1}}},
	})
	s.TestOKPrepare(t, "overwrite_a_empty", `{"a": {}}`, Test[M]{
		PrepareJscan: func() M { return M{"a": {Data: D{0}}, "b": {Data: D{1}}} },
		Expect:       M{"a": {}, "b": {Data: D{1}}},
	})
	s.TestOKPrepare(t, "overwrite_a", `{"a": {"data":[9,9]}}`, Test[M]{
		PrepareJscan: func() M {
			return M{"a": {Data: D{0, 1, 2, 3, 4, 5}}, "b": {Data: D{1}}}
		},
		Expect: M{"a": {Data: D{9, 9}}, "b": {Data: D{1}}},
	})

	s.testErr(t, "int", `1`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "string", `"text"`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "non_array_element", `{"x":42}`, 5, jscandec.ErrUnexpectedValue)
}

func TestDecodeJSONUnmarshaler(t *testing.T) {
	s := newTestSetup[jsonUnmarshalerImpl](t, *jscandec.DefaultOptions)
	s.TestOK(t, "integer", `123`, jsonUnmarshalerImpl{Value: `123`})
	s.TestOK(t, "float", `3.14`, jsonUnmarshalerImpl{Value: `3.14`})
	s.TestOK(t, "string", `"okay"`, jsonUnmarshalerImpl{Value: `"okay"`})
	s.TestOK(t, "true", `true`, jsonUnmarshalerImpl{Value: `true`})
	s.TestOK(t, "false", `false`, jsonUnmarshalerImpl{Value: `false`})
	s.TestOK(t, "null", `null`, jsonUnmarshalerImpl{Value: `null`})
	s.TestOK(t, "array_empty", `[]`, jsonUnmarshalerImpl{Value: `[]`})
	s.TestOK(t, "array", `[1,"okay",true,{ }]`,
		jsonUnmarshalerImpl{Value: `[1,"okay",true,{ }]`})
	s.TestOK(t, "object_empty", `{}`, jsonUnmarshalerImpl{Value: `{}`})
	s.TestOK(t, "object_empty", `{"foo":{"bar":"baz"}}`,
		jsonUnmarshalerImpl{Value: `{"foo":{"bar":"baz"}}`})
}

func TestDecodeTextUnmarshaler(t *testing.T) {
	s := newTestSetup[textUnmarshalerImpl](t, *jscandec.DefaultOptions)
	s.TestOK(t, "string", `"text"`, textUnmarshalerImpl{Value: `text`})
	s.TestOK(t, "string_escaped", `"\"text\""`, textUnmarshalerImpl{Value: `"text"`})
	s.TestOK(t, "null", `null`, textUnmarshalerImpl{Value: ``})
	s.TestOK(t, "string_empty", `""`, textUnmarshalerImpl{Value: ``})

	s.testErr(t, "int", `123`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `3.14`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array_empty", `[]`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `["foo"]`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object_empty", `{}`, 0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object", `{"foo":"bar"}`, 0, jscandec.ErrUnexpectedValue)
}

func TestDecodeTextUnmarshalerMapKey(t *testing.T) {
	type U = textUnmarshalerImpl
	s := newTestSetup[map[U]int](t, *jscandec.DefaultOptions)
	s.TestOK(t, "empty", `{}`, map[U]int{})
	s.TestOK(t, "null", `null`, map[U]int(nil))
	s.TestOK(t, "text", `{"text":1}`, map[U]int{{Value: "text"}: 1})
	s.TestOK(t, "empty_key", `{"":2}`, map[U]int{{Value: ""}: 2})
	s.TestOK(t, "escaped", `{"\"escaped\tkey\"":3}`,
		map[U]int{{Value: "\"escaped\tkey\""}: 3})

	s.testErr(t, "int", `123`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "float", `3.14`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "true", `true`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array_empty", `[]`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `["foo"]`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object", `{"foo":"bar"}`,
		7, jscandec.ErrUnexpectedValue)
}

func TestDecodeUnmarshalerFields(t *testing.T) {
	type S struct {
		String string              `json:"string"`
		JSON   jsonUnmarshalerImpl `json:"json"`
		Text   textUnmarshalerImpl `json:"text"`
		Tail   []int               `json:"tail"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.TestOK(t, "integer",
		`{"string":"a","json":42,"text":"foo","tail":[1,2]}`,
		S{
			String: "a",
			JSON:   jsonUnmarshalerImpl{Value: `42`},
			Text:   textUnmarshalerImpl{Value: "foo"},
			Tail:   []int{1, 2},
		})
	s.TestOK(t, "string", `{
		"string":"b",
		"json":"\"text\"",
		"text":"\"text\"",
		"tail":[1,2]}`,
		S{
			String: "b",
			JSON:   jsonUnmarshalerImpl{Value: `"\"text\""`},
			Text:   textUnmarshalerImpl{Value: `"text"`},
			Tail:   []int{1, 2},
		})
	s.TestOK(t, "array", `{"string":"c","json":[1,2, 3],"text":"","tail":[1,2]}`,
		S{
			String: "c",
			JSON:   jsonUnmarshalerImpl{Value: `[1,2, 3]`},
			Text:   textUnmarshalerImpl{Value: ""},
			Tail:   []int{1, 2},
		})
	s.TestOK(t, "object", `{"string":"d","json":{"foo":["bar", null]},"tail":[1,2]}`,
		S{
			String: "d",
			JSON:   jsonUnmarshalerImpl{Value: `{"foo":["bar", null]}`},
			Text:   textUnmarshalerImpl{Value: ""},
			Tail:   []int{1, 2},
		})
}

func TestDecodeJSONUnmarshalerErr(t *testing.T) {
	s := newTestSetup[unmarshalerImplErr](t, *jscandec.DefaultOptions)
	s.testErrCheck(t, "integer", `123`, func(t *testing.T, errIndex int, err error) {
		require.Equal(t, errUnmarshalerImpl, err)
	})

	s2 := newTestSetup[map[textUnmarshalerImplErr]struct{}](t, *jscandec.DefaultOptions)
	s2.testErrCheck(t, "map", `{"x":"y"}`, func(t *testing.T, errIndex int, err error) {
		require.Equal(t, errTextUnmarshalerImpl, err)
	})

	type S struct{ Unmarshaler textUnmarshalerImplErr }
	s3 := newTestSetup[S](t, *jscandec.DefaultOptions)
	s3.testErrCheck(t, "struct_field", `{"Unmarshaler":"abc"}`,
		func(t *testing.T, errIndex int, err error) {
			require.Equal(t, errTextUnmarshalerImpl, err)
		})
}

func TestDecodeSyntaxErrorUnexpectedEOF(t *testing.T) {
	tokenizerString := jscan.NewTokenizer[string](16, 1024)
	d, err := jscandec.NewDecoder[string, []int](
		tokenizerString, jscandec.DefaultInitOptions,
	)
	require.NoError(t, err)
	var v []int
	_, err = d.Decode(`[1,2,3`, &v, jscandec.DefaultOptions)
	var jscanErr jscan.Error[string]
	require.True(t, errors.As(err, &jscanErr))
	require.Equal(t, jscan.ErrorCodeUnexpectedEOF, jscanErr.Code)
}

func BenchmarkSmall(b *testing.B) {
	in := []byte(`[[true],[false,false,false,false],[],[],[true]]`) // 18 tokens

	b.Run("jscan", func(b *testing.B) {
		tok := jscan.NewTokenizer[[]byte](8, 64)
		d, err := jscandec.NewDecoder[[]byte, [][]bool](tok, jscandec.DefaultInitOptions)
		if err != nil {
			b.Fatalf("initializing decoder: %v", err)
		}
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			var v [][]bool
			if _, err := d.Decode(in, &v, jscandec.DefaultOptions); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("jsoniter", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v [][]bool
			if err := jsoniter.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func skipIfNot64bitSystem(t *testing.T) {
	if uintptr(8) != unsafe.Sizeof(int(0)) {
		t.Skip("this test must run on a 64-bit system")
	}
}

// jsonUnmarshalerImpl implements encoding/json.Unmarshaler.
type jsonUnmarshalerImpl struct{ Value string }

func (impl *jsonUnmarshalerImpl) UnmarshalJSON(data []byte) error {
	impl.Value = string(data)
	return nil
}

// unmarshalerImplErr implements encoding/json.Unmarshaler and always returns an error.
type unmarshalerImplErr struct{ Value string }

func (impl *unmarshalerImplErr) UnmarshalJSON(data []byte) error {
	return errUnmarshalerImpl
}

var errUnmarshalerImpl = errors.New("unmarshalerImplErr test error")

// textUnmarshalerImpl implements encoding/json.Unmarshaler.
type textUnmarshalerImpl struct{ Value string }

func (impl *textUnmarshalerImpl) UnmarshalText(text []byte) error {
	impl.Value = string(text)
	return nil
}

// textUnmarshalerImplErr implements encoding/json.Unmarshaler.
type textUnmarshalerImplErr struct{ Value string }

func (impl *textUnmarshalerImplErr) UnmarshalText(text []byte) error {
	return errTextUnmarshalerImpl
}

var errTextUnmarshalerImpl = errors.New("textUnmarshalerImplErr test error")

func TestErrStringTagOptionOnUnsupportedType(t *testing.T) {
	type S struct {
		//lint:ignore SA5008 the JSON string option is used intentionally
		Unsupported map[string]string `json:",string"` //nolint:staticcheck
	}

	testIn := `{"unsupported":{"not":"okay"}}`
	var v S
	require.NoError(t, json.Unmarshal([]byte(testIn), &v))

	t.Run("check_disabled", func(t *testing.T) {
		tok := jscan.NewTokenizer[string](1, 1)
		dec, err := jscandec.NewDecoder[string, S](tok, jscandec.DefaultInitOptions)
		require.NoError(t, err)
		require.NotNil(t, dec)
		var v S
		_, err = dec.Decode(testIn, &v, jscandec.DefaultOptions)
		require.NoError(t, err)
	})

	t.Run("err", func(t *testing.T) {
		tok := jscan.NewTokenizer[string](1, 1)
		opt := *jscandec.DefaultInitOptions
		opt.DisallowStringTagOptOnUnsupportedTypes = true
		dec, err := jscandec.NewDecoder[string, S](tok, &opt)
		require.Equal(t, err, jscandec.ErrStringTagOptionOnUnsupportedType)
		require.Nil(t, dec)
	})
}

func TestMemReuse(t *testing.T) {
	optsInit := jscandec.DefaultInitOptions
	optsDec := jscandec.DefaultOptions

	type S []string
	t.Run("string", func(t *testing.T) {
		tok := jscan.NewTokenizer[string](1, 16)
		dec, err := jscandec.NewDecoder[string, S](tok, optsInit)
		require.NoError(t, err)

		var v1 S
		_, err = dec.Decode(`["first","second","third"]`, &v1, optsDec)
		require.NoError(t, err)
		require.Equal(t, S{"first", "second", "third"}, v1)

		var v2 S
		_, err = dec.Decode(`["FIRST","SECOND","THIRD"]`, &v2, optsDec)
		require.NoError(t, err)

		require.Equal(t, S{"first", "second", "third"}, v1)
		require.Equal(t, S{"FIRST", "SECOND", "THIRD"}, v2)
	})
}

// TestDecodeNumber tests jscandec.Number, which can't be tested with a normal test
// because encoding/json.Number and jscandec.Number are different types
// and encoding/json fails to unmarshal it the same way.
func TestDecodeNumber(t *testing.T) {
	s := newTestSetup[jscandec.Number](t, *jscandec.DefaultOptions)

	testOK := func(name, input string, expect jscandec.Number) {
		s.TestOKPrepare(t, name, input, Test[jscandec.Number]{
			PrepareEncodingjson: func() any { x := json.Number(""); return &x },
			Check: func(t *testing.T, vJscan jscandec.Number, vEncodingJson any) {
				require.Equal(t, string(*vEncodingJson.(*json.Number)), string(vJscan))
				require.Equal(t, expect.String(), vJscan.String())
			},
			Expect: expect,
		})
	}

	testOK("null", `null`, "")
	testOK("0", `0`, "0")
	testOK("1", `1`, "1")
	testOK("-1", `-1`, "-1")
	testOK("int32_min", `-2147483648`, "-2147483648")
	testOK("int32_max", `2147483647`, "2147483647")
	testOK("int64_min", `-9223372036854775808`, "-9223372036854775808")
	testOK("int64_max", `9223372036854775807`, "9223372036854775807")
	testOK("overflow64_hi", `9223372036854775808`, "9223372036854775808")
	testOK("overflow64_lo", `-9223372036854775809`, "-9223372036854775809")
	testOK("bigint", `123456789012345678901234567890123456789012345678901234567890`,
		"123456789012345678901234567890123456789012345678901234567890")
	testOK("0.1", `0.1`, "0.1")
	testOK("-0.1", `-0.1`, "-0.1")
	testOK("bigdec",
		`1234567890123456789012345678901234567890123456789012345678901234567890.0`,
		"1234567890123456789012345678901234567890123456789012345678901234567890.0")
	testOK("bigdec_after_comma",
		`0.1234567890123456789012345678901234567890123456789012345678901234567890`,
		"0.1234567890123456789012345678901234567890123456789012345678901234567890")
	testOK("bigdec_after_comma",
		`1234567890123456789012345678901234567890.123456789012345678901234567890`,
		"1234567890123456789012345678901234567890.123456789012345678901234567890")
	testOK("exponent", `1234e1234`, "1234e1234")
	testOK("exponent_uppercase", `1234E1234`, "1234E1234")
	testOK("exponent_neg", `1234E-1234`, "1234E-1234")

	testOK("str_0", `"0"`, "0")
	testOK("str_1", `"1"`, "1")
	testOK("str_-1", `"-1"`, "-1")
	testOK("str_int32_min", `"-2147483648"`, "-2147483648")
	testOK("str_int32_max", `"2147483647"`, "2147483647")
	testOK("str_int64_min", `"-9223372036854775808"`, "-9223372036854775808")
	testOK("str_int64_max", `"9223372036854775807"`, "9223372036854775807")
	testOK("str_overflow64_hi", `"9223372036854775808"`, "9223372036854775808")
	testOK("str_overflow64_lo", `"-9223372036854775809"`, "-9223372036854775809")
	testOK("str_bigint",
		`"123456789012345678901234567890123456789012345678901234567890"`,
		"123456789012345678901234567890123456789012345678901234567890")
	testOK("str_0.1", `"0.1"`, "0.1")
	testOK("str_-0.1", `"-0.1"`, "-0.1")
	testOK("str_bigdec",
		`"1234567890123456789012345678901234567890123456789012345678901234567890.0"`,
		"1234567890123456789012345678901234567890123456789012345678901234567890.0")
	testOK("str_bigdec_after_comma",
		`"0.1234567890123456789012345678901234567890123456789012345678901234567890"`,
		"0.1234567890123456789012345678901234567890123456789012345678901234567890")
	testOK("str_bigdec_after_comma",
		`"1234567890123456789012345678901234567890.123456789012345678901234567890"`,
		"1234567890123456789012345678901234567890.123456789012345678901234567890")
	testOK("str_exponent", `"1234e1234"`, "1234e1234")
	testOK("str_exponent_uppercase", `"1234E1234"`, "1234E1234")
	testOK("str_exponent_neg", `"1234E-1234"`, "1234E-1234")

	s.testErr(t, "true", `true`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "false", `false`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "array", `[]`,
		0, jscandec.ErrUnexpectedValue)
	s.testErr(t, "object", `{}`,
		0, jscandec.ErrUnexpectedValue)

	testOKInt64 := func(name, input string, expect int64) {
		s.TestOKPrepare(t, name, input, Test[jscandec.Number]{
			PrepareEncodingjson: func() any { x := json.Number(""); return &x },
			Check: func(t *testing.T, vJscan jscandec.Number, vEncodingJson any) {
				require.Equal(t, string(*vEncodingJson.(*json.Number)), string(vJscan))
				a, err := vJscan.Int64()
				require.NoError(t, err)
				require.Equal(t, expect, a)
			},
		})
	}

	testOKInt64("int64_0", `0`, 0)
	testOKInt64("int64_-123", `-123`, -123)
	testOKInt64("int64_min", `-9223372036854775808`, math.MinInt64)
	testOKInt64("int64_min", `9223372036854775807`, math.MaxInt64)
	testOKInt64("str_int64_0", `"0"`, 0)
	testOKInt64("str_int64_-123", `"-123"`, -123)
	testOKInt64("str_int64_min", `"-9223372036854775808"`, math.MinInt64)
	testOKInt64("str_int64_min", `"9223372036854775807"`, math.MaxInt64)

	testOKFloat64 := func(name, input string, expect float64) {
		s.TestOKPrepare(t, name, input, Test[jscandec.Number]{
			PrepareEncodingjson: func() any { x := json.Number(""); return &x },
			Check: func(t *testing.T, vJscan jscandec.Number, vEncodingJson any) {
				require.Equal(t, string(*vEncodingJson.(*json.Number)), string(vJscan))
				a, err := vJscan.Float64()
				require.NoError(t, err)
				require.Equal(t, expect, a)
			},
		})
	}

	testOKFloat64("float64_0", `0`, 0)
	testOKFloat64("float64_-123", `-123`, -123)
	testOKFloat64("float64_min", `-9223372036854775808`, math.MinInt64)
	testOKFloat64("float64_min", `9223372036854775807`, math.MaxInt64)
	testOKFloat64("float64_1e24", `1e24`, 1e24)
	testOKFloat64("float64_pi", `3.14159`, 3.14159)
	testOKFloat64("str_float64_0", `"0"`, 0)
	testOKFloat64("str_float64_-123", `"-123"`, -123)
	testOKFloat64("str_float64_min", `"-9223372036854775808"`, math.MinInt64)
	testOKFloat64("str_float64_min", `"9223372036854775807"`, math.MaxInt64)
	testOKFloat64("str_float64_1e24", `"1e24"`, 1e24)
	testOKFloat64("str_float64_pi", `"3.14159"`, 3.14159)
}

func TestDisableFieldNameUnescaping(t *testing.T) {
	type S struct {
		ID string `json:"id"`
	}
	s := newTestSetup[S](t, jscandec.DecodeOptions{
		DisableFieldNameUnescaping: true,
	})

	s.TestOK(t, "no_unescaping", `{"id":"ok"}`, S{ID: "ok"})
	s.TestOK(t, "no_unescaping_case_insensitive_match", `{"ID":"ok"}`, S{ID: "ok"})
	s.TestOKPrepare(t, "escaped_treat_as_unknown", `{"\u0069\u0064":"not ok"}`, Test[S]{
		Check: func(t *testing.T, vJscan S, vEncodingJson any) {
			require.Equal(t, S{}, vJscan) // Ignore unknown field
			// encoding/json doesn't have an option to disable unescaping of field names.
			require.Equal(t, &S{ID: "not ok"}, vEncodingJson)
		},
	})
	s.TestOKPrepare(t, "escaped_case_insensitive", `{"\u0049\u0044":"not ok"}`, Test[S]{
		Check: func(t *testing.T, vJscan S, vEncodingJson any) {
			require.Equal(t, S{}, vJscan) // Ignore unknown field
			// encoding/json doesn't have an option to disable unescaping of field names.
			require.Equal(t, &S{ID: "not ok"}, vEncodingJson)
		},
	})
}

func TestEscapedStructTag(t *testing.T) {
	type S struct {
		ID string `json:"\u0069\u0064"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)

	s.TestOK(t, "no_unescaping", `{"\u0069\u0064":"ok"}`, S{ID: "ok"})
}

// TestDisableFieldNameUnescapingNoTag is same as TestDisableFieldNameUnescaping
// but with no struct tags involved.
func TestDisableFieldNameUnescapingNoTag(t *testing.T) {
	type S struct{ ID string }
	s := newTestSetup[S](t, jscandec.DecodeOptions{
		DisableFieldNameUnescaping: true,
	})

	s.TestOK(t, "no_unescaping", `{"ID":"ok"}`, S{ID: "ok"})
	s.TestOK(t, "no_unescaping_case_insensitive_match", `{"id":"ok"}`, S{ID: "ok"})
	s.TestOKPrepare(t, "escaped_treat_as_unknown", `{"\u0049\u0044":"not ok"}`, Test[S]{
		Check: func(t *testing.T, vJscan S, vEncodingJson any) {
			require.Equal(t, S{}, vJscan) // Ignore unknown field
			// encoding/json doesn't have an option to disable unescaping of field names.
			require.Equal(t, &S{ID: "not ok"}, vEncodingJson)
		},
	})
	s.TestOKPrepare(t, "escaped_case_insensitive", `{"\u0069\u0064":"not ok"}`, Test[S]{
		Check: func(t *testing.T, vJscan S, vEncodingJson any) {
			require.Equal(t, S{}, vJscan) // Ignore unknown field
			// encoding/json doesn't have an option to disable unescaping of field names.
			require.Equal(t, &S{ID: "not ok"}, vEncodingJson)
		},
	})
}

func TestDisableCaseInsensitiveMatching(t *testing.T) {
	type S struct {
		ID string `json:"id"`
	}
	s := newTestSetup[S](t, jscandec.DecodeOptions{
		DisableCaseInsensitiveMatching: true,
	})

	s.TestOK(t, "exact_match", `{"id":"ok"}`, S{ID: "ok"})
	s.TestOKPrepare(t, "no_match_treat_as_unknown", `{"ID":"not ok"}`, Test[S]{
		Check: func(t *testing.T, vJscan S, vEncodingJson any) {
			require.Equal(t, S{}, vJscan) // Ignore unknown field
			// encoding/json doesn't have an option to disable case-insensitive matching.
			require.Equal(t, &S{ID: "not ok"}, vEncodingJson)
		},
	})
	s.TestOKPrepare(t, "no_match_mixed", `{"Id":"not ok"}`, Test[S]{
		Check: func(t *testing.T, vJscan S, vEncodingJson any) {
			require.Equal(t, S{}, vJscan) // Ignore unknown field
			// encoding/json doesn't have an option to disable case-insensitive matching.
			require.Equal(t, &S{ID: "not ok"}, vEncodingJson)
		},
	})
}

// TestDisableCaseInsensitiveMatchingNoTag is same as TestDisableCaseInsensitiveMatching
// but with no struct tags involved.
func TestDisableCaseInsensitiveMatchingNoTag(t *testing.T) {
	type S struct{ ID string }
	s := newTestSetup[S](t, jscandec.DecodeOptions{
		DisableCaseInsensitiveMatching: true,
	})

	s.TestOK(t, "exact_match", `{"ID":"ok"}`, S{ID: "ok"})
	s.TestOKPrepare(t, "no_match_treat_as_unknown", `{"id":"not ok"}`, Test[S]{
		Check: func(t *testing.T, vJscan S, vEncodingJson any) {
			require.Equal(t, S{}, vJscan) // Ignore unknown field
			// encoding/json doesn't have an option to disable case-insensitive matching.
			require.Equal(t, &S{ID: "not ok"}, vEncodingJson)
		},
	})
	s.TestOKPrepare(t, "no_match_mixed", `{"Id":"not ok"}`, Test[S]{
		Check: func(t *testing.T, vJscan S, vEncodingJson any) {
			require.Equal(t, S{}, vJscan) // Ignore unknown field
			// encoding/json doesn't have an option to disable case-insensitive matching.
			require.Equal(t, &S{ID: "not ok"}, vEncodingJson)
		},
	})
}
