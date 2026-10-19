package lib

import "testing"

var sink []byte
var ssink string

func BenchmarkAlloc(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sink = make([]byte, 1024)
	}
}

func BenchmarkNoAlloc(b *testing.B) {
	b.ReportAllocs()
	x := 0
	for i := 0; i < b.N; i++ {
		x += i
	}
	_ = x
}

func TestAllocs(t *testing.T) {
	n := testing.AllocsPerRun(100, func() { sink = make([]byte, 1024) })
	if n < 1 {
		t.Errorf("AllocsPerRun = %v, want >= 1", n)
	}
	t.Log(n)
	z := testing.AllocsPerRun(100, func() {})
	t.Log(z)
}
