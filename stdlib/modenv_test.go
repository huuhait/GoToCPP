package stdlib_test

import "os"

// The tests never download a module or use a real module cache: a fixture
// with a require line that is not replaced or vendored would otherwise go to
// the network (internal/modfetch, docs/spec/modules.md).
func init() {
	os.Setenv("GOCPP_MODCACHE", "off")
	os.Setenv("GOCPP_OFFLINE", "1")
}
