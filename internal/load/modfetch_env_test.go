package load

import "os"

// The loader downloads a required module that is neither replaced nor
// vendored (internal/modfetch). The tests of this package never touch the
// network or a module cache of the developer's: no cache folder, offline.
func init() {
	os.Setenv("GOCPP_MODCACHE", "off")
	os.Setenv("GOCPP_OFFLINE", "1")
}
