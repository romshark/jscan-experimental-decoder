package bench_test

import (
	json "encoding/json"
	"runtime"
	"testing"

	jscandec "github.com/romshark/jscan-experimental-decoder"
	"github.com/romshark/jscan-experimental-decoder/bench"
	"github.com/romshark/jscan-experimental-decoder/bench/easyjsongen"
	"github.com/romshark/jscan-experimental-decoder/bench/ffjsongen"
	segmentio "github.com/segmentio/encoding/json"

	jsonv2 "github.com/go-json-experiment/json"
	goccy "github.com/goccy/go-json"
	jsoniter "github.com/json-iterator/go"
	easyjson "github.com/mailru/easyjson"
	ffjson "github.com/pquerna/ffjson/ffjson"
	jscan "github.com/romshark/jscan/v2"
	"github.com/stretchr/testify/require"
)

func TestImplementationsStruct3(t *testing.T) {
	in := `{
		"name":"Test name",
		"number": 100553,
		"tags": ["sports", "portable"]
	}`
	expect := func() bench.Struct3 {
		return bench.Struct3{
			Name:   "Test name",
			Number: 100553,
			Tags:   []string{"sports", "portable"},
		}
	}

	t.Run("unmr/encoding_json", func(t *testing.T) {
		var v bench.Struct3
		require.NoError(t, json.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/jsoniter", func(t *testing.T) {
		var v bench.Struct3
		require.NoError(t, jsoniter.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/goccy", func(t *testing.T) {
		var v bench.Struct3
		require.NoError(t, goccy.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/jsonv2", func(t *testing.T) {
		var v bench.Struct3
		require.NoError(t, jsonv2.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/segmentio", func(t *testing.T) {
		var v bench.Struct3
		require.NoError(t, segmentio.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/jscan", func(t *testing.T) {
		d, err := jscandec.NewDecoder[[]byte, bench.Struct3](
			jscan.NewTokenizer[[]byte](32, len(in)/2), jscandec.DefaultInitOptions,
		)
		require.NoError(t, err)
		var v bench.Struct3
		if err := d.Decode([]byte(in), &v, jscandec.DefaultOptions); err.IsErr() {
			t.Fatal(err)
		}
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/jscan_unmarshal", func(t *testing.T) {
		var v bench.Struct3
		if err := jscandec.Unmarshal([]byte(in), &v); err != nil {
			t.Fatal(err)
		}
		require.Equal(t, expect(), v)
	})

	t.Run("genr/easyjson", func(t *testing.T) {
		var v easyjsongen.Struct3
		require.NoError(t, easyjson.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), bench.Struct3(v))
	})

	t.Run("genr/ffjson", func(t *testing.T) {
		var v ffjsongen.Struct3
		require.NoError(t, ffjson.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), bench.Struct3(v))
	})

	t.Run("hand/gjson", func(t *testing.T) {
		v, err := bench.GJSONStruct3([]byte(in))
		require.NoError(t, err)
		require.Equal(t, expect(), v)
	})

	t.Run("hand/fastjson", func(t *testing.T) {
		v, err := bench.FastjsonStruct3([]byte(in))
		require.NoError(t, err)
		require.Equal(t, expect(), v)
	})

	t.Run("hand/jscan", func(t *testing.T) {
		tokenizer := jscan.NewTokenizer[[]byte](8, len(in)/2)
		v, err := bench.JscanStruct3(tokenizer, []byte(in))
		require.NoError(t, err)
		require.Equal(t, expect(), v)
	})
}

func BenchmarkDecodeStruct3(b *testing.B) {
	in := []byte(`{
		"name":"Test name",
		"number": 100553,
		"tags": ["sports", "portable"]
	}`)

	b.Run("unmr/encoding_json", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v bench.Struct3
			if err := json.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jsoniter", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v bench.Struct3
			if err := jsoniter.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/goccy", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v bench.Struct3
			if err := goccy.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jsonv2", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v bench.Struct3
			if err := jsonv2.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/segmentio", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v bench.Struct3
			if err := segmentio.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jscan", func(b *testing.B) {
		tokenizer := jscan.NewTokenizer[[]byte](32, 128)
		d, err := jscandec.NewDecoder[[]byte, bench.Struct3](
			tokenizer, jscandec.DefaultInitOptions,
		)
		if err != nil {
			b.Fatal(err)
		}
		var v bench.Struct3
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			if err := d.Decode(in, &v, jscandec.DefaultOptions); err.IsErr() {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jscan_unmarshal", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v bench.Struct3
			if err := jscandec.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("genr/easyjson", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v easyjsongen.Struct3
			if err := easyjson.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("genr/ffjson", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v ffjsongen.Struct3
			if err := ffjson.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("hand/gjson", func(b *testing.B) {
		var v bench.Struct3
		var err error
		for n := 0; n < b.N; n++ {
			if v, err = bench.GJSONStruct3(in); err != nil {
				b.Fatal(err)
			}
		}
		runtime.KeepAlive(v)
	})

	b.Run("hand/fastjson", func(b *testing.B) {
		var v bench.Struct3
		var err error
		for n := 0; n < b.N; n++ {
			if v, err = bench.FastjsonStruct3(in); err != nil {
				b.Fatal(err)
			}
		}
		runtime.KeepAlive(v)
	})

	b.Run("hand/jscan", func(b *testing.B) {
		tokenizer := jscan.NewTokenizer[[]byte](8, len(in)/2)
		var v bench.Struct3
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			var err error
			if v, err = bench.JscanStruct3(tokenizer, in); err != nil {
				b.Fatal(err)
			}
		}
		runtime.KeepAlive(v)
	})
}
