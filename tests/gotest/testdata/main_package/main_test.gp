package main

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 2) != 4 {
		t.Fatal("bad")
	}
}

// The package under test is built as a library: its init functions ran, and its types are named
// main.T, as in the test binary of go test.
func TestMainPackage(t *testing.T) {
	t.Logf("%s %T %v", inited, point{1, 2}, point{1, 2})
}
