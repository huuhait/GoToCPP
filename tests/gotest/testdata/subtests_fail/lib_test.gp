package lib

import (
	"testing"
)

func TestGroup(t *testing.T) {
	t.Run("good", func(t *testing.T) {})
	t.Run("bad", func(t *testing.T) {
		t.Error("broken")
	})
	t.Run("also bad", func(t *testing.T) {
		t.Run("deeper", func(t *testing.T) {
			t.Fatal("very broken")
		})
	})
}
