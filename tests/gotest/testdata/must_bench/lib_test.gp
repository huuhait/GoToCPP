package lib

import "testing"

func TestPasses(t *testing.T) {
	n := Parse!("12")
	t.Log("parsed", n)
}

func BenchmarkParse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Parse!("1")
	}
}

// A benchmark with a failing must call is b.Fatal.
func BenchmarkBad(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Parse!("z")
	}
}

func BenchmarkAfter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Parse!("2")
	}
}
