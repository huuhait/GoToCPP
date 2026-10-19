//go:build !unix

package toolchain

import "context"

// lock does nothing here: gocpp downloads the toolchain only on Linux hosts (Windows hosts come
// with Phase 14, and have no pinned archive until then).
func lock(context.Context, string) (func(), error) {
	return func() {}, nil
}
