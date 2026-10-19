//go:build !windows

package fsretry

// transient is false here: a failed rename of another system is final.
func transient(error) bool { return false }
