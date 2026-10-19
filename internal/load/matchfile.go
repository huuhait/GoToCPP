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
