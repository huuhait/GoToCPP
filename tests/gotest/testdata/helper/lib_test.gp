package lib

import (
	"testing"
)

func check(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestHelper(t *testing.T) {
	check(t, 1, 1)
	check(t, 2, 3)
}
