package lib

import "testing"

func TestPasses(t *testing.T) {
	n, err := Parse("12"); if err != nil { t.Fatal(err) }
	t.Log("parsed", n)
}

func BenchmarkParse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := Parse("1"); if err != nil { b.Fatal(err) }
	}
}

// A benchmark with a failing must call is b.Fatal.
func BenchmarkBad(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := Parse("z"); if err != nil { b.Fatal(err) }
	}
}

func BenchmarkAfter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := Parse("2"); if err != nil { b.Fatal(err) }
	}
}
