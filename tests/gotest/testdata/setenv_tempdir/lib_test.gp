package lib

import (
	"os"
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
		if got := os.Getenv("GOCPP_FIXTURE_VAR"); got != "y" {
			t.Errorf("os.Getenv after t.Setenv(y) = %q", got)
		}
	})
	if got := os.Getenv("GOCPP_FIXTURE_VAR"); got != "x" {
		t.Errorf("os.Getenv after the subtest restored x = %q", got)
	}
}
