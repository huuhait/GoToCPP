//go:build !unix

package server

import "os"

// ownedByUs relies on the per-user temporary folder and our marker on non-Unix platforms; it does not inspect ACLs.
func ownedByUs(os.FileInfo) bool { return true }

// pidAlive reports whether a process with the id pid exists: not known here, so the age of the marker decides.
func pidAlive(int) bool { return false }
