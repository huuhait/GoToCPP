package lib_test

import (
	"testing"

	"example.com/m"
)

func TestExternal(t *testing.T) {
	t.Log(lib.Add(1, 1))
}
