package lib

import (
	"testing"
)

func TestOK(t *testing.T) {}

func TestBad(t *testing.T) {
	t.Errorf("got %d, want %d", Add(2, 2), 5)
	t.Error("second", "error", 3)
}
