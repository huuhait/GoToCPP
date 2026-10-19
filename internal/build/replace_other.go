//go:build !windows

package build

// makeWritable does nothing here (see replace_windows.go).
func makeWritable(string) {}

// replaceError is err here.
func replaceError(err error, _ string) error { return err }
