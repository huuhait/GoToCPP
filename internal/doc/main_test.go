package doc

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// The tests never download modules or use the real module cache.
	os.Setenv("GOCPP_MODCACHE", "off")
	os.Setenv("GOCPP_OFFLINE", "1")
	os.Exit(m.Run())
}
