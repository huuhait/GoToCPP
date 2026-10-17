package build

import (
	"os"
	"strings"
)

// EnvOff reports whether the environment variable name turns something off: it is set to
// anything but "", "0" and "false" (in any case). GOCPP_NOOBJCACHE (the object cache,
// objcache.go) and GOCPP_NOPROBECACHE (the probe results on disk, internal/interop) follow
// this one rule.
func EnvOff(name string) bool {
	v := os.Getenv(name)
	return v != "" && v != "0" && !strings.EqualFold(v, "false")
}

// NativeArch reports whether one of the parts (compiler flags, the values of environment
// variables as NAME=value) asks the compiler for the host's CPU (-march=native,
// -mtune=native, -mcpu=native). What the compiler makes then depends on the machine: a
// cache folder that two machines share (a home folder over NFS) would serve one machine the
// other's answer, so the caches of compiler results (the object cache, the probe results of
// internal/interop) don't keep it.
func NativeArch(parts ...string) bool {
	for _, s := range parts {
		if strings.Contains(s, "-march=native") || strings.Contains(s, "-mtune=native") || strings.Contains(s, "-mcpu=native") {
			return true
		}
	}
	return false
}
