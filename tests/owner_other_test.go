//go:build !unix

package tests

import "io/fs"

func ownedByMe(fs.FileInfo) bool { return true }
