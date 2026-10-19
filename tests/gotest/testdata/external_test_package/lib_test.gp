package lib_test

import (
	"testing"

	"example.com/m"
)

func TestExternal(t *testing.T) {
	if lib.Add(2, 3) != 5 {
		t.Fatal("bad")
	}
}
