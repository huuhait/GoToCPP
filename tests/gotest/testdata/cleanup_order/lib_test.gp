package lib

import (
	"testing"
)

func TestCleanup(t *testing.T) {
	t.Cleanup(func() { t.Log("cleanup 1") })
	t.Cleanup(func() { t.Log("cleanup 2") })
	t.Run("sub", func(t *testing.T) {
		t.Cleanup(func() { t.Log("sub cleanup") })
		t.Log("sub body")
	})
	t.Log("body done")
}
