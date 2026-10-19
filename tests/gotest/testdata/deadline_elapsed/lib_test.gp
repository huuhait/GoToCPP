package lib

import (
	"testing"
	"time"
)

func TestDeadline(t *testing.T) {
	d, ok := t.Deadline()
	left := time.Until(d)
	t.Log(ok, left > 0 && left <= 30*time.Second)
}

func BenchmarkE(b *testing.B) {
	for i := 0; i < b.N; i++ {
	}
	if b.Elapsed() < 0 {
		b.Fatal("negative")
	}
	b.Log(b.Elapsed().Seconds() >= 0, b.Elapsed() <= time.Hour)
}
