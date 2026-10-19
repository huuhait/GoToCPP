//go:build !unix && !windows

package toolchain

import "context"

// lock does nothing here: gocpp downloads the toolchain only on Linux and Windows hosts.
func lock(context.Context, string) (func(), error) {
	return func() {}, nil
}
