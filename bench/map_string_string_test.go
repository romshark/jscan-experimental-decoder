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

func TestImplementationsMapStringString(t *testing.T) {
	in := `{"foo":"bar", "1234":"","ъ":"ツ"}`
	expect := func() map[string]string {
		return map[string]string{"foo": "bar", "1234": "", "ъ": "ツ"}
	}

	t.Run("unmr/encoding_json", func(t *testing.T) {
		var v map[string]string
		require.NoError(t, json.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/jsoniter", func(t *testing.T) {
		var v map[string]string
		require.NoError(t, jsoniter.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/goccy", func(t *testing.T) {
		var v map[string]string
		require.NoError(t, goccy.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/jsonv2", func(t *testing.T) {
		var v map[string]string
		require.NoError(t, jsonv2.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/segmentio", func(t *testing.T) {
		var v map[string]string
		require.NoError(t, segmentio.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/jscan", func(t *testing.T) {
		d, err := jscandec.NewDecoder[[]byte, map[string]string](
			jscan.NewTokenizer[[]byte](2048, 2048*1024), jscandec.DefaultInitOptions,
		)
		require.NoError(t, err)
		var v map[string]string
		if err := d.Decode([]byte(in), &v, jscandec.DefaultOptions); err.IsErr() {
			t.Fatal(err)
		}
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/jscan_unmarshal", func(t *testing.T) {
		var v map[string]string
		if err := jscandec.Unmarshal([]byte(in), &v); err != nil {
			t.Fatal(err)
		}
		require.Equal(t, expect(), v)
	})

	t.Run("genr/easyjson", func(t *testing.T) {
		// We need to wrap the original input string into an object
		// since easyjson only supports struct unmarshalers
		in := []byte(`{"data":` + string(in) + `}`)
		var v easyjsongen.MapStringString
		require.NoError(t, easyjson.Unmarshal(in, &v))
		require.Equal(t, expect(), v.Data)
	})

	t.Run("genr/ffjson", func(t *testing.T) {
		// We need to wrap the original input string into an object
		// since ffjson only supports struct unmarshalers
		in := []byte(`{"data":` + string(in) + `}`)
		var v ffjsongen.MapStringString
		require.NoError(t, ffjson.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v.Data)
	})

	t.Run("hand/gjson", func(t *testing.T) {
		v, err := bench.GJSONMapStringString([]byte(in))
		require.NoError(t, err)
		require.Equal(t, expect(), v)
	})

	t.Run("hand/jscan", func(t *testing.T) {
		tokenizer := jscan.NewTokenizer[[]byte](2048, 2048*1024)
		v, err := bench.JscanMapStringString(tokenizer, []byte(in))
		require.NoError(t, err)
		require.Equal(t, expect(), v)
	})
}

func BenchmarkDecodeMapStringString(b *testing.B) {
	in := []byte(`{"foo":"bar", "1234":""}`)

	b.Run("unmr/encoding_json", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v map[string]string
			if err := json.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jsoniter", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v map[string]string
			if err := jsoniter.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/goccy", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v map[string]string
			if err := goccy.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jsonv2", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v map[string]string
			if err := jsonv2.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/segmentio", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v map[string]string
			if err := segmentio.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jscan", func(b *testing.B) {
		tokenizer := jscan.NewTokenizer[[]byte](2048, 2048*1024)
		d, err := jscandec.NewDecoder[[]byte, map[string]string](
			tokenizer, jscandec.DefaultInitOptions,
		)
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			var v map[string]string
			if err := d.Decode(in, &v, jscandec.DefaultOptions); err.IsErr() {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jscan_unmarshal", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v map[string]string
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
			var v easyjsongen.MapStringString
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
			var v ffjsongen.MapStringString
			if err := ffjson.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("hand/gjson", func(b *testing.B) {
		var v map[string]string
		var err error
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			if v, err = bench.GJSONMapStringString(in); err != nil {
				b.Fatal(err)
			}
		}
		runtime.KeepAlive(v)
	})

	b.Run("hand/jscan", func(b *testing.B) {
		tokenizer := jscan.NewTokenizer[[]byte](2048, 2048*1024)
		var v map[string]string
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			var err error
			if v, err = bench.JscanMapStringString(tokenizer, in); err != nil {
				b.Fatal(err)
			}
		}
		runtime.KeepAlive(v)
	})
}
