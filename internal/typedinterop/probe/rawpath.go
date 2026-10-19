package probe

import "os"

// Paths that the cache watches are built with joinRaw and dirRaw, never with filepath.Join, filepath.Dir
// or filepath.Clean: those drop a ".." together with the element before it, as the text says, but the
// system takes the parent of what the element names (dir/link/.. is the parent of link's target, not dir).
// The path is stat-ed and read as it was spelled, so that the system resolves it the way the compiler's
// lookup did.

// isSep reports whether c separates the elements of a path on this system (both slashes on Windows).
func isSep(c byte) bool { return c == '/' || (os.PathSeparator == '\\' && c == '\\') }

// joinRaw returns dir and name as one path, with nothing cleaned.
func joinRaw(dir, name string) string {
	if dir == "" {
		return name
	}
	if isSep(dir[len(dir)-1]) {
		return dir + name
	}
	return dir + string(os.PathSeparator) + name
}

// dirRaw returns the folder of the path file: everything before its last element, as it is spelled.
func dirRaw(file string) string {
	i := len(file) - 1
	for i >= 0 && !isSep(file[i]) {
		i--
	}
	if i < 0 {
		return "."
	}
	if i == 0 || (i == 2 && file[1] == ':') {
		return file[:i+1] // (the root)
	}
	return file[:i]
}
