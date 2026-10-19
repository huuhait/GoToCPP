package lib

import (
	"testing"
)

func BenchmarkBad(b *testing.B) {
	b.Fatal("benchmark failed")
}
