package jscandec_test

import (
	_ "embed"
	"encoding/json"
	"errors"
	"math"
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
	Decode(S, *T, *jscandec.DecodeOptions) jscandec.ErrorDecode
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
	dStr, err := jscandec.NewDecoder[string, T](tokenizerString)
	require.NoError(t, err)
	tokenizerBytes := jscan.NewTokenizer[[]byte](16, 1024)
	dBytes, err := jscandec.NewDecoder[[]byte, T](tokenizerBytes)
	require.NoError(t, err)
	return testSetup[T]{
		decodeOptions: &decodeOptions,
		decoderString: dStr,
		decoderBytes:  dBytes,
	}
}

// testOK makes sure that input can be decoded to T successfully,
// equals expect (if any) and yields the same results as encoding/json.Unmarshal.
// expect is optional.
func (s testSetup[T]) testOK(t *testing.T, name, input string, expect ...T) {
	t.Helper()
	if len(expect) > 1 {
		t.Fatalf("more than one (%d) expectation", len(expect))
	}
	check := func(t *testing.T, stdResult, jscanResult T) {
		t.Helper()
		require.Equal(t, stdResult, jscanResult,
			"deviation between encoding/json and jscan results")
		if len(expect) > 0 {
			require.Equal(t, expect[0], jscanResult)
		}
		runtime.GC() // Make sure GC is happy
	}
	t.Run(name+"/bytes", func(t *testing.T) {
		t.Helper()
		var std, actual T
		stdDec := json.NewDecoder(strings.NewReader(input))
		if s.decodeOptions.DisallowUnknownFields {
			stdDec.DisallowUnknownFields()
		}
		require.NoError(t, stdDec.Decode(&std), "error in encoding/json")
		err := s.decoderBytes.Decode([]byte(input), &actual, s.decodeOptions)
		if err.IsErr() {
			t.Fatal(err.Error())
		}
		check(t, std, actual)
	})
	t.Run(name+"/string", func(t *testing.T) {
		t.Helper()
		var std, actual T
		stdDec := json.NewDecoder(strings.NewReader(input))
		if s.decodeOptions.DisallowUnknownFields {
			stdDec.DisallowUnknownFields()
		}
		require.NoError(t, stdDec.Decode(&std), "error in encoding/json")
		err := s.decoderString.Decode(input, &actual, s.decodeOptions)
		if err.IsErr() {
			t.Fatal(err.Error())
		}
		check(t, std, actual)
	})
}

// testErr makes sure that input fails to parse for both jscan and encoding/json
// and that the returned error equals expect.
func (s testSetup[T]) testErr(
	t *testing.T, name, input string, expect jscandec.ErrorDecode,
) {
	t.Helper()
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
	t.Run(name+"/bytes", func(t *testing.T) {
		t.Helper()
		var std, v T
		stdDec := json.NewDecoder(strings.NewReader(input))
		if s.decodeOptions.DisallowUnknownFields {
			stdDec.DisallowUnknownFields()
		}
		require.Error(t, stdDec.Decode(&std), "no error in encoding/json")
		err := s.decoderBytes.Decode([]byte(input), &v, s.decodeOptions)
		check(t, err)
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
		err := s.decoderString.Decode(input, &v, s.decodeOptions)
		check(t, err)
		runtime.GC() // Make sure GC is happy
	})
}

func TestDecodeNil(t *testing.T) {
	tokenizer := jscan.NewTokenizer[string](64, 1024)
	d, err := jscandec.NewDecoder[string, [][]bool](tokenizer)
	require.NoError(t, err)
	errDec := d.Decode(`"foo"`, nil, jscandec.DefaultOptions)
	require.True(t, errDec.IsErr())
	require.Equal(t, jscandec.ErrNilDest, errDec.Err)
}

func TestDecodeBool(t *testing.T) {
	s := newTestSetup[bool](t, *jscandec.DefaultOptions)
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
	s := newTestSetup[any](t, *jscandec.DefaultOptions)
	s.testOK(t, "int_0", `0`, float64(0))
	s.testOK(t, "int_42", `42`, float64(42))
	s.testOK(t, "number", `3.1415`, float64(3.1415))
	s.testOK(t, "true", `true`, true)
	s.testOK(t, "false", `false`, false)
	s.testOK(t, "string", `"string"`, "string")
	s.testOK(t, "string_escaped", `"\"\u30C4\""`, `"ツ"`)
	s.testOK(t, "null", `null`, nil)
	s.testOK(t, "array_empty", `[]`, []any{})
	s.testOK(t, "array_int", `[0,1,2]`, []any{float64(0), float64(1), float64(2)})
	s.testOK(t, "array_string", `["a", "b", "\t"]`, []any{"a", "b", "\t"})
	s.testOK(t, "array_bool", `[true, false]`, []any{true, false})
	s.testOK(t, "array_null", `[null, null, null]`, []any{nil, nil, nil})
	s.testOK(t, "object_empty", `{}`, map[string]any{})
	s.testOK(t, "array_mix", `[null, false, 42, "x", {}, true]`,
		[]any{nil, false, float64(42), "x", map[string]any{}, true})
	s.testOK(t, "object_multi", `{
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

	s.testErrCheck(t, "float_range_hi", `1e309`,
		func(t *testing.T, err jscandec.ErrorDecode) {
			require.ErrorIs(t, err.Err, strconv.ErrRange)
			require.Equal(t, 0, err.Index)
		})
}

func TestDecodeUint(t *testing.T) {
	skipIfNot64bitSystem(t)
	s := newTestSetup[uint](t, *jscandec.DefaultOptions)
	s.testOK(t, "0", `0`, 0)
	s.testOK(t, "1", `1`, 1)
	s.testOK(t, "int32_max", `2147483647`, math.MaxInt32)
	s.testOK(t, "int64_max", `18446744073709551615`, math.MaxUint64)

	s.testErr(t, "overflow_hi", `18446744073709551616`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_l21", `111111111111111111111`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})

	s.testErr(t, "negative", `-1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "-1", `-1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "float", `0.1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "exponent", `1e2`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeInt(t *testing.T) {
	skipIfNot64bitSystem(t)
	s := newTestSetup[int](t, *jscandec.DefaultOptions)
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
	s := newTestSetup[float32](t, *jscandec.DefaultOptions)
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
	s := newTestSetup[float64](t, *jscandec.DefaultOptions)
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
	s := newTestSetup[uint64](t, *jscandec.DefaultOptions)
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

	s.testErr(t, "negative", `-1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
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
	s := newTestSetup[int64](t, *jscandec.DefaultOptions)
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
	s := newTestSetup[uint32](t, *jscandec.DefaultOptions)
	s.testOK(t, "0", `0`, 0)
	s.testOK(t, "1", `1`, 1)
	s.testOK(t, "max", `4294967295`, math.MaxUint32)

	s.testErr(t, "overflow_hi", `4294967296`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_hi2", `5000000000`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_l11", `11111111111`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})

	s.testErr(t, "negative", `-1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
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
	s := newTestSetup[int32](t, *jscandec.DefaultOptions)
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
	s := newTestSetup[uint16](t, *jscandec.DefaultOptions)
	s.testOK(t, "0", `0`, 0)
	s.testOK(t, "1", `1`, 1)
	s.testOK(t, "max", `65535`, math.MaxUint16)

	s.testErr(t, "overflow_hi", `65536`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_hi2", `70000`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_l6", `111111`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})

	s.testErr(t, "negative", `-1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
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
	s := newTestSetup[int16](t, *jscandec.DefaultOptions)
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
	s := newTestSetup[uint8](t, *jscandec.DefaultOptions)
	s.testOK(t, "0", `0`, 0)
	s.testOK(t, "1", `1`, 1)
	s.testOK(t, "max", `255`, math.MaxUint8)

	s.testErr(t, "overflow_hi", `256`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_hi2", `300`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})
	s.testErr(t, "overflow_l4", `1111`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 0})

	s.testErr(t, "negative", `-1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
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
	s := newTestSetup[int8](t, *jscandec.DefaultOptions)
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
	newTestSetup[string](t, *jscandec.DefaultOptions).testOK(t, "string", `null`, "")
	newTestSetup[bool](t, *jscandec.DefaultOptions).testOK(t, "bool", `null`, false)
	newTestSetup[int](t, *jscandec.DefaultOptions).testOK(t, "int", `null`, 0)
	newTestSetup[int8](t, *jscandec.DefaultOptions).testOK(t, "int8", `null`, 0)
	newTestSetup[int16](t, *jscandec.DefaultOptions).testOK(t, "int16", `null`, 0)
	newTestSetup[int32](t, *jscandec.DefaultOptions).testOK(t, "int32", `null`, 0)
	newTestSetup[int64](t, *jscandec.DefaultOptions).testOK(t, "int64", `null`, 0)
	newTestSetup[uint](t, *jscandec.DefaultOptions).testOK(t, "uint", `null`, 0)
	newTestSetup[uint8](t, *jscandec.DefaultOptions).testOK(t, "uint8", `null`, 0)
	newTestSetup[uint16](t, *jscandec.DefaultOptions).testOK(t, "uint16", `null`, 0)
	newTestSetup[uint32](t, *jscandec.DefaultOptions).testOK(t, "uint32", `null`, 0)
	newTestSetup[uint64](t, *jscandec.DefaultOptions).testOK(t, "uint64", `null`, 0)
	newTestSetup[float32](t, *jscandec.DefaultOptions).testOK(t, "float32", `null`, 0)
	newTestSetup[float64](t, *jscandec.DefaultOptions).testOK(t, "float64", `null`, 0)

	// Slices
	newTestSetup[[]bool](t, *jscandec.DefaultOptions).
		testOK(t, "slice_bool", `null`, nil)
	newTestSetup[[]string](t, *jscandec.DefaultOptions).
		testOK(t, "slice_string", `null`, nil)

	// Primitives in array
	newTestSetup[[]bool](t, *jscandec.DefaultOptions).
		testOK(t, "array_bool", `[null]`, []bool{false})
	newTestSetup[[]string](t, *jscandec.DefaultOptions).
		testOK(t, "array_string", `[null]`, []string{""})
	newTestSetup[[]int](t, *jscandec.DefaultOptions).
		testOK(t, "array_int", `[null]`, []int{0})
	newTestSetup[[]int8](t, *jscandec.DefaultOptions).
		testOK(t, "array_int8", `[null]`, []int8{0})
	newTestSetup[[]int16](t, *jscandec.DefaultOptions).
		testOK(t, "array_int16", `[null]`, []int16{0})
	newTestSetup[[]int32](t, *jscandec.DefaultOptions).
		testOK(t, "array_int32", `[null]`, []int32{0})
	newTestSetup[[]int64](t, *jscandec.DefaultOptions).
		testOK(t, "array_int64", `[null]`, []int64{0})
	newTestSetup[[]uint](t, *jscandec.DefaultOptions).
		testOK(t, "array_int", `[null]`, []uint{0})
	newTestSetup[[]uint8](t, *jscandec.DefaultOptions).
		testOK(t, "array_int8", `[null]`, []uint8{0})
	newTestSetup[[]uint16](t, *jscandec.DefaultOptions).
		testOK(t, "array_int16", `[null]`, []uint16{0})
	newTestSetup[[]uint32](t, *jscandec.DefaultOptions).
		testOK(t, "array_int32", `[null]`, []uint32{0})
	newTestSetup[[]uint64](t, *jscandec.DefaultOptions).
		testOK(t, "array_int64", `[null]`, []uint64{0})
	newTestSetup[[]float32](t, *jscandec.DefaultOptions).
		testOK(t, "array_float32", `[null]`, []float32{0})
	newTestSetup[[]float64](t, *jscandec.DefaultOptions).
		testOK(t, "array_float64", `[null]`, []float64{0})
}

func TestDecodeString(t *testing.T) {
	s := newTestSetup[string](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `""`, "")
	s.testOK(t, "spaces", `"   "`, "   ")
	s.testOK(t, "hello_world", `"Hello World!"`, "Hello World!")
	s.testOK(t, "unicode", `"юникод-жеж"`, "юникод-жеж")
	s.testOK(t, "escaped", `"\"\\\""`, `"\"`)
	s.testOK(t, "escaped_unicode", `"\u0436\u0448\u0444\u30C4"`, `жшфツ`)
}

func TestDecode2DSliceBool(t *testing.T) {
	type T = [][]bool
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.testOK(t, "3_2", `[[true],[false, true],[ ]]`, T{{true}, {false, true}, {}})
	s.testOK(t, "2_1", `[[],[false]]`, T{{}, {false}})
	s.testOK(t, "2_0", `[[],[]]`, T{{}, {}})
	s.testOK(t, "1", `[]`, T{})
	s.testOK(t, "array_2d_bool_6m", array2DBool6M)
}

func TestDecodeSliceInt(t *testing.T) {
	skipIfNot64bitSystem(t)

	type T = []int
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.testOK(t, "three_items", `[ 1, -23, 456 ]`, T{1, -23, 456})
	s.testOK(t, "max_int", `[9223372036854775807]`, T{math.MaxInt})
	s.testOK(t, "min_int", `[-9223372036854775808]`, T{math.MinInt})
	s.testOK(t, "one_item", `[ 1 ]`, T{1})
	s.testOK(t, "empty", `[]`, T{})
	s.testOK(t, "null", `null`, T(nil))
	s.testOK(t, "null_element", `[ null ]`, T{0})
	s.testOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.testErr(t, "overflow_hi", `[9223372036854775808]`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 1})
	s.testErr(t, "overflow_lo", `[-9223372036854775809]`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 1})
	s.testErr(t, "wrong_type_object", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
}

func TestDecodeSliceInt8(t *testing.T) {
	type T = []int8
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.testOK(t, "three_items", `[ 1, -23, 123 ]`, T{1, -23, 123})
	s.testOK(t, "max_int8", `[127]`, T{math.MaxInt8})
	s.testOK(t, "min_int8", `[-128]`, T{math.MinInt8})
	s.testOK(t, "one_item", `[ 1 ]`, T{1})
	s.testOK(t, "empty", `[]`, T{})
	s.testOK(t, "null", `null`, T(nil))
	s.testOK(t, "null_element", `[ null ]`, T{0})
	s.testOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.testErr(t, "overflow_hi", `[128]`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 1})
	s.testErr(t, "overflow_lo", `[-129]`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 1})
	s.testErr(t, "wrong_type_object", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
}

func TestDecodeSliceInt16(t *testing.T) {
	type T = []int16
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.testOK(t, "three_items", `[ 1, -23, 123 ]`, T{1, -23, 123})
	s.testOK(t, "max_int16", `[32767]`, T{math.MaxInt16})
	s.testOK(t, "min_int16", `[-32768]`, T{math.MinInt16})
	s.testOK(t, "one_item", `[ 1 ]`, T{1})
	s.testOK(t, "empty", `[]`, T{})
	s.testOK(t, "null", `null`, T(nil))
	s.testOK(t, "null_element", `[ null ]`, T{0})
	s.testOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.testErr(t, "overflow_hi", `[32768]`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 1})
	s.testErr(t, "overflow_lo", `[-32769]`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 1})
	s.testErr(t, "wrong_type_object", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
}

func TestDecodeSliceInt32(t *testing.T) {
	type T = []int32
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.testOK(t, "three_items", `[ 1, -23, 123 ]`, T{1, -23, 123})
	s.testOK(t, "max_int32", `[2147483647]`, T{math.MaxInt32})
	s.testOK(t, "min_int32", `[-2147483648]`, T{math.MinInt32})
	s.testOK(t, "one_item", `[ 1 ]`, T{1})
	s.testOK(t, "empty", `[]`, T{})
	s.testOK(t, "null", `null`, T(nil))
	s.testOK(t, "null_element", `[ null ]`, T{0})
	s.testOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.testErr(t, "overflow_hi", `[2147483648]`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 1})
	s.testErr(t, "overflow_lo", `[-2147483649]`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 1})
	s.testErr(t, "wrong_type_object", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
}

func TestDecodeSliceInt64(t *testing.T) {
	type T = []int64
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.testOK(t, "three_items", `[ 1, -23, 123 ]`, T{1, -23, 123})
	s.testOK(t, "max_int64", `[9223372036854775807]`, T{math.MaxInt64})
	s.testOK(t, "min_int64", `[-9223372036854775808]`, T{math.MinInt64})
	s.testOK(t, "one_item", `[ 1 ]`, T{1})
	s.testOK(t, "empty", `[]`, T{})
	s.testOK(t, "null", `null`, T(nil))
	s.testOK(t, "null_element", `[ null ]`, T{0})
	s.testOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.testErr(t, "overflow_hi", `[9223372036854775808]`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 1})
	s.testErr(t, "overflow_lo", `[-9223372036854775809]`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 1})
	s.testErr(t, "wrong_type_object", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
}

func TestDecodeSliceUint(t *testing.T) {
	skipIfNot64bitSystem(t)

	type T = []uint
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.testOK(t, "three_items", `[ 1, 23, 456 ]`, T{1, 23, 456})
	s.testOK(t, "max_uint", `[18446744073709551615]`, T{math.MaxUint})
	s.testOK(t, "one_item", `[ 1 ]`, T{1})
	s.testOK(t, "empty", `[]`, T{})
	s.testOK(t, "null", `null`, T(nil))
	s.testOK(t, "null_element", `[ null ]`, T{0})
	s.testOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.testErr(t, "negative", `[-1]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "overflow_hi", `[18446744073709551616]`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 1})
	s.testErr(t, "wrong_type_object", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
}

func TestDecodeSliceUint8(t *testing.T) {
	skipIfNot64bitSystem(t)

	type T = []uint8
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.testOK(t, "three_items", `[ 1, 23, 123 ]`, T{1, 23, 123})
	s.testOK(t, "max_uint", `[255]`, T{math.MaxUint8})
	s.testOK(t, "one_item", `[ 1 ]`, T{1})
	s.testOK(t, "empty", `[]`, T{})
	s.testOK(t, "null", `null`, T(nil))
	s.testOK(t, "null_element", `[ null ]`, T{0})
	s.testOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.testErr(t, "negative", `[-1]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "overflow_hi", `[256]`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 1})
	s.testErr(t, "wrong_type_object", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
}

func TestDecodeSliceUint16(t *testing.T) {
	skipIfNot64bitSystem(t)

	type T = []uint16
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.testOK(t, "three_items", `[ 1, 23, 123 ]`, T{1, 23, 123})
	s.testOK(t, "max_uint", `[65535]`, T{math.MaxUint16})
	s.testOK(t, "one_item", `[ 1 ]`, T{1})
	s.testOK(t, "empty", `[]`, T{})
	s.testOK(t, "null", `null`, T(nil))
	s.testOK(t, "null_element", `[ null ]`, T{0})
	s.testOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.testErr(t, "negative", `[-1]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "overflow_hi", `[65536]`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 1})
	s.testErr(t, "wrong_type_object", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
}

func TestDecodeSliceUint32(t *testing.T) {
	skipIfNot64bitSystem(t)

	type T = []uint32
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.testOK(t, "three_items", `[ 1, 23, 123 ]`, T{1, 23, 123})
	s.testOK(t, "max_uint", `[4294967295]`, T{math.MaxUint32})
	s.testOK(t, "one_item", `[ 1 ]`, T{1})
	s.testOK(t, "empty", `[]`, T{})
	s.testOK(t, "null", `null`, T(nil))
	s.testOK(t, "null_element", `[ null ]`, T{0})
	s.testOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.testErr(t, "negative", `[-1]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "overflow_hi", `[4294967296]`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 1})
	s.testErr(t, "wrong_type_object", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
}

func TestDecodeSliceUint64(t *testing.T) {
	skipIfNot64bitSystem(t)

	type T = []uint64
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.testOK(t, "three_items", `[ 1, 23, 123 ]`, T{1, 23, 123})
	s.testOK(t, "max_uint", `[18446744073709551615]`, T{math.MaxUint64})
	s.testOK(t, "one_item", `[ 1 ]`, T{1})
	s.testOK(t, "empty", `[]`, T{})
	s.testOK(t, "null", `null`, T(nil))
	s.testOK(t, "null_element", `[ null ]`, T{0})
	s.testOK(t, "null_element_multi", `[ null, 1, null ]`, T{0, 1, 0})

	s.testErr(t, "negative", `[-1]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "overflow_hi", `[18446744073709551616]`,
		jscandec.ErrorDecode{Err: jscandec.ErrIntegerOverflow, Index: 1})
	s.testErr(t, "wrong_type_object", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_element_float", `[1,3.14]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
}

func TestDecodeSliceString(t *testing.T) {
	type T = []string
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.testOK(t, "three_items", `[ "a", "", "cde" ]`, T{"a", "", "cde"})
	s.testOK(t, "one_items", `[ "abc" ]`, T{"abc"})
	s.testOK(t, "escaped", `[ "\"abc\tdef\"" ]`, T{"\"abc\tdef\""})
	s.testOK(t, "unicode", `["жзш","ツ!"]`, T{`жзш`, `ツ!`})
	s.testOK(t, "empty", `[]`, T{})
	s.testOK(t, "null", `null`, T(nil))
	s.testOK(t, "null_element", `[null]`, T{""})
	s.testOK(t, "null_element_multi", `[null,"okay",null]`, T{"", "okay", ""})
	s.testOK(t, "array_str_1024_639k", arrayStr1024)

	s.testErr(t, "wrong_type_string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_object", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_element_array", `["okay",[]]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 8})
	s.testErr(t, "wrong_type_element_float", `["okay",3.14]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 8})
}

func TestDecodeSliceBool(t *testing.T) {
	type T = []bool
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.testOK(t, "three_items", `[ true, false, true ]`, T{true, false, true})
	s.testOK(t, "one_items", `[ true ]`, T{true})
	s.testOK(t, "empty", `[]`, T{})
	s.testOK(t, "null", `null`, T(nil))
	s.testOK(t, "null_element", `[null]`, T{false})
	s.testOK(t, "null_element_multi", `[null,true,null]`, T{false, true, false})

	s.testErr(t, "wrong_type_string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_object", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_element_array", `[true,[]]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 6})
	s.testErr(t, "wrong_type_element_float", `[true,3.14]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 6})
}

func TestDecodeSliceFloat32(t *testing.T) {
	type T = []float32
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.testOK(t, "three_items", `[0, 3.14, 2.5]`, T{0, 3.14, 2.5})
	s.testOK(t, "1", `[1]`, T{1})
	s.testOK(t, "-1", `[-1]`, T{-1})
	s.testOK(t, "min_int", `[-16777215]`, T{-16_777_215})
	s.testOK(t, "max_int", `[16777215]`, T{16_777_215})
	s.testOK(t, "pi7", `[3.1415927]`, T{3.1415927})
	s.testOK(t, "-pi7", `[-3.1415927]`, T{-3.1415927})
	s.testOK(t, "3.4028235e38", `[3.4028235e38]`, T{3.4028235e38})
	s.testOK(t, "min_pos", `[1.4e-45]`, T{1.4e-45})
	s.testOK(t, "3.4e38", `[3.4e38]`, T{3.4e38})
	s.testOK(t, "-3.4e38", `[-3.4e38]`, T{-3.4e38})
	s.testOK(t, "avogadros_num", `[6.022e23]`, T{6.022e23})

	s.testErrCheck(t, "range_hi", `[3.5e38]`,
		func(t *testing.T, err jscandec.ErrorDecode) {
			require.ErrorIs(t, err.Err, strconv.ErrRange)
			require.Equal(t, 1, err.Index)
		})

	s.testErr(t, "wrong_type_string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_object", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_element_array", `[1,[]]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
}

func TestDecodeSliceFloat64(t *testing.T) {
	type T = []float64
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.testOK(t, "three_items", `[0, 3.14, 2.5]`, T{0, 3.14, 2.5})
	s.testOK(t, "0", `[0]`, T{0})
	s.testOK(t, "1", `[1]`, T{1})
	s.testOK(t, "-1", `[-1]`, T{-1})
	s.testOK(t, "1.0", `[1.0]`, T{1})
	s.testOK(t, "1.000000003", `[1.000000003]`, T{1.000000003})
	s.testOK(t, "max_int", `[9007199254740991]`, T{9_007_199_254_740_991})
	s.testOK(t, "min_int", `[-9007199254740991]`, T{-9_007_199_254_740_991})
	s.testOK(t, "pi",
		`[3.141592653589793238462643383279502884197]`,
		T{3.141592653589793238462643383279502884197})
	s.testOK(t, "pi_neg",
		`[-3.141592653589793238462643383279502884197]`,
		T{-3.141592653589793238462643383279502884197})
	s.testOK(t, "3.4028235e38", `[3.4028235e38]`, T{3.4028235e38})
	s.testOK(t, "exponent", `[1.7976931348623157e308]`, T{1.7976931348623157e308})
	s.testOK(t, "neg_exponent", `[1.7976931348623157e-308]`, T{1.7976931348623157e-308})
	s.testOK(t, "1.4e-45", `[1.4e-45]`, T{1.4e-45})
	s.testOK(t, "neg_exponent", `[-1.7976931348623157e308]`, T{-1.7976931348623157e308})
	s.testOK(t, "3.4e38", `[3.4e38]`, T{3.4e38})
	s.testOK(t, "-3.4e38", `[-3.4e38]`, T{-3.4e38})
	s.testOK(t, "avogadros_num", `[6.022e23]`, T{6.022e23})
	s.testOK(t, "array_float_1024", arrayFloat1024)

	s.testErrCheck(t, "range_hi", `[1e309]`,
		func(t *testing.T, err jscandec.ErrorDecode) {
			require.ErrorIs(t, err.Err, strconv.ErrRange)
			require.Equal(t, 1, err.Index)
		})

	s.testErr(t, "wrong_type_string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_object", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "wrong_type_element_array", `[1,[]]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
	s.testErr(t, "wrong_type_element_string", `[1,"nope"]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 3})
}

func TestDecode2DSliceInt(t *testing.T) {
	type T = [][]int
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
	s.testOK(t, "3_2", `[[0],[12, 123],[ ]]`, T{{0}, {12, 123}, {}})
	s.testOK(t, "2_1", `[[],[-12345678]]`, T{{}, {-12_345_678}})
	s.testOK(t, "2_0", `[[],[]]`, T{{}, {}})
	s.testOK(t, "1", `[]`, T{})
}

func TestDecodeMatrix2Int(t *testing.T) {
	type T = [2][2]int
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
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
	s := newTestSetup[T](t, *jscandec.DefaultOptions)
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

func TestDecodeEmptyStruct(t *testing.T) {
	type S struct{}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.testOK(t, "null", `null`, S{})
	s.testOK(t, "empty_object", `{}`, S{})
	s.testOK(t, "object", `{"x":"y"}`, S{})
	s.testOK(t, "object_multikey",
		`{"x":"y","abc":[{"x":"y","2":42}, null, {}]}`, S{})

	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array_empty", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[{},{}]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeSliceEmptyStruct(t *testing.T) {
	type S struct{}
	s := newTestSetup[[]S](t, *jscandec.DefaultOptions)
	s.testOK(t, "null", `null`, []S(nil))
	s.testOK(t, "empty_array", `[]`, []S{})
	s.testOK(t, "array_one", `[{}]`, []S{{}})
	s.testOK(t, "array_multiple", `[{},{},{}]`, []S{{}, {}, {}})

	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "object_empty", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "object", `{"x":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeArray(t *testing.T) {
	s := newTestSetup[[3]int](t, *jscandec.DefaultOptions)
	s.testOK(t, "null", `null`, [3]int{})
	s.testOK(t, "empty_array", `[]`, [3]int{})
	s.testOK(t, "array_one", `[1]`, [3]int{1, 0, 0})
	s.testOK(t, "array_full", `[1,2,3]`, [3]int{1, 2, 3})
	s.testOK(t, "array_overflow",
		`[1,2,3,false,true,{},{"x":"y"},[],null,42,3.14]`, [3]int{1, 2, 3})

	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "float", `3.14`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "object_empty", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "object", `{"x":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeArray2D(t *testing.T) {
	type A [2][2]int
	s := newTestSetup[A](t, *jscandec.DefaultOptions)
	s.testOK(t, "null", `null`, A{})
	s.testOK(t, "empty_array", `[]`, A{})
	s.testOK(t, "array_one", `[[]]`, A{})
	s.testOK(t, "array_full", `[[1,2],[3,4]]`, A{{1, 2}, {3, 4}})
	s.testOK(t, "array_overflow",
		`[[1,2],[3,4],false,true,{},{"x":"y"},[],null,42,3.14]`, A{{1, 2}, {3, 4}})
	s.testOK(t, "array_overflow_in_subarray",
		`[[1,2, 3,[],{}],[4,5, 6,[],{}],false,true,{},{"x":"y"},[],null,42,3.14]`,
		A{{1, 2}, {4, 5}})

	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "float", `3.14`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "object_empty", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "object", `{"x":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeArrayLen0(t *testing.T) {
	s := newTestSetup[[0]int](t, *jscandec.DefaultOptions)
	s.testOK(t, "null", `null`, [0]int{})
	s.testOK(t, "empty_array", `[]`, [0]int{})
	s.testOK(t, "array_one", `[1]`, [0]int{})
	s.testOK(t, "array_one_empty_object", `[{}]`, [0]int{})
	s.testOK(t, "array_multiple",
		`[false,true,{},{"x":"y"},[],null,42,3.14]`, [0]int{})

	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "float", `3.14`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "object_empty", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "object", `{"x":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeArrayLen02D(t *testing.T) {
	type A [0][0]int
	s := newTestSetup[A](t, *jscandec.DefaultOptions)
	s.testOK(t, "null", `null`, A{})
	s.testOK(t, "empty_array", `[]`, A{})
	s.testOK(t, "array_one", `[1]`, A{})
	s.testOK(t, "array_one_empty_object", `[{}]`, A{})
	s.testOK(t, "array_multiple",
		`[false,true,{},{"x":"y"},[],null,42,3.14]`, A{})

	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "float", `3.14`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "object_empty", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "object", `{"x":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeArrayArrayLen0(t *testing.T) {
	type A [2][0]int
	s := newTestSetup[A](t, *jscandec.DefaultOptions)
	s.testOK(t, "null", `null`, A{})
	s.testOK(t, "empty_array", `[]`, A{})
	s.testOK(t, "array_overflow",
		`[[],[],false,true,{},{"x":"y"},[],null,42,3.14]`, A{})
	s.testOK(t, "array_overflow_in_subarray",
		`[["foo",1.2],["bar",3.4],false,true,{},{"x":"y"},[],null,42,3.14]`, A{})

	s.testErr(t, "array_int", `[1,2]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "array_one_empty_object", `[{}]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "float", `3.14`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "object_empty", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "object", `{"x":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeArrayEmptyStruct(t *testing.T) {
	type S struct{}
	s := newTestSetup[[3]S](t, *jscandec.DefaultOptions)
	s.testOK(t, "null", `null`, [3]S{})
	s.testOK(t, "empty_array", `[]`, [3]S{})
	s.testOK(t, "array_one", `[{}]`, [3]S{})
	s.testOK(t, "array_full", `[{},{},{}]`, [3]S{})
	s.testOK(t, "array_overflow", `[{},{},{},{},{},{},{},{}, {}]`, [3]S{})

	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "object_empty", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "object", `{"x":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeStruct(t *testing.T) {
	type S struct {
		Foo int    `json:"foo"`
		Bar string `json:"bar"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
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

func TestDecodeStructErrUknownField(t *testing.T) {
	type S struct {
		Foo int    `json:"foo"`
		Bar string `json:"bar"`
	}
	s := newTestSetup[S](t, jscandec.DecodeOptions{DisallowUnknownFields: true})
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

	s.testOK(t, "empty", `{}`, S{})

	s.testErr(t, "unknown_field",
		`{"bar":"bar","unknown":42,"foo":102}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnknownField, Index: 13})
	s.testErr(t, "unknown_fields_only",
		`{"unknown":42, "unknown2": "bad"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnknownField, Index: 1})
	s.testErr(t, "name_mismatch", `{"faz":42,"baz":"bazz"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnknownField, Index: 1})

	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeStructFields(t *testing.T) {
	type S struct {
		Any   any
		Map   map[string]any
		Slice []any
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.testOK(t, "case_insensitive_match",
		`{"any":42,"Map":{"foo":"bar"},"SLICE":[1,false,"x"]}`,
		S{
			Any:   float64(42),
			Map:   map[string]any{"foo": "bar"},
			Slice: []any{float64(1), false, "x"},
		})
	s.testOK(t, "different_order_and_types",
		`{"map":{},"slice":[{"1":"2", "3":4},null,[]],"any":{"x":"y"}}`,
		S{
			Map:   map[string]any{},
			Slice: []any{map[string]any{"1": "2", "3": float64(4)}, nil, []any{}},
			Any:   map[string]any{"x": "y"},
		})
	s.testOK(t, "null_fields",
		`{"map":null,"slice":null,"any":null}`,
		S{})
	s.testOK(t, "partial_one_field",
		`{"slice":[{"x":false,"y":42},{"Имя":"foo"},{"x":{}}]}`,
		S{Slice: []any{
			map[string]any{"x": false, "y": float64(42)},
			map[string]any{"Имя": "foo"},
			map[string]any{"x": map[string]any{}},
		}})

	s.testOK(t, "null", `null`, S{})
	s.testOK(t, "empty", `{}`, S{})
	s.testOK(t, "name_mismatch", `{"faz":42,"baz":"bazz"}`, S{})

	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeStringTagString(t *testing.T) {
	type S struct {
		String string `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty_string",
		`{"string":"\"\""}`, S{String: ""})
	s.testOK(t, "space",
		`{"string":"\" \""}`, S{String: " "})
	s.testOK(t, "text",
		`{"string":"\"text\""}`, S{String: "text"})

	s.testErr(t, "empty", `{"string":""}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})
	s.testErr(t, "space_prefix", `{"string":" \"\""}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})
	s.testErr(t, "space_suffix", `{"string":"\"\" "}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})
	s.testErr(t, "multiple_strings", `{"string":"\"first\"\"second\""}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})
	s.testErr(t, "suffix_new_line", `{"string":"\"okay\"\n"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})
	s.testErr(t, "suffix_text", `{"string":"\"okay\"abc"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})
	s.testErr(t, "suffix_text", `{"string":"\"ok\"\"ay\""}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})

	s.testOK(t, "empty", `{}`, S{})
	s.testOK(t, "null", `null`, S{})

	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeStringTagInt(t *testing.T) {
	type S struct {
		Int int `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, S{})
	s.testOK(t, "null", `null`, S{})
	s.testOK(t, "min", `{"int":"-9223372036854775808"}`, S{Int: -9223372036854775808})
	s.testOK(t, "max", `{"int":"9223372036854775807"}`, S{Int: 9223372036854775807})

	s.testErr(t, "overflow_hi", `{"int":"9223372036854775808"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 7})
	s.testErr(t, "overflow_lo", `{"int":"-9223372036854775809"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 7})
	s.testErr(t, "float", `{"int":"3.14"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 7})
	s.testErr(t, "exponent", `{"int":"3e2"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 7})
}

func TestDecodeStringTagInt8(t *testing.T) {
	type S struct {
		Int8 int8 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, S{})
	s.testOK(t, "null", `null`, S{})
	s.testOK(t, "min", `{"int8":"-128"}`, S{Int8: -128})
	s.testOK(t, "max", `{"int8":"127"}`, S{Int8: 127})

	s.testErr(t, "overflow_hi", `{"int8":"128"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 8})
	s.testErr(t, "overflow_lo", `{"int8":"-129"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 8})
	s.testErr(t, "float", `{"int8":"3.14"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 8})
	s.testErr(t, "exponent", `{"int8":"3e2"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 8})
}

func TestDecodeStringTagInt16(t *testing.T) {
	type S struct {
		Int16 int16 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, S{})
	s.testOK(t, "null", `null`, S{})
	s.testOK(t, "min", `{"int16":"-32768"}`, S{Int16: -32768})
	s.testOK(t, "max", `{"int16":"32767"}`, S{Int16: 32767})

	s.testErr(t, "overflow_hi", `{"int16":"32768"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 9})
	s.testErr(t, "overflow_lo", `{"int16":"-32769"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 9})
	s.testErr(t, "float", `{"int16":"3.14"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 9})
	s.testErr(t, "exponent", `{"int16":"3e2"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 9})
}

func TestDecodeStringTagInt32(t *testing.T) {
	type S struct {
		Int32 int32 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, S{})
	s.testOK(t, "null", `null`, S{})
	s.testOK(t, "min", `{"int32":"-2147483648"}`, S{Int32: -2147483648})
	s.testOK(t, "max", `{"int32":"2147483647"}`, S{Int32: 2147483647})

	s.testErr(t, "overflow_hi", `{"int32":"2147483648"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 9})
	s.testErr(t, "overflow_lo", `{"int32":"-2147483649"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 9})
	s.testErr(t, "float", `{"int32":"3.14"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 9})
	s.testErr(t, "exponent", `{"int32":"3e2"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 9})
}

func TestDecodeStringTagInt64(t *testing.T) {
	type S struct {
		Int64 int64 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, S{})
	s.testOK(t, "null", `null`, S{})
	s.testOK(t, "min", `{"int64":"-9223372036854775808"}`,
		S{Int64: -9223372036854775808})
	s.testOK(t, "max", `{"int64":"9223372036854775807"}`,
		S{Int64: 9223372036854775807})

	s.testErr(t, "overflow_hi", `{"int64":"9223372036854775808"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 9})
	s.testErr(t, "overflow_lo", `{"int64":"-9223372036854775809"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 9})
	s.testErr(t, "float", `{"int64":"3.14"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 9})
	s.testErr(t, "exponent", `{"int64":"3e2"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 9})
}

func TestDecodeStringTagUint(t *testing.T) {
	type S struct {
		Uint uint `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, S{})
	s.testOK(t, "null", `null`, S{})
	s.testOK(t, "min", `{"uint":"0"}`, S{Uint: 0})
	s.testOK(t, "max", `{"uint":"18446744073709551615"}`, S{Uint: 18446744073709551615})

	s.testErr(t, "overflow_hi", `{"uint":"18446744073709551616"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 8})
	s.testErr(t, "negative", `{"uint":"-1"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 8})
	s.testErr(t, "float", `{"uint":"3.14"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 8})
	s.testErr(t, "exponent", `{"uint":"3e2"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 8})
}

func TestDecodeStringTagUint8(t *testing.T) {
	type S struct {
		Uint8 uint8 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, S{})
	s.testOK(t, "null", `null`, S{})
	s.testOK(t, "min", `{"uint8":"0"}`, S{Uint8: 0})
	s.testOK(t, "max", `{"uint8":"255"}`, S{Uint8: 255})

	s.testErr(t, "overflow_hi", `{"uint8":"256"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 9})
	s.testErr(t, "negative", `{"uint8":"-1"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 9})
	s.testErr(t, "float", `{"uint8":"3.14"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 9})
	s.testErr(t, "exponent", `{"uint8":"3e2"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 9})
}

func TestDecodeStringTagUint16(t *testing.T) {
	type S struct {
		Uint16 uint16 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, S{})
	s.testOK(t, "null", `null`, S{})
	s.testOK(t, "min", `{"uint16":"0"}`, S{Uint16: 0})
	s.testOK(t, "max", `{"uint16":"65535"}`, S{Uint16: 65535})

	s.testErr(t, "overflow_hi", `{"uint16":"65536"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})
	s.testErr(t, "negative", `{"uint16":"-1"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})
	s.testErr(t, "float", `{"uint16":"3.14"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})
	s.testErr(t, "exponent", `{"uint16":"3e2"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})
}

func TestDecodeStringTagUint32(t *testing.T) {
	type S struct {
		Uint32 uint32 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, S{})
	s.testOK(t, "null", `null`, S{})
	s.testOK(t, "min", `{"uint32":"0"}`, S{Uint32: 0})
	s.testOK(t, "max", `{"uint32":"4294967295"}`,
		S{Uint32: 4294967295})

	s.testErr(t, "overflow_hi", `{"uint32":"4294967296"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})
	s.testErr(t, "negative", `{"uint32":"-1"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})
	s.testErr(t, "float", `{"uint32":"3.14"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})
	s.testErr(t, "exponent", `{"uint32":"3e2"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})
}

func TestDecodeStringTagUint64(t *testing.T) {
	type S struct {
		Uint64 uint64 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, S{})
	s.testOK(t, "null", `null`, S{})
	s.testOK(t, "min", `{"uint64":"0"}`, S{Uint64: 0})
	s.testOK(t, "max", `{"uint64":"18446744073709551615"}`,
		S{Uint64: 18446744073709551615})

	s.testErr(t, "overflow_hi", `{"uint64":"18446744073709551616"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})
	s.testErr(t, "negative", `{"uint64":"-1"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})
	s.testErr(t, "float", `{"uint64":"3.14"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})
	s.testErr(t, "exponent", `{"uint64":"3e2"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 10})
}

func TestDecodeStringTagFloat32(t *testing.T) {
	type S struct {
		Float32 float32 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, S{})
	s.testOK(t, "null", `null`, S{})
	s.testOK(t, "zero", `{"float32":"0"}`, S{Float32: 0})
	s.testOK(t, "integer", `{"float32":"123"}`, S{Float32: 123})
	s.testOK(t, "-pi7", `{"float32":"-3.1415927"}`, S{Float32: -3.1415927})
	s.testOK(t, "avogadros_num", `{"float32":"6.022e23"}`, S{Float32: 6.022e23})

	s.testErrCheck(t, "range_hi", `{"float32":"3.5e38"}`,
		func(t *testing.T, err jscandec.ErrorDecode) {
			require.ErrorIs(t, err.Err, strconv.ErrRange)
			require.Equal(t, 11, err.Index)
		})
}

func TestDecodeStringTagFloat64(t *testing.T) {
	type S struct {
		Float64 float64 `json:",string"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, S{})
	s.testOK(t, "null", `null`, S{})
	s.testOK(t, "zero", `{"float64":"0"}`, S{Float64: 0})
	s.testOK(t, "integer", `{"float64":"123"}`, S{Float64: 123})
	s.testOK(t, "1.000000003", `{"float64":"1.000000003"}`,
		S{Float64: 1.000000003})
	s.testOK(t, "max_int", `{"float64":"9007199254740991"}`,
		S{Float64: 9_007_199_254_740_991})
	s.testOK(t, "min_int", `{"float64":"-9007199254740991"}`,
		S{Float64: -9_007_199_254_740_991})
	s.testOK(t, "pi",
		`{"float64":"3.141592653589793238462643383279502884197"}`,
		S{Float64: 3.141592653589793238462643383279502884197})
	s.testOK(t, "pi_neg",
		`{"float64":"-3.141592653589793238462643383279502884197"}`,
		S{Float64: -3.141592653589793238462643383279502884197})
	s.testOK(t, "3.4028235e38", `{"float64":"3.4028235e38"}`,
		S{Float64: 3.4028235e38})
	s.testOK(t, "exponent", `{"float64":"1.7976931348623157e308"}`,
		S{Float64: 1.7976931348623157e308})
	s.testOK(t, "neg_exponent", `{"float64":"1.7976931348623157e-308"}`,
		S{Float64: 1.7976931348623157e-308})
	s.testOK(t, "1.4e-45", `{"float64":"1.4e-45"}`,
		S{Float64: 1.4e-45})
	s.testOK(t, "neg_exponent", `{"float64":"-1.7976931348623157e308"}`,
		S{Float64: -1.7976931348623157e308})
	s.testOK(t, "3.4e38", `{"float64":"3.4e38"}`,
		S{Float64: 3.4e38})
	s.testOK(t, "-3.4e38", `{"float64":"-3.4e38"}`,
		S{Float64: -3.4e38})
	s.testOK(t, "avogadros_num", `{"float64":"6.022e23"}`,
		S{Float64: 6.022e23})

	s.testErrCheck(t, "range_hi", `{"float64":"1e309"}`,
		func(t *testing.T, err jscandec.ErrorDecode) {
			require.ErrorIs(t, err.Err, strconv.ErrRange)
			require.Equal(t, 11, err.Index)
		})
}

func TestDecodePointerInt(t *testing.T) {
	s := newTestSetup[*int](t, *jscandec.DefaultOptions)
	s.testOK(t, "valid", `42`, Ptr(int(42)))

	s.testErr(t, "float", `1.1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodePointerStruct(t *testing.T) {
	type S struct {
		Foo string `json:"foo"`
		Bar any    `json:"bar"`
	}
	s := newTestSetup[*S](t, *jscandec.DefaultOptions)
	s.testOK(t, "valid", `{"foo":"™","bar":[1,true]}`, &S{
		Foo: "™",
		Bar: []any{float64(1), true},
	})

	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodePointerAny(t *testing.T) {
	s := newTestSetup[*any](t, *jscandec.DefaultOptions)
	s.testOK(t, "int", `[1]`, Ptr(any([]any{float64(1)})))
	s.testOK(t, "string", `"text"`, Ptr(any("text")))
	s.testOK(t, "array_int", `[1]`, Ptr(any([]any{float64(1)})))
	s.testOK(t, "array_int", `{"foo":1}`, Ptr(any(map[string]any{"foo": float64(1)})))
}

func TestDecodePointer3DInt(t *testing.T) {
	s := newTestSetup[***int](t, *jscandec.DefaultOptions)
	s.testOK(t, "valid", `42`, Ptr(Ptr(Ptr(int(42)))))

	s.testErr(t, "float", `1.1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func Ptr[T any](v T) *T { return &v }

func TestDecodeStructSlice(t *testing.T) {
	type S struct {
		Foo int    `json:"foo"`
		Bar string `json:"bar"`
	}
	s := newTestSetup[[]S](t, *jscandec.DefaultOptions)
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
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
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
	s.testOK(t, "multiple_overrides",
		`{"":"1", "":"12", "":"123"}`, M{"": "123"}) // Take last
	s.testOK(t, "many",
		`{
			"foo": "1", "bar": "a", "baz": "2", "muzz": "",
			"longer_key": "longer test text"
		}`, M{
			"foo": "1", "bar": "a", "baz": "2", "muzz": "",
			"longer_key": "longer test text",
		})
	s.testOK(t, "escaped",
		`{"\"key\"":"\"value\"\t\u0042"}`, M{"\"key\"": "\"value\"\tB"})

	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeMapIntToString(t *testing.T) {
	type M map[int]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, M{})
	s.testOK(t, "null", `null`, M(nil))
	s.testOK(t, "positive_and_negative", `{"0":0, "42":42, "-123456789":123456789}`,
		M{0: 0, 42: 42, -123456789: 123456789})

	s.testErr(t, "overflow_hi", `{"9223372036854775808":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "overflow_lo", `{"-9223372036854775809":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "float", `{"3.14":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "exponent", `{"3e2":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeMapInt8ToString(t *testing.T) {
	type M map[int8]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, M{})
	s.testOK(t, "null", `null`, M(nil))
	s.testOK(t, "min_and_max", `{"0":0, "-128":-128, "127":127}`,
		M{0: 0, -128: -128, 127: 127})

	s.testErr(t, "overflow_hi", `{"128":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "overflow_lo", `{"-129":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "float", `{"3.14":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "exponent", `{"3e2":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeMapInt16ToString(t *testing.T) {
	type M map[int16]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, M{})
	s.testOK(t, "null", `null`, M(nil))
	s.testOK(t, "min_and_max", `{"0":0, "-32768":-32768, "32767":32767}`,
		M{0: 0, -32768: -32768, 32767: 32767})

	s.testErr(t, "overflow_hi", `{"32768":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "overflow_lo", `{"-32769":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "float", `{"3.14":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "exponent", `{"3e2":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeMapInt32ToString(t *testing.T) {
	type M map[int32]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, M{})
	s.testOK(t, "null", `null`, M(nil))
	s.testOK(t, "min_and_max", `{"0":0,
		"-2147483648":-2147483648, "2147483647":2147483647}`,
		M{0: 0, -2147483648: -2147483648, 2147483647: 2147483647})

	s.testErr(t, "overflow_hi", `{"2147483648":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "overflow_lo", `{"-2147483649":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "float", `{"3.14":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "exponent", `{"3e2":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeMapInt64ToString(t *testing.T) {
	type M map[int64]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, M{})
	s.testOK(t, "null", `null`, M(nil))
	s.testOK(t, "min_and_max", `{"0":0,
		"-9223372036854775808":-9223372036854775808,
		"9223372036854775807":9223372036854775807}`,
		M{
			0:                    0,
			-9223372036854775808: -9223372036854775808,
			9223372036854775807:  9223372036854775807,
		})

	s.testErr(t, "overflow_hi", `{"9223372036854775808":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "overflow_lo", `{"-9223372036854775809":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "float", `{"3.14":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "exponent", `{"3e2":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeMapUintToString(t *testing.T) {
	type M map[uint]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, M{})
	s.testOK(t, "null", `null`, M(nil))
	s.testOK(t, "positive_and_negative", `{"0":0, "42":42, "18446744073709551615":1}`,
		M{0: 0, 42: 42, 18446744073709551615: 1})

	s.testErr(t, "overflow", `{"18446744073709551616":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "negative", `{"-1":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "float", `{"3.14":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "exponent", `{"3e2":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeMapUint8ToString(t *testing.T) {
	type M map[uint8]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, M{})
	s.testOK(t, "null", `null`, M(nil))
	s.testOK(t, "positive_and_negative", `{"0":0, "42":42, "255":1}`,
		M{0: 0, 42: 42, 255: 1})

	s.testErr(t, "overflow", `{"256":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "negative", `{"-1":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "float", `{"3.14":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "exponent", `{"3e2":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeMapUint16ToString(t *testing.T) {
	type M map[uint16]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, M{})
	s.testOK(t, "null", `null`, M(nil))
	s.testOK(t, "positive_and_negative", `{"0":0, "42":42, "65535":1}`,
		M{0: 0, 42: 42, 65535: 1})

	s.testErr(t, "overflow", `{"65536":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "negative", `{"-1":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "float", `{"3.14":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "exponent", `{"3e2":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeMapUint32ToString(t *testing.T) {
	type M map[uint32]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, M{})
	s.testOK(t, "null", `null`, M(nil))
	s.testOK(t, "positive_and_negative", `{"0":0, "42":42, "4294967295":1}`,
		M{0: 0, 42: 42, 4294967295: 1})

	s.testErr(t, "overflow", `{"4294967296":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "negative", `{"-1":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "float", `{"3.14":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "exponent", `{"3e2":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "int", `1`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "string", `"text"`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeMapUint64ToString(t *testing.T) {
	type M map[uint64]int
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, M{})
	s.testOK(t, "null", `null`, M(nil))
	s.testOK(t, "positive_and_negative", `{"0":0, "42":42, "18446744073709551615":1}`,
		M{0: 0, 42: 42, 18446744073709551615: 1})

	s.testErr(t, "overflow", `{"18446744073709551616":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "negative", `{"-1":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "float", `{"3.14":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
	s.testErr(t, "exponent", `{"3e2":0}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 1})
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
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
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
	s.testOK(t, "multiple_overrides",
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
	s.testOK(t, "escaped",
		`{" \b " : {" \" ":" \u30C4 "} }`, M{" \b ": M2{` " `: ` ツ `}})

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
	s := newTestSetup[M](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, M{})
	s.testOK(t, "one",
		`{"x":{"name":"first","id":1}}`, M{"x": S{Name: "first", ID: 1}})
	s.testOK(t, "empty_struct",
		`{"x":{}}`, M{"x": {}})
	s.testOK(t, "null_value",
		`{"":null}`, M{"": {}})
	s.testOK(t, "escaped_key",
		`{"\u30c4":{}}`, M{`ツ`: {}})
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

func TestDecodeJSONUnmarshaler(t *testing.T) {
	s := newTestSetup[jsonUnmarshalerImpl](t, *jscandec.DefaultOptions)
	s.testOK(t, "integer", `123`, jsonUnmarshalerImpl{Value: `123`})
	s.testOK(t, "float", `3.14`, jsonUnmarshalerImpl{Value: `3.14`})
	s.testOK(t, "string", `"okay"`, jsonUnmarshalerImpl{Value: `"okay"`})
	s.testOK(t, "true", `true`, jsonUnmarshalerImpl{Value: `true`})
	s.testOK(t, "false", `false`, jsonUnmarshalerImpl{Value: `false`})
	s.testOK(t, "null", `null`, jsonUnmarshalerImpl{Value: `null`})
	s.testOK(t, "array_empty", `[]`, jsonUnmarshalerImpl{Value: `[]`})
	s.testOK(t, "array", `[1,"okay",true,{ }]`,
		jsonUnmarshalerImpl{Value: `[1,"okay",true,{ }]`})
	s.testOK(t, "object_empty", `{}`, jsonUnmarshalerImpl{Value: `{}`})
	s.testOK(t, "object_empty", `{"foo":{"bar":"baz"}}`,
		jsonUnmarshalerImpl{Value: `{"foo":{"bar":"baz"}}`})
}

func TestDecodeTextUnmarshaler(t *testing.T) {
	s := newTestSetup[textUnmarshalerImpl](t, *jscandec.DefaultOptions)
	s.testOK(t, "string", `"text"`, textUnmarshalerImpl{Value: `text`})
	s.testOK(t, "string_escaped", `"\"text\""`, textUnmarshalerImpl{Value: `"text"`})
	s.testOK(t, "null", `null`, textUnmarshalerImpl{Value: ``})
	s.testOK(t, "string_empty", `""`, textUnmarshalerImpl{Value: ``})

	s.testErr(t, "int", `123`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "float", `3.14`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array_empty", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `["foo"]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "object_empty", `{}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "object", `{"foo":"bar"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
}

func TestDecodeTextUnmarshalerMapKey(t *testing.T) {
	type U = textUnmarshalerImpl
	s := newTestSetup[map[U]int](t, *jscandec.DefaultOptions)
	s.testOK(t, "empty", `{}`, map[U]int{})
	s.testOK(t, "null", `null`, map[U]int(nil))
	s.testOK(t, "text", `{"text":1}`, map[U]int{{Value: "text"}: 1})
	s.testOK(t, "empty_key", `{"":2}`, map[U]int{{Value: ""}: 2})
	s.testOK(t, "escaped", `{"\"escaped\tkey\"":3}`,
		map[U]int{{Value: "\"escaped\tkey\""}: 3})

	s.testErr(t, "int", `123`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "float", `3.14`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "false", `false`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "true", `true`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array_empty", `[]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "array", `["foo"]`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 0})
	s.testErr(t, "object", `{"foo":"bar"}`,
		jscandec.ErrorDecode{Err: jscandec.ErrUnexpectedValue, Index: 7})
}

func TestDecodeUnmarshalerFields(t *testing.T) {
	type S struct {
		String string              `json:"string"`
		JSON   jsonUnmarshalerImpl `json:"json"`
		Text   textUnmarshalerImpl `json:"text"`
		Tail   []int               `json:"tail"`
	}
	s := newTestSetup[S](t, *jscandec.DefaultOptions)
	s.testOK(t, "integer",
		`{"string":"a","json":42,"text":"foo","tail":[1,2]}`,
		S{
			String: "a",
			JSON:   jsonUnmarshalerImpl{Value: `42`},
			Text:   textUnmarshalerImpl{Value: "foo"},
			Tail:   []int{1, 2},
		})
	s.testOK(t, "string", `{
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
	s.testOK(t, "array", `{"string":"c","json":[1,2, 3],"text":"","tail":[1,2]}`,
		S{
			String: "c",
			JSON:   jsonUnmarshalerImpl{Value: `[1,2, 3]`},
			Text:   textUnmarshalerImpl{Value: ""},
			Tail:   []int{1, 2},
		})
	s.testOK(t, "object", `{"string":"d","json":{"foo":["bar", null]},"tail":[1,2]}`,
		S{
			String: "d",
			JSON:   jsonUnmarshalerImpl{Value: `{"foo":["bar", null]}`},
			Text:   textUnmarshalerImpl{Value: ""},
			Tail:   []int{1, 2},
		})
}

func TestDecodeJSONUnmarshalerErr(t *testing.T) {
	s := newTestSetup[unmarshalerImplErr](t, *jscandec.DefaultOptions)
	s.testErrCheck(t, "integer", `123`, func(t *testing.T, err jscandec.ErrorDecode) {
		require.Equal(t, errUnmarshalerImpl, err.Err)
	})

	s2 := newTestSetup[map[textUnmarshalerImplErr]struct{}](t, *jscandec.DefaultOptions)
	s2.testErrCheck(t, "map", `{"x":"y"}`, func(t *testing.T, err jscandec.ErrorDecode) {
		require.Equal(t, errTextUnmarshalerImpl, err.Err)
	})

	type S struct{ Unmarshaler textUnmarshalerImplErr }
	s3 := newTestSetup[S](t, *jscandec.DefaultOptions)
	s3.testErrCheck(t, "struct_field", `{"Unmarshaler":"abc"}`,
		func(t *testing.T, err jscandec.ErrorDecode) {
			require.Equal(t, errTextUnmarshalerImpl, err.Err)
		})
}

func TestDecodeSyntaxErrorUnexpectedEOF(t *testing.T) {
	tokenizerString := jscan.NewTokenizer[string](16, 1024)
	d, err := jscandec.NewDecoder[string, []int](tokenizerString)
	require.NoError(t, err)
	var v []int
	errDecode := d.Decode(`[1,2,3`, &v, jscandec.DefaultOptions)
	var jscanErr jscan.Error[string]
	require.True(t, errors.As(errDecode.Err, &jscanErr))
	require.Equal(t, jscan.ErrorCodeUnexpectedEOF, jscanErr.Code)
}

func BenchmarkSmall(b *testing.B) {
	in := []byte(`[[true],[false,false,false,false],[],[],[true]]`) // 18 tokens

	b.Run("jscan", func(b *testing.B) {
		tok := jscan.NewTokenizer[[]byte](8, 64)
		d, err := jscandec.NewDecoder[[]byte, [][]bool](tok)
		if err != nil {
			b.Fatalf("initializing decoder: %v", err)
		}
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			var v [][]bool
			if err := d.Decode(in, &v, jscandec.DefaultOptions); err.IsErr() {
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
