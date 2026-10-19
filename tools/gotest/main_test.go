package main

import (
	"os"
	"testing"
)

// The tests never download or read the user's caches (the tool's own
// fetcher is tested with a fixture tarball).
func TestMain(m *testing.M) {
	os.Setenv("GOCPP_MODCACHE", "off")
	os.Setenv("GOCPP_OFFLINE", "1")
	os.Exit(m.Run())
}
