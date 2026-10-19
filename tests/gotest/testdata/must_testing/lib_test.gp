package lib

import "testing"

func TestOk(t *testing.T) {
	n := Parse!("12")
	t.Log("parsed", n)
}

func TestStops(t *testing.T) {
	t.Log("before")
	n := Parse!("x")
	t.Log("never printed", n)
}

func TestAfter(t *testing.T) {
	t.Log("still runs")
}

func load(t *testing.T, s string) int {
	t.Helper()
	return Parse!(s)
}

func TestHelper(t *testing.T) {
	t.Log(load(t, "7"))
	t.Log(load(t, ""))
}

func TestSub(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		t.Log(Parse!("5"))
	})
	t.Run("bad", func(t *testing.T) {
		t.Log(Parse!("5x"))
	})
	t.Log("the parent goes on")
}

func TestEmptyIsErrEmpty(t *testing.T) {
	_, err := Parse("")
	if err != ErrEmpty {
		t.Fatalf("got %v", err)
	}
}

// A closure without results inside a test uses the t of the test: its failing must call is t.Fatal.
func TestClosure(t *testing.T) {
	check := func(s string) {
		Parse!(s)
		t.Log("checked", s)
	}
	check("1")
	check("")
	t.Log("never printed")
}

// A helper that takes a testing.TB gets t.Fatal as well.
func tb(t testing.TB, s string) int {
	t.Helper()
	return Parse!(s)
}

func TestTB(t *testing.T) {
	t.Log(tb(t, "3"))
	t.Log(tb(t, "z"))
}
