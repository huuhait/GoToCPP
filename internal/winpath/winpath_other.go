//go:build !windows

package winpath

func acpSafe(string) bool { return true }

func short(string) (string, bool) { return "", false }
