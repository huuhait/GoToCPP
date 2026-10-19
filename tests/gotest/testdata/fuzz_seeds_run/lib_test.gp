package lib

import "testing"

func FuzzAdd(f *testing.F) {
	f.Add(1, 2)
	f.Add(-5, 5)
	f.Fuzz(func(t *testing.T, a, b int) {
		if Add(a, b) != Add(b, a) {
			t.Fatal("not commutative")
		}
	})
}

func fuzzRev(t *testing.T, s string, n byte) {
	if Rev(Rev(s)) != s {
		t.Errorf("Rev(Rev(%q)) != %q", s, s)
	}
	t.Log("n =", n)
}

func FuzzP(f *testing.F) {
	f.Add("hello", byte(3))
	f.Fuzz(fuzzRev)
}

func FuzzF(f *testing.F) {
	f.Add([]byte("a"), 10)
	f.Fuzz(func(t *testing.T, b []byte, n int) {
		if n > 5 {
			t.Fatalf("big %d %q", n, b)
		}
	})
}

func FuzzMismatch(f *testing.F) {
	f.Add("x")
	f.Fuzz(func(t *testing.T, n int) {})
}

func TestPlain(t *testing.T) {}
