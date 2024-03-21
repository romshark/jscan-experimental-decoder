package bench_test

import (
	json "encoding/json"
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

func TestImplementationsAnyInt(t *testing.T) {
	in, expect := `42`, float64(42)

	t.Run("unmr/encoding_json", func(t *testing.T) {
		var v any
		require.NoError(t, json.Unmarshal([]byte(in), &v))
		require.Equal(t, expect, v)
	})

	t.Run("unmr/jsoniter", func(t *testing.T) {
		var v any
		require.NoError(t, jsoniter.Unmarshal([]byte(in), &v))
		require.Equal(t, expect, v)
	})

	t.Run("unmr/goccy", func(t *testing.T) {
		var v any
		require.NoError(t, goccy.Unmarshal([]byte(in), &v))
		require.Equal(t, expect, v)
	})

	t.Run("unmr/jsonv2", func(t *testing.T) {
		var v any
		require.NoError(t, jsonv2.Unmarshal([]byte(in), &v))
		require.Equal(t, expect, v)
	})

	t.Run("unmr/segmentio", func(t *testing.T) {
		var v any
		require.NoError(t, segmentio.Unmarshal([]byte(in), &v))
		require.Equal(t, expect, v)
	})

	t.Run("unmr/jscan", func(t *testing.T) {
		d, err := jscandec.NewDecoder[[]byte, any](
			jscan.NewTokenizer[[]byte](2048, 2048*1024), jscandec.DefaultInitOptions,
		)
		require.NoError(t, err)
		var v any
		if _, err := d.Decode([]byte(in), &v, jscandec.DefaultOptions); err != nil {
			t.Fatal(err)
		}
		require.Equal(t, expect, v)
	})

	t.Run("unmr/jscan_unmarshal", func(t *testing.T) {
		var v any
		if err := jscandec.Unmarshal([]byte(in), &v); err != nil {
			t.Fatal(err)
		}
		require.Equal(t, expect, v)
	})

	t.Run("genr/easyjson", func(t *testing.T) {
		// We need to wrap the original input string into an object
		// since easyjson only supports struct unmarshalers
		in := []byte(`{"data":` + string(in) + `}`)
		var v easyjsongen.Any
		require.NoError(t, easyjson.Unmarshal(in, &v))
		require.Equal(t, expect, v.Data)
	})

	t.Run("genr/ffjson", func(t *testing.T) {
		// We need to wrap the original input string into an object
		// since ffjson only supports struct unmarshalers
		in := []byte(`{"data":` + string(in) + `}`)
		var v ffjsongen.Any
		require.NoError(t, ffjson.Unmarshal([]byte(in), &v))
		require.Equal(t, expect, v.Data)
	})

	t.Run("hand/gjson", func(t *testing.T) {
		v, err := bench.GJSONAny([]byte(in))
		require.NoError(t, err)
		require.Equal(t, expect, v)
	})

	t.Run("hand/jscan", func(t *testing.T) {
		tokenizer := jscan.NewTokenizer[[]byte](4, 8)
		v, err := bench.JscanAny(tokenizer, []byte(in))
		require.NoError(t, err)
		require.Equal(t, expect, v)
	})
}

func BenchmarkAnyInt(b *testing.B) {
	in := []byte(`42`)

	b.Run("unmr/encoding_json", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v any
			if err := json.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jsoniter", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v any
			if err := jsoniter.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/goccy", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v any
			if err := goccy.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jsonv2", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v any
			if err := jsonv2.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/egmentio", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v any
			if err := segmentio.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jscan", func(b *testing.B) {
		tokenizer := jscan.NewTokenizer[[]byte](2048, 2048*1024)
		d, err := jscandec.NewDecoder[[]byte, any](
			tokenizer, jscandec.DefaultInitOptions,
		)
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			var v any
			if _, err := d.Decode(in, &v, jscandec.DefaultOptions); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jscan_unmarshal", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v any
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
			var v easyjsongen.Any
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
			var v ffjsongen.Any
			if err := ffjson.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("hand/gjson", func(b *testing.B) {
		var v any
		var err error
		for n := 0; n < b.N; n++ {
			if v, err = bench.GJSONAny(in); err != nil {
				b.Fatal(err)
			}
		}
		runtime.KeepAlive(v)
	})

	b.Run("hand/jscan", func(b *testing.B) {
		tokenizer := jscan.NewTokenizer[[]byte](4, 8)
		var v any
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			var err error
			if v, err = bench.JscanAny(tokenizer, in); err != nil {
				b.Fatal(err)
			}
		}
		runtime.KeepAlive(v)
	})
}
