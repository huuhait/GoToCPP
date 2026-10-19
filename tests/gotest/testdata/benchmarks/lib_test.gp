package lib

import (
	"testing"
)

func BenchmarkAdd(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Add(i, i)
	}
}

func BenchmarkLoop(b *testing.B) {
	for b.Loop() {
		Add(1, 2)
	}
}

func BenchmarkSub(b *testing.B) {
	b.Run("one", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			Add(1, 1)
		}
	})
	b.Run("two", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			Add(2, 2)
		}
	})
}
