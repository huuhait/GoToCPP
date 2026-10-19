package lib

import (
	"testing"
)

func TestSetenvTempDir(t *testing.T) {
	t.Setenv("GOCPP_FIXTURE_VAR", "x")
	a, b := t.TempDir(), t.TempDir()
	if a == "" || a == b {
		t.Errorf("bad temp dirs %q %q", a, b)
	}
	t.Run("sub", func(t *testing.T) {
		t.Setenv("GOCPP_FIXTURE_VAR", "y")
		t.Log(t.TempDir() != "")
	})
}
