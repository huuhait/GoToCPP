//go:build !unix

package wintest

// FreeBytesOf says the free space is unknown here.
func FreeBytesOf(string) (int64, error) { return -1, nil }
