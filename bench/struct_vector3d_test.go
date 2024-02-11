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

func TestImplementationsStructVector3D(t *testing.T) {
	in := `{"X":0.0052265971,"Y":12.6644301,"Z":10}`
	expect := func() bench.StructVector3D {
		return bench.StructVector3D{
			X: 0.0052265971,
			Y: 12.6644301,
			Z: 10,
		}
	}

	t.Run("std", func(t *testing.T) {
		var v bench.StructVector3D
		require.NoError(t, json.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("jsoniter", func(t *testing.T) {
		var v bench.StructVector3D
		require.NoError(t, jsoniter.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("goccy", func(t *testing.T) {
		var v bench.StructVector3D
		require.NoError(t, goccy.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("easyjson", func(t *testing.T) {
		var v easyjsongen.StructVector3D
		require.NoError(t, easyjson.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), bench.StructVector3D(v))
	})

	t.Run("ffjson", func(t *testing.T) {
		var v ffjsongen.StructVector3D
		require.NoError(t, ffjson.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), bench.StructVector3D(v))
	})

	t.Run("gjson", func(t *testing.T) {
		v, err := bench.GJSONStructVector3D([]byte(in))
		require.NoError(t, err)
		require.Equal(t, expect(), v)
	})

	t.Run("fastjson", func(t *testing.T) {
		v, err := bench.FastjsonStructVector3D([]byte(in))
		require.NoError(t, err)
		require.Equal(t, expect(), v)
	})

	t.Run("jsonv2", func(t *testing.T) {
		var v bench.StructVector3D
		require.NoError(t, jsonv2.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("segmentio", func(t *testing.T) {
		var v bench.StructVector3D
		require.NoError(t, segmentio.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("jscan/decoder", func(t *testing.T) {
		d, err := jscandec.NewDecoder[[]byte, bench.StructVector3D](
			jscan.NewTokenizer[[]byte](32, len(in)/2), jscandec.DefaultInitOptions,
		)
		require.NoError(t, err)
		var v bench.StructVector3D
		if err := d.Decode([]byte(in), &v, jscandec.DefaultOptions); err.IsErr() {
			t.Fatal(err)
		}
		require.Equal(t, expect(), v)
	})

	t.Run("jscan/unmarshal", func(t *testing.T) {
		var v bench.StructVector3D
		if err := jscandec.Unmarshal([]byte(in), &v); err != nil {
			t.Fatal(err)
		}
		require.Equal(t, expect(), v)
	})

	t.Run("jscan/handwritten", func(t *testing.T) {
		tokenizer := jscan.NewTokenizer[[]byte](8, len(in)/2)
		v, err := bench.JscanStructVector3D(tokenizer, []byte(in))
		require.NoError(t, err)
		require.Equal(t, expect(), v)
	})
}

func BenchmarkDecodeStructVector3D(b *testing.B) {
	in := []byte(`{"x":0.0052265971,"y":12.6644301,"z":10}`)

	b.Run("std", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v bench.StructVector3D
			if err := json.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("jsoniter", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v bench.StructVector3D
			if err := jsoniter.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("goccy", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v bench.StructVector3D
			if err := goccy.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("easyjson", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v easyjsongen.StructVector3D
			if err := easyjson.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("ffjson", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v ffjsongen.StructVector3D
			if err := ffjson.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("gjson", func(b *testing.B) {
		var v bench.StructVector3D
		var err error
		for n := 0; n < b.N; n++ {
			if v, err = bench.GJSONStructVector3D(in); err != nil {
				b.Fatal(err)
			}
		}
		runtime.KeepAlive(v)
	})

	b.Run("fastjson", func(b *testing.B) {
		var v bench.StructVector3D
		var err error
		for n := 0; n < b.N; n++ {
			if v, err = bench.FastjsonStructVector3D(in); err != nil {
				b.Fatal(err)
			}
		}
		runtime.KeepAlive(v)
	})

	b.Run("jsonv2", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v bench.StructVector3D
			if err := jsonv2.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("segmentio", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v bench.StructVector3D
			if err := segmentio.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("jscan/decoder", func(b *testing.B) {
		tokenizer := jscan.NewTokenizer[[]byte](32, 128)
		d, err := jscandec.NewDecoder[[]byte, bench.StructVector3D](
			tokenizer, jscandec.DefaultInitOptions,
		)
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			var v bench.StructVector3D
			if err := d.Decode(in, &v, jscandec.DefaultOptions); err.IsErr() {
				b.Fatal(err)
			}
		}
	})

	b.Run("jscan/unmarshal", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v bench.StructVector3D
			if err := jscandec.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("jscan/handwritten", func(b *testing.B) {
		tokenizer := jscan.NewTokenizer[[]byte](8, len(in)/2)
		var v bench.StructVector3D
		var err error
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			if v, err = bench.JscanStructVector3D(tokenizer, in); err != nil {
				b.Fatal(err)
			}
		}
		runtime.KeepAlive(v)
	})
}
