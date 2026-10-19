package lib

import (
	"testing"
)

func TestAdd(t *testing.T) {
	if Add(1, 2) != 3 {
		t.Fatal("bad")
	}
}

func TestAddNeg(t *testing.T) {
	if got := Add(-1, -2); got != -3 {
		t.Errorf("Add(-1,-2) = %d", got)
	}
}
