package bench_test

import (
	json "encoding/json"
	"os"
	"runtime"
	"testing"

	jscandec "github.com/romshark/jscan-experimental-decoder"
	"github.com/romshark/jscan-experimental-decoder/bench"
	"github.com/romshark/jscan-experimental-decoder/bench/easyjsongen"
	"github.com/romshark/jscan-experimental-decoder/bench/ffjsongen"

	jsonv2 "github.com/go-json-experiment/json"
	goccy "github.com/goccy/go-json"
	jsoniter "github.com/json-iterator/go"
	easyjson "github.com/mailru/easyjson"
	ffjson "github.com/pquerna/ffjson/ffjson"
	jscan "github.com/romshark/jscan/v2"
	segmentio "github.com/segmentio/encoding/json"
	"github.com/stretchr/testify/require"
)

func TestImplementationsDecodeArrayString(t *testing.T) {
	// This file was generated using https://go.dev/play/p/GduL-yXAYN-
	in, err := os.ReadFile("../testdata/array_string_15k.json")
	require.NoError(t, err)

	expect := func() (expect []string) {
		require.NoError(t, json.Unmarshal(in, &expect))
		return expect
	}

	t.Run("unmr/encoding_json", func(t *testing.T) {
		var v []string
		require.NoError(t, json.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/jsoniter", func(t *testing.T) {
		var v []string
		require.NoError(t, jsoniter.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/goccy", func(t *testing.T) {
		var v []string
		require.NoError(t, goccy.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/jsonv2", func(t *testing.T) {
		var v []string
		require.NoError(t, jsonv2.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/segmentio", func(t *testing.T) {
		var v []string
		require.NoError(t, segmentio.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/jscan", func(t *testing.T) {
		d, err := jscandec.NewDecoder[[]byte, []string](
			jscan.NewTokenizer[[]byte](2048, 2048*1024), jscandec.DefaultInitOptions,
		)
		require.NoError(t, err)
		var v []string
		if err := d.Decode([]byte(in), &v, jscandec.DefaultOptions); err.IsErr() {
			t.Fatal(err)
		}
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/jscan_unmarshal", func(t *testing.T) {
		var v []string
		if err := jscandec.Unmarshal([]byte(in), &v); err != nil {
			t.Fatal(err)
		}
		require.Equal(t, expect(), v)
	})

	t.Run("genr/easyjson", func(t *testing.T) {
		// We need to wrap the original input string into an object
		// since easyjson only supports struct unmarshalers
		in := []byte(`{"data":` + string(in) + `}`)
		v := &easyjsongen.StringArray{}
		require.NoError(t, easyjson.Unmarshal(in, v))
		require.Equal(t, expect(), v.Data)
	})

	t.Run("genr/ffjson", func(t *testing.T) {
		// We need to wrap the original input string into an object
		// since ffjson only supports struct unmarshalers
		in := []byte(`{"data":` + string(in) + `}`)
		var v ffjsongen.StringArray
		require.NoError(t, ffjson.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v.Data)
	})

	t.Run("hand/fastjson", func(t *testing.T) {
		v, err := bench.FastjsonArrayString([]byte(in))
		require.NoError(t, err)
		require.Equal(t, expect(), v)
	})

	t.Run("hand/gjson", func(t *testing.T) {
		v, err := bench.GJSONArrayString([]byte(in))
		require.NoError(t, err)
		require.Equal(t, expect(), v)
	})

	t.Run("hand/jscan", func(t *testing.T) {
		tokenizer := jscan.NewTokenizer[[]byte](2048, 2048*1024)
		v, err := bench.JscanArrayString(tokenizer, []byte(in))
		require.NoError(t, err)
		require.Equal(t, expect(), v)
	})
}

func BenchmarkDecodeArrayString(b *testing.B) {
	in, err := os.ReadFile("../testdata/array_string_15k.json")
	if err != nil {
		b.Fatal(err)
	}

	b.Run("unmr/encoding_json", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v []string
			if err := json.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jsoniter", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v []string
			if err := jsoniter.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/goccy", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v []string
			if err := goccy.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jsonv2", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v []string
			if err := jsonv2.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/segmentio", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v []string
			if err := segmentio.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jscan", func(b *testing.B) {
		tokenizer := jscan.NewTokenizer[[]byte](2048, 2048*1024)
		d, err := jscandec.NewDecoder[[]byte, []string](
			tokenizer, jscandec.DefaultInitOptions,
		)
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			var v []string
			if err := d.Decode(in, &v, jscandec.DefaultOptions); err.IsErr() {
				b.Fatal(err)
			}
			runtime.KeepAlive(v)
		}
	})

	b.Run("unmr/jscan_unmarshal", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v []string
			if err := jscandec.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("genr/easyjson", func(b *testing.B) {
		// We need to wrap the original input string into an object
		// since easyjson only supports struct unmarshalers
		in := []byte(`{"data":` + string(in) + `}`)
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			var v easyjsongen.StringArray
			if err := easyjson.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("genr/ffjson", func(b *testing.B) {
		// We need to wrap the original input string into an object
		// since ffjson only supports struct unmarshalers
		in := []byte(`{"data":` + string(in) + `}`)
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			var v ffjsongen.StringArray
			if err := ffjson.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("hand/fastjson", func(b *testing.B) {
		var v []string
		var err error
		for n := 0; n < b.N; n++ {
			if v, err = bench.FastjsonArrayString(in); err != nil {
				b.Fatal(err)
			}
		}
		runtime.KeepAlive(v)
	})

	b.Run("hand/gjson", func(b *testing.B) {
		var v []string
		var err error
		for n := 0; n < b.N; n++ {
			if v, err = bench.GJSONArrayString(in); err != nil {
				b.Fatal(err)
			}
		}
		runtime.KeepAlive(v)
	})

	b.Run("hand/jscan", func(b *testing.B) {
		tokenizer := jscan.NewTokenizer[[]byte](2048, 2048*1024)
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			var v []string
			var err error
			if v, err = bench.JscanArrayString(tokenizer, in); err != nil {
				b.Fatal(err)
			}
			runtime.KeepAlive(v)
		}
	})
}
