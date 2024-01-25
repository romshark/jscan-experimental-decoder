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
		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			d = NewDecoder[[]byte, [][]bool](tok)
		}
		runtime.KeepAlive(d)
	})
}
