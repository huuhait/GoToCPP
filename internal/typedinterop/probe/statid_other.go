//go:build !linux && !darwin

package probe

import "os"

// changeTime: not available here (see statid_linux.go); the check by size and modification
// time is then only used for files that were not modified within two seconds.
func changeTime(os.FileInfo) (int64, bool) { return 0, false }
