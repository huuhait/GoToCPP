package load

// MatchFile reports whether the .gp file name with the contents src is in a
// package for the target goos/goarch and the extra build tags, by the file
// name suffixes (_linux.gp, _amd64.gp) and the //go:build lines: the rule
// that Load applies to the files of a folder. For a broken build constraint
// it returns false and the message ("multiple //go:build comments"). Other
// tools that read a package folder without Load (gocpp doc) use it.
func MatchFile(goos, goarch string, tags []string, name string, src []byte) (ok bool, msg string) {
	return newMatcher(goos, goarch, tags).file(name, src)
}

// BuildTags returns the function that tells whether a build tag (linux,
// amd64, unix, gocpp, go1.20, or one of the extra tags) is satisfied for the
// target goos/goarch: the rule of a //go:build line in a .gp file, which is the
// one that Load applies. It is what an if block of a cxx recipe asks (ADR-129),
// so a condition there means what it means in a build constraint, an unknown
// tag included (it is not satisfied, as in Go). The function remembers its
// answers and is not for use by several goroutines at a time.
func BuildTags(goos, goarch string, tags []string) func(tag string) bool {
	m := newMatcher(goos, goarch, tags)
	answers := make(map[string]bool)
	return func(tag string) bool {
		if ok, seen := answers[tag]; seen {
			return ok
		}
		// A file with only this tag as its constraint is in the package if the tag holds.
		ok, _ := m.file("tag.go", []byte("//go:build "+tag+"\n\npackage p\n"))
		answers[tag] = ok
		return ok
	}
}
