package jscandec_test

import (
	_ "embed"
	"encoding/json"
	"math"
	"runtime"
	"strconv"
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
	Decode(S, *T) jscandec.ErrorDecode
}

type testSetup[T any] struct{ decoder DecoderIface[string, T] }

func newTestSetup[T any]() testSetup[T] {
	tokenizer := jscan.NewTokenizer[string](64, 2048*1024)
	d := jscandec.NewDecoder[string, T](tokenizer)
	return testSetup[T]{decoder: d}
}

// testOK makes sure that input can be decoded to T successfully,
// equals expect (if any) and yields the same results as encoding/json.Unmarshal.
// expect is optional.
func (s testSetup[T]) testOK(t *testing.T, name, input string, expect ...T) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Helper()
		if len(expect) > 1 {
			t.Fatalf("more than one (%d) expectation", len(expect))
		}
		var std, actual T
		require.NoError(t, json.Unmarshal([]byte(input), &std))
		err := s.decoder.Decode(input, &actual)
		if err.IsErr() {
			t.Fatal(err.Error())
		}
		require.Equal(t, std, actual, "deviation between encoding/json and jscan")
		if len(expect) > 0 {
			require.Equal(t, expect[0], actual)
		}
		runtime.GC() // Make sure GC is happy
	})
}

// testErr makes sure that input fails to parse for both jscan and encoding/json
// and that the returned error equals expect.
func (s testSetup[T]) testErr(
	t *testing.T, name, input string, expect jscandec.ErrorDecode,
) {
	s.testErrCheck(t, name, input, func(t *testing.T, err jscandec.ErrorDecode) {
		t.Helper()
		require.Equal(t, expect.Err, err.Err)
		require.Equal(t, expect.Index, err.Index)
	})
}

// testErrCheck makes sure that input fails to parse for both jscan and encoding/json
// and calls check with the error returned.
func (s testSetup[T]) testErrCheck(
	t *testing.T, name, input string,
	check func(t *testing.T, err jscandec.ErrorDecode),
) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Helper()
		var std, v T
		require.Error(t, json.Unmarshal([]byte(input), &std), "no error in encoding/json")
		err := s.decoder.Decode(input, &v)
		check(t, err)
		runtime.GC() // Make sure GC is happy
	})
}

func TestDecodeNil(t *testing.T) {
	tokenizer := jscan.NewTokenizer[string](64, 1024)
	d := jscandec.NewDecoder[string, [][]bool](tokenizer)
	err := d.Decode(`"foo"`, nil)
	require.True(t, err.IsErr())
	require.Equal(t, jscandec.ErrNilDest, err.Err)
}

func TestDecodeBool(t *testing.T) {
	s := newTestSetup[bool]()
	s.testOK(t, "true", `true`, true)
	s.testOK(t, "false", `false`, false)

	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeAny(t *testing.T) {
	s := newTestSetup[any]()
	s.testOK(t, "int_0", `0`, float64(0))
	s.testOK(t, "int_42", `42`, float64(42))
	s.testOK(t, "number", `3.1415`, float64(3.1415))
	s.testOK(t, "true", `true`, true)
	s.testOK(t, "false", `false`, false)
	s.testOK(t, "string", `"string"`, "string")
	s.testOK(t, "null", `null`, nil)
	s.testOK(t, "array_empty", `[]`, []any{})
	s.testOK(t, "array_int", `[0,1,2]`, []any{float64(0), float64(1), float64(2)})
	s.testOK(t, "array_string", `["a", "b"]`, []any{"a", "b"})
	s.testOK(t, "array_bool", `[true, false]`, []any{true, false})
	s.testOK(t, "array_null", `[null, null, null]`, []any{nil, nil, nil})
	s.testOK(t, "object_empty", `{}`, map[string]any{})
	s.testOK(t, "array_mix", `[null, false, 42, "x", {}, true]`,
		[]any{nil, false, float64(42), "x", map[string]any{}, true})
	s.testOK(t, "object_multi", `{
		"num":         42,
		"str":         "text",
		"bool_true":   true,
		"bool_false":  false,
		"array_empty": [],
		"array_mix":   [null, false, 42, "x", {}, true],
		"null":        null
	}`, map[string]any{
		"num":         float64(42),
		"str":         "text",
		"bool_true":   true,
		"bool_false":  false,
		"array_empty": []any{},
		"array_mix":   []any{nil, false, float64(42), "x", map[string]any{}, true},
		"null":        nil,
	})
}

func TestDecodeUint(t *testing.T) {
	require64bitSystem(t)
	s := newTestSetup[uint]()
	s.testOK(t, "0", `0`, 0)
	s.testOK(t, "1", `1`, 1)
	s.testOK(t, "int32_max", `2147483647`, math.MaxInt32)
	s.testOK(t, "int64_max", `18446744073709551615`, math.MaxUint64)

	s.testErr(t, "overflow_hi", `18446744073709551616`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_l21", `111111111111111111111`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})

	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "-1", `-1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	// s.testErr(t, "null", `null`,
	// 	jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	// s.testErr(t, "float", `0.1`,
	//	jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	// s.testErr(t, "exponent", `1e2`,
	//	jscandec.ErrorDecode{Err: ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeInt(t *testing.T) {
	require64bitSystem(t)
	s := newTestSetup[int]()
	s.testOK(t, "0", `0`, 0)
	s.testOK(t, "1", `1`, 1)
	s.testOK(t, "-1", `-1`, -1)
	s.testOK(t, "int32_min", `-2147483648`, math.MinInt32)
	s.testOK(t, "int32_max", `2147483647`, math.MaxInt32)
	s.testOK(t, "int64_min", `-9223372036854775808`, math.MinInt64)
	s.testOK(t, "int64_max", `9223372036854775807`, math.MaxInt64)

	s.testErr(t, "overflow_hi", `9223372036854775808`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_lo", `-9223372036854775809`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})

	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	// s.testErr(t, "null", `null`,
	// 	jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	// s.testErr(t, "float", `0.1`,
	//	jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	// s.testErr(t, "exponent", `1e2`,
	//	jscandec.ErrorDecode{Err: ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeFloat32(t *testing.T) {
	s := newTestSetup[float32]()
	s.testOK(t, "0", `0`, 0)
	s.testOK(t, "1", `1`, 1)
	s.testOK(t, "-1", `-1`, -1)
	s.testOK(t, "min_int", `-16777215`, -16_777_215)
	s.testOK(t, "max_int", `16777215`, 16_777_215)
	s.testOK(t, "pi7", `3.1415927`, 3.1415927)
	s.testOK(t, "-pi7", `-3.1415927`, -3.1415927)
	s.testOK(t, "3.4028235e38", `3.4028235e38`, 3.4028235e38)
	s.testOK(t, "min_pos", `1.4e-45`, 1.4e-45)
	s.testOK(t, "3.4e38", `3.4e38`, 3.4e38)
	s.testOK(t, "-3.4e38", `-3.4e38`, -3.4e38)
	s.testOK(t, "avogadros_num", `6.022e23`, 6.022e23)

	s.testErrCheck(t, "range_hi", `3.5e38`,
		func(t *testing.T, err jscandec.ErrorDecode) {
			require.ErrorIs(t, err.Err, strconv.ErrRange)
			require.Equal(t, 0, err.Index)
		})

	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeFloat64(t *testing.T) {
	s := newTestSetup[float64]()
	s.testOK(t, "0", `0`, 0)
	s.testOK(t, "1", `1`, 1)
	s.testOK(t, "-1", `-1`, -1)
	s.testOK(t, "1.0", `1.0`, 1)
	s.testOK(t, "1.000000003", `1.000000003`, 1.000000003)
	s.testOK(t, "max_int", `9007199254740991`, 9_007_199_254_740_991)
	s.testOK(t, "min_int", `-9007199254740991`, -9_007_199_254_740_991)
	s.testOK(t, "pi",
		`3.141592653589793238462643383279502884197`,
		3.141592653589793238462643383279502884197)
	s.testOK(t, "pi_neg",
		`-3.141592653589793238462643383279502884197`,
		-3.141592653589793238462643383279502884197)
	s.testOK(t, "3.4028235e38", `3.4028235e38`, 3.4028235e38)
	s.testOK(t, "exponent", `1.7976931348623157e308`, 1.7976931348623157e308)
	s.testOK(t, "neg_exponent", `1.7976931348623157e-308`, 1.7976931348623157e-308)
	s.testOK(t, "1.4e-45", `1.4e-45`, 1.4e-45)
	s.testOK(t, "neg_exponent", `-1.7976931348623157e308`, -1.7976931348623157e308)
	s.testOK(t, "3.4e38", `3.4e38`, 3.4e38)
	s.testOK(t, "-3.4e38", `-3.4e38`, -3.4e38)
	s.testOK(t, "avogadros_num", `6.022e23`, 6.022e23)

	s.testErrCheck(t, "range_hi", `1e309`,
		func(t *testing.T, err jscandec.ErrorDecode) {
			require.ErrorIs(t, err.Err, strconv.ErrRange)
			require.Equal(t, 0, err.Index)
		})

	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeUint64(t *testing.T) {
	s := newTestSetup[uint64]()
	s.testOK(t, "0", `0`, 0)
	s.testOK(t, "1", `1`, 1)
	s.testOK(t, "int32_max", `2147483647`, math.MaxInt32)
	s.testOK(t, "uint32_max", `4294967295`, math.MaxUint32)
	s.testOK(t, "max", `18446744073709551615`, math.MaxUint64)

	s.testErr(t, "overflow_hi", `18446744073709551616`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_hi2", `19000000000000000000`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_l21", `111111111111111111111`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})

	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeInt64(t *testing.T) {
	s := newTestSetup[int64]()
	s.testOK(t, "0", `0`, 0)
	s.testOK(t, "1", `1`, 1)
	s.testOK(t, "-1", `-1`, -1)
	s.testOK(t, "int32_min", `-2147483648`, math.MinInt32)
	s.testOK(t, "int32_max", `2147483647`, math.MaxInt32)
	s.testOK(t, "min", `-9223372036854775808`, math.MinInt64)
	s.testOK(t, "max", `9223372036854775807`, math.MaxInt64)

	s.testErr(t, "overflow_hi", `9223372036854775808`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_lo", `-9223372036854775809`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})

	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeUint32(t *testing.T) {
	s := newTestSetup[uint32]()
	s.testOK(t, "0", `0`, 0)
	s.testOK(t, "1", `1`, 1)
	s.testOK(t, "max", `4294967295`, math.MaxUint32)

	s.testErr(t, "overflow_hi", `4294967296`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_hi2", `5000000000`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_l11", `11111111111`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})

	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeInt32(t *testing.T) {
	s := newTestSetup[int32]()
	s.testOK(t, "0", `0`, 0)
	s.testOK(t, "1", `1`, 1)
	s.testOK(t, "-1", `-1`, -1)
	s.testOK(t, "min", `-2147483648`, math.MinInt32)
	s.testOK(t, "max", `2147483647`, math.MaxInt32)

	s.testErr(t, "overflow_hi", `2147483648`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_lo", `-2147483649`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})

	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeUint16(t *testing.T) {
	s := newTestSetup[uint16]()
	s.testOK(t, "0", `0`, 0)
	s.testOK(t, "1", `1`, 1)
	s.testOK(t, "max", `65535`, math.MaxUint16)

	s.testErr(t, "overflow_hi", `65536`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_hi2", `70000`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_l6", `111111`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})

	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeInt16(t *testing.T) {
	s := newTestSetup[int16]()
	s.testOK(t, "0", `0`, 0)
	s.testOK(t, "1", `1`, 1)
	s.testOK(t, "-1", `-1`, -1)
	s.testOK(t, "min", `-32768`, math.MinInt16)
	s.testOK(t, "max", `32767`, math.MaxInt16)

	s.testErr(t, "overflow_hi", `32768`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_lo", `-32769`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})

	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeUint8(t *testing.T) {
	s := newTestSetup[uint8]()
	s.testOK(t, "0", `0`, 0)
	s.testOK(t, "1", `1`, 1)
	s.testOK(t, "max", `255`, math.MaxUint8)

	s.testErr(t, "overflow_hi", `256`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_hi2", `300`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_l4", `1111`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})

	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeInt8(t *testing.T) {
	s := newTestSetup[int8]()
	s.testOK(t, "0", `0`, 0)
	s.testOK(t, "1", `1`, 1)
	s.testOK(t, "-1", `-1`, -1)
	s.testOK(t, "min", `-128`, math.MinInt8)
	s.testOK(t, "max", `127`, math.MaxInt8)

	s.testErr(t, "overflow_hi", `128`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_lo", `-129`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})

	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeNull(t *testing.T) {
	// Primitive types
	newTestSetup[string]().testOK(t, "string", `null`, "")
	newTestSetup[bool]().testOK(t, "bool", `null`, false)
	newTestSetup[int]().testOK(t, "int", `null`, 0)
	newTestSetup[int8]().testOK(t, "int8", `null`, 0)
	newTestSetup[int16]().testOK(t, "int16", `null`, 0)
	newTestSetup[int32]().testOK(t, "int32", `null`, 0)
	newTestSetup[int64]().testOK(t, "int64", `null`, 0)
	newTestSetup[uint]().testOK(t, "uint", `null`, 0)
	newTestSetup[uint8]().testOK(t, "uint8", `null`, 0)
	newTestSetup[uint16]().testOK(t, "uint16", `null`, 0)
	newTestSetup[uint32]().testOK(t, "uint32", `null`, 0)
	newTestSetup[uint64]().testOK(t, "uint64", `null`, 0)
	newTestSetup[float32]().testOK(t, "float32", `null`, 0)
	newTestSetup[float64]().testOK(t, "float64", `null`, 0)

	// Slices
	newTestSetup[[]bool]().testOK(t, "slice_bool", `null`, nil)
	newTestSetup[[]string]().testOK(t, "slice_string", `null`, nil)

	// Primitives in array
	newTestSetup[[]bool]().testOK(t, "array_bool", `[null]`, []bool{false})
	newTestSetup[[]string]().testOK(t, "array_string", `[null]`, []string{""})
	newTestSetup[[]int]().testOK(t, "array_int", `[null]`, []int{0})
	newTestSetup[[]int8]().testOK(t, "array_int8", `[null]`, []int8{0})
	newTestSetup[[]int16]().testOK(t, "array_int16", `[null]`, []int16{0})
	newTestSetup[[]int32]().testOK(t, "array_int32", `[null]`, []int32{0})
	newTestSetup[[]int64]().testOK(t, "array_int64", `[null]`, []int64{0})
	newTestSetup[[]uint]().testOK(t, "array_int", `[null]`, []uint{0})
	newTestSetup[[]uint8]().testOK(t, "array_int8", `[null]`, []uint8{0})
	newTestSetup[[]uint16]().testOK(t, "array_int16", `[null]`, []uint16{0})
	newTestSetup[[]uint32]().testOK(t, "array_int32", `[null]`, []uint32{0})
	newTestSetup[[]uint64]().testOK(t, "array_int64", `[null]`, []uint64{0})
	newTestSetup[[]float32]().testOK(t, "array_float32", `[null]`, []float32{0})
	newTestSetup[[]float64]().testOK(t, "array_float64", `[null]`, []float64{0})
}

func TestDecodeString(t *testing.T) {
	s := newTestSetup[string]()
	s.testOK(t, "empty", `""`, "")
	s.testOK(t, "spaces", `"   "`, "   ")
	s.testOK(t, "hello_world", `"Hello World!"`, "Hello World!")
	s.testOK(t, "unicede", `"юникодж"`, "юникодж")
}

func TestDecode2DSliceBool(t *testing.T) {
	type T = [][]bool
	s := newTestSetup[T]()
	s.testOK(t, "3_2", `[[true],[false, true],[ ]]`, T{{true}, {false, true}, {}})
	s.testOK(t, "2_1", `[[],[false]]`, T{{}, {false}})
	s.testOK(t, "2_0", `[[],[]]`, T{{}, {}})
	s.testOK(t, "1", `[]`, T{})
	s.testOK(t, "array_2d_bool_6m", array2DBool6M)
}

func TestDecodeSliceString(t *testing.T) {
	type T = []string
	s := newTestSetup[T]()
	s.testOK(t, "3", `[ "a", "ab", "cde" ]`, T{"a", "ab", "cde"})
	s.testOK(t, "2", `[ "abc" ]`, T{"abc"})
	s.testOK(t, "0", `[]`, T{})
	s.testOK(t, "array_str_1024_639k", arrayStr1024)
}

func TestDecodeSliceFloat32(t *testing.T) {
	type T = []float32
	s := newTestSetup[T]()
	s.testOK(t, "3", `[ 1, 1.1, 3.1415e5 ]`, T{1, 1.1, 3.1415e5})
	s.testOK(t, "2", `[ 0 ]`, T{0})
	s.testOK(t, "0", `[]`, T{})
	s.testOK(t, "array_dec_1024_10k", arrayFloat1024)
}

func TestDecodeSliceFloat64(t *testing.T) {
	type T = []float64
	s := newTestSetup[T]()
	s.testOK(t, "3", `[ 1, 1.1, 3.1415e5 ]`, T{1, 1.1, 3.1415e5})
	s.testOK(t, "2", `[ 0 ]`, T{0})
	s.testOK(t, "2", `[ 9007199254740991 ]`, T{9_007_199_254_740_991})
	s.testOK(t, "0", `[]`, T{})
	s.testOK(t, "array_dec_1024_10k", arrayFloat1024)
}

func TestDecode2DSliceInt(t *testing.T) {
	type T = [][]int
	s := newTestSetup[T]()
	s.testOK(t, "3_2", `[[0],[12, 123],[ ]]`, T{{0}, {12, 123}, {}})
	s.testOK(t, "2_1", `[[],[-12345678]]`, T{{}, {-12_345_678}})
	s.testOK(t, "2_0", `[[],[]]`, T{{}, {}})
	s.testOK(t, "1", `[]`, T{})
}

func TestDecodeMatrix2Int(t *testing.T) {
	type T = [2][2]int
	s := newTestSetup[T]()
	s.testOK(t, "complete",
		`[[0,1],[2,3]]`,
		T{{0, 1}, {2, 3}})
	s.testOK(t, "empty",
		`[]`,
		T{{0, 0}, {0, 0}})
	s.testOK(t, "sub_arrays_empty",
		`[[],[]]`,
		T{{0, 0}, {0, 0}})
	s.testOK(t, "incomplete",
		`[[1],[2]]`,
		T{{1, 0}, {2, 0}})
	s.testOK(t, "partially_incomplete",
		`[[1,2],[3]]`,
		T{{1, 2}, {3, 0}})
}

func TestDecodeMatrix4Int(t *testing.T) {
	type T = [4][4]int
	s := newTestSetup[T]()
	s.testOK(t, "complete",
		`[[0,1,2,3],[4,5,6,7],[8,9,10,11],[12,13,14,15]]`,
		T{{0, 1, 2, 3}, {4, 5, 6, 7}, {8, 9, 10, 11}, {12, 13, 14, 15}})
	s.testOK(t, "incomplete",
		`[[1],[2,3],[4,5,6],[]]`,
		T{{1, 0, 0, 0}, {2, 3, 0, 0}, {4, 5, 6, 0}, {0, 0, 0, 0}})
	s.testOK(t, "empty",
		`[]`,
		T{{0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}})
	s.testOK(t, "sub_arrays_empty_incomplete",
		`[[],[]]`,
		T{{0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}, {0, 0, 0, 0}})
}

func TestDecodeStruct(t *testing.T) {
	type S struct {
		Foo int    `json:"foo"`
		Bar string `json:"bar"`
	}
	s := newTestSetup[S]()
	s.testOK(t, "regular_field_order",
		`{"foo":42,"bar":"bazz"}`, S{Foo: 42, Bar: "bazz"})
	s.testOK(t, "reversed_field_order",
		`{"bar":"abc","foo":1234}`, S{Foo: 1234, Bar: "abc"})
	s.testOK(t, "case_insensitive_match1",
		`{"FOO":42,"BAR":"bazz"}`, S{Foo: 42, Bar: "bazz"})
	s.testOK(t, "case_insensitive_match2",
		`{"Foo":42,"Bar":"bazz"}`, S{Foo: 42, Bar: "bazz"})
	s.testOK(t, "null_fields", `{"foo":null,"bar":null}`, S{Foo: 0, Bar: ""})

	s.testOK(t, "missing_field_foo",
		`{"bar":"bar"}`, S{Bar: "bar"})
	s.testOK(t, "missing_field_bar",
		`{"foo":12345}`, S{Foo: 12345})
	s.testOK(t, "unknown_field",
		`{"bar":"bar","unknown":42,"foo":102}`, S{Foo: 102, Bar: "bar"})
	s.testOK(t, "unknown_fields_only",
		`{"unknown":42, "unknown2": "bad"}`, S{})

	s.testOK(t, "empty", `{}`, S{})
	s.testOK(t, "name_mismatch", `{"faz":42,"baz":"bazz"}`, S{})

	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeStructSlice(t *testing.T) {
	type S struct {
		Foo int    `json:"foo"`
		Bar string `json:"bar"`
	}
	s := newTestSetup[[]S]()
	s.testOK(t, "empty_array",
		`[]`, []S{})
	s.testOK(t, "regular_field_order",
		`[{"foo":42,"bar":"bazz"}]`, []S{{Foo: 42, Bar: "bazz"}})
	s.testOK(t, "multiple",
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
	s.testOK(t, "empty_null_and_unknown_fields",
		`[
			{ },
			null,
			{"faz":42,"baz":"bazz"}
		]`, []S{{}, {}, {}})
	s.testOK(t, "mixed",
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
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeMapStringToString(t *testing.T) {
	type M map[string]string
	s := newTestSetup[M]()
	s.testOK(t, "empty", `{}`, M{})
	s.testOK(t, "2_pairs",
		`{"foo":"42","bar":"bazz"}`, M{"foo": "42", "bar": "bazz"})
	s.testOK(t, "empty_strings",
		`{"":""}`, M{"": ""})
	s.testOK(t, "multiple_empty_strings",
		`{"":"", "":""}`, M{"": ""})
	s.testOK(t, "null_value",
		`{"":null}`, M{"": ""})
	s.testOK(t, "duplicate_values",
		`{"a":"1","a":"2"}`, M{"a": "2"}) // Take last
	s.testOK(t, "mulitple_overrides",
		`{"":"1", "":"12", "":"123"}`, M{"": "123"}) // Take last
	s.testOK(t, "many",
		`{
			"foo": "1", "bar": "a", "baz": "2", "muzz": "",
			"longer_key": "longer test text"
		}`, M{
			"foo": "1", "bar": "a", "baz": "2", "muzz": "",
			"longer_key": "longer test text",
		})

	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeMapStringToMapStringToString(t *testing.T) {
	type M2 map[string]string
	type M map[string]M2
	s := newTestSetup[M]()
	s.testOK(t, "empty", `{}`, M{})
	s.testOK(t, "2_pairs",
		`{
			"a":{"a1":"a1val","a2":"a2val"},
			"b":{"b1":"b1val","b2":"b2val"}
		}`,
		M{
			"a": M2{"a1": "a1val", "a2": "a2val"},
			"b": M2{"b1": "b1val", "b2": "b2val"},
		})
	s.testOK(t, "empty_strings",
		`{"":{"":""}}`, M{"": M2{"": ""}})
	s.testOK(t, "multiple_empty_strings",
		`{"":{"":"", "":""}, "":{"":"", "":""}}`,
		M{"": M2{"": ""}})
	s.testOK(t, "null_value",
		`{"n":null,"x":{"x":null}}`, M{"n": nil, "x": M2{"x": ""}})
	s.testOK(t, "duplicate_values",
		`{"a":{"foo":"bar"},"a":{"baz":"faz"}}`,
		M{"a": M2{"baz": "faz"}}) // Take last
	s.testOK(t, "mulitple_overrides",
		`{"":{"a":"b"}, "":{"c":"d"}, "":{"e":"f"}}`,
		M{"": {"e": "f"}}) // Take last
	s.testOK(t, "mixed",
		`{
			"":{},
			"first_key":{"f1":"first1_value","f2":"first2_value"},
			"second_key":null
		}`, M{
			"":           M2{},
			"first_key":  M2{"f1": "first1_value", "f2": "first2_value"},
			"second_key": nil,
		})

	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "map_string_string", `{"foo":"bar"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 7})
	s.testErr(t, "map_string_map_string_int", `{"foo":{"bar":42}}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 14})
}

func TestDecodeMapStringToStruct(t *testing.T) {
	type S struct {
		Name string `json:"name"`
		ID   int    `json:"id"`
	}
	type M map[string]S
	s := newTestSetup[M]()
	s.testOK(t, "empty", `{}`, M{})
	s.testOK(t, "one",
		`{"x":{"name":"first","id":1}}`, M{"x": S{Name: "first", ID: 1}})
	s.testOK(t, "empty_struct",
		`{"x":{}}`, M{"x": {}})
	s.testOK(t, "null_value",
		`{"":null}`, M{"": {}})
	s.testOK(t, "multiple",
		`{
			"x":{"name":"first","id":1},
			"y":{"name":"second","id":2}
		}`, M{
			"x": S{Name: "first", ID: 1},
			"y": S{Name: "second", ID: 2},
		})

	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "non_object_element", `{"x":42}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 5})
}

func BenchmarkSmall(b *testing.B) {
	in := []byte(`[[true],[false,false,false,false],[],[],[true]]`) // 18 tokens

	b.Run("jscan", func(b *testing.B) {
		tok := jscan.NewTokenizer[[]byte](8, 64)
		d := jscandec.NewDecoder[[]byte, [][]bool](tok)
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			var v [][]bool
			if err := d.Decode(in, &v); err.IsErr() {
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

func require64bitSystem(t *testing.T) {
	require.Equal(t, uintptr(8), unsafe.Sizeof(int(0)),
		"this test must run on a 64-bit system")
}
