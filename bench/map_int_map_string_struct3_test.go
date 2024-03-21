package bench_test

import (
	json "encoding/json"
	"testing"

	jscandec "github.com/romshark/jscan-experimental-decoder"
	"github.com/romshark/jscan-experimental-decoder/bench"
	"github.com/romshark/jscan-experimental-decoder/bench/easyjsongen"

	jsonv2 "github.com/go-json-experiment/json"
	goccy "github.com/goccy/go-json"
	jsoniter "github.com/json-iterator/go"
	easyjson "github.com/mailru/easyjson"
	jscan "github.com/romshark/jscan/v2"
	segmentio "github.com/segmentio/encoding/json"
	"github.com/stretchr/testify/require"
)

const testMapIntMapStringStruct3 = `{
	"10001": {
		"first": {
			"name":   "First Struct3 instance",
			"number": 1001,
			"tags":   []
		}
	},
	"10002": {
		"second": {},
		"third": {
			"name":   "Third Struct3 instance",
			"number": 1003,
			"tags":   ["JSON", "is", "awesome"]
		},
		"fourth": null,
		"fifth": {
			"name":   "Fifth Struct3 instance",
			"number": 1005,
			"tags":   ["Go", "is", "awesome"]
		}
	}
}`

func TestImplementationsMapIntMapStringStruct3(t *testing.T) {
	in := testMapIntMapStringStruct3
	expect := func() map[int]map[string]bench.Struct3 {
		return map[int]map[string]bench.Struct3{
			10001: {
				"first": {
					Name:   "First Struct3 instance",
					Number: 1001,
					Tags:   []string{},
				},
			},
			10002: {
				"second": {},
				"third": {
					Name:   "Third Struct3 instance",
					Number: 1003,
					Tags:   []string{"JSON", "is", "awesome"},
				},
				"fourth": {},
				"fifth": {
					Name:   "Fifth Struct3 instance",
					Number: 1005,
					Tags:   []string{"Go", "is", "awesome"},
				},
			},
		}
	}
	expectEasyjson := func() map[int]map[string]easyjsongen.Struct3 {
		return map[int]map[string]easyjsongen.Struct3{
			10001: {
				"first": {
					Name:   "First Struct3 instance",
					Number: 1001,
					Tags:   []string{},
				},
			},
			10002: {
				"second": {},
				"third": {
					Name:   "Third Struct3 instance",
					Number: 1003,
					Tags:   []string{"JSON", "is", "awesome"},
				},
				"fourth": {},
				"fifth": {
					Name:   "Fifth Struct3 instance",
					Number: 1005,
					Tags:   []string{"Go", "is", "awesome"},
				},
			},
		}
	}
	// expectFFjson := func() map[int]map[string]ffjsongen.Struct3 {
	// 	return map[int]map[string]ffjsongen.Struct3{
	// 		10001: {
	// 			"first": {
	// 				Name:   "First Struct3 instance",
	// 				Number: 1001,
	// 				Tags:   []string{},
	// 			},
	// 		},
	// 		10002: {
	// 			"second": {},
	// 			"third": {
	// 				Name:   "Third Struct3 instance",
	// 				Number: 1003,
	// 				Tags:   []string{"JSON", "is", "awesome"},
	// 			},
	// 			"fourth": {},
	// 			"fifth": {
	// 				Name:   "Fifth Struct3 instance",
	// 				Number: 1005,
	// 				Tags:   []string{"Go", "is", "awesome"},
	// 			},
	// 		},
	// 	}
	// }

	t.Run("unmr/encoding_json", func(t *testing.T) {
		var v map[int]map[string]bench.Struct3
		require.NoError(t, json.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/jsoniter", func(t *testing.T) {
		var v map[int]map[string]bench.Struct3
		require.NoError(t, jsoniter.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/goccy", func(t *testing.T) {
		var v map[int]map[string]bench.Struct3
		require.NoError(t, goccy.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/jsonv2", func(t *testing.T) {
		var v map[int]map[string]bench.Struct3
		require.NoError(t, jsonv2.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/segmentio", func(t *testing.T) {
		var v map[int]map[string]bench.Struct3
		require.NoError(t, segmentio.Unmarshal([]byte(in), &v))
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/jscan", func(t *testing.T) {
		d, err := jscandec.NewDecoder[[]byte, map[int]map[string]bench.Struct3](
			jscan.NewTokenizer[[]byte](2048, 2048*1024), jscandec.DefaultInitOptions,
		)
		require.NoError(t, err)
		var v map[int]map[string]bench.Struct3
		if _, err := d.Decode([]byte(in), &v, jscandec.DefaultOptions); err != nil {
			t.Fatal(err)
		}
		require.Equal(t, expect(), v)
	})

	t.Run("unmr/jscan_unmarshal", func(t *testing.T) {
		var v map[int]map[string]bench.Struct3
		if err := jscandec.Unmarshal([]byte(in), &v); err != nil {
			t.Fatal(err)
		}
		require.Equal(t, expect(), v)
	})

	t.Run("genr/easyjson", func(t *testing.T) {
		// We need to wrap the original input string into an object
		// since easyjson only supports struct unmarshalers
		in := []byte(`{"data":` + string(in) + `}`)
		var v easyjsongen.MapIntMapStringStruct3
		require.NoError(t, easyjson.Unmarshal(in, &v))
		require.Equal(t, expectEasyjson(), v.Data)
	})

	// ffjson seems to not support unmarshaling map keys of type int.
	//
	// t.Run("genr/ffjson", func(t *testing.T) {
	// 	// We need to wrap the original input string into an object
	// 	// since ffjson only supports struct unmarshalers
	// 	in := []byte(`{"data":` + string(in) + `}`)
	// 	var v ffjsongen.MapIntMapStringStruct3
	// 	require.NoError(t, ffjson.Unmarshal([]byte(in), &v))
	// 	require.Equal(t, expectFFjson(), v.Data)
	// })
}

func BenchmarkDecodeMapIntMapStringStruct3(b *testing.B) {
	in := []byte(testMapIntMapStringStruct3)

	b.Run("unmr/encoding_json", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v map[int]map[string]bench.Struct3
			if err := json.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jsoniter", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v map[int]map[string]bench.Struct3
			if err := jsoniter.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/goccy", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v map[int]map[string]bench.Struct3
			if err := goccy.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jsonv2", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v map[int]map[string]bench.Struct3
			if err := jsonv2.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/segmentio", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v map[int]map[string]bench.Struct3
			if err := segmentio.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jscan", func(b *testing.B) {
		tokenizer := jscan.NewTokenizer[[]byte](2048, 2048*1024)
		d, err := jscandec.NewDecoder[[]byte, map[int]map[string]bench.Struct3](
			tokenizer, jscandec.DefaultInitOptions,
		)
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			var v map[int]map[string]bench.Struct3
			if _, err := d.Decode(in, &v, jscandec.DefaultOptions); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("unmr/jscan_unmarshal", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			var v map[int]map[string]bench.Struct3
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
			var v easyjsongen.MapIntMapStringStruct3
			if err := easyjson.Unmarshal(in, &v); err != nil {
				b.Fatal(err)
			}
		}
	})

	// b.Run("genr/ffjson", func(b *testing.B) {
	// 	// We need to wrap the original input string into an object
	// 	// since ffjson only supports struct unmarshalers
	// 	in := []byte(`{"data":` + string(in) + `}`)
	// 	b.ResetTimer()
	// 	for n := 0; n < b.N; n++ {
	// 		var v ffjsongen.MapIntMapStringStruct3
	// 		if err := ffjson.Unmarshal(in, &v); err != nil {
	// 			b.Fatal(err)
	// 		}
	// 	}
	// })
}
