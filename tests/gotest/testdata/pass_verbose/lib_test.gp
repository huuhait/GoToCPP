package lib

import (
	"testing"
)

func TestA(t *testing.T) {}

func TestB(t *testing.T) {
	t.Log("hello from B")
	t.Logf("n=%d", 42)
}
