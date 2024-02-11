package jscandec

import (
	"runtime"
	"testing"

	"github.com/romshark/jscan/v2"
)

func BenchmarkNewDecoder(b *testing.B) {
	b.Run("[][]bool", func(b *testing.B) {
		tok := jscan.NewTokenizer[[]byte](64, 1024)
		var d *Decoder[[]byte, [][]bool]
		var err error
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			if d, err = NewDecoder[[]byte, [][]bool](
				tok, DefaultInitOptions,
			); err != nil {
				b.Fatal(err)
			}
		}
		runtime.KeepAlive(d)
	})
}
