package lib

import (
	"testing"
)

func TestS(t *testing.T) {
	for _, n := range []string{"a", "b1", "b2"} {
		t.Run(n, func(t *testing.T) {
			t.Run("x", func(t *testing.T) {})
			t.Run("y", func(t *testing.T) {})
		})
	}
}
