//go:build unix

package wintest

import "syscall"

// FreeBytesOf returns the free bytes of the file system that holds dir (for a normal user).
func FreeBytesOf(dir string) (int64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}
