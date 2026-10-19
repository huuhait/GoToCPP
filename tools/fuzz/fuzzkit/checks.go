package fuzzkit

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/internal/cppgen"
)

var lineRx = regexp.MustCompile(`(?m)^[ \t]*#[ \t]*line[ \t]+(\S+)(?:[ \t]+("(?:[^"\\\n]|\\.)*"))?[ \t]*$`)

// CheckLines checks the #line directives of the generated files of the program src: each has a
// line number from 1 on and a file that is the program's source (the line number must be in it:
// at most one past its last line), a generated file of the program (same rule), or a file of the
// standard library ($GOCPP/...). A debugger and the compiler's messages go by them.
func CheckLines(src []byte, files []cppgen.File) error {
	lines := func(b []byte) int { return bytes.Count(b, []byte("\n")) + 1 }
	gen := map[string]int{}
	for _, f := range files {
		gen[GenRoot+"/"+f.Path] = lines(f.Data)
	}
	for _, f := range files {
		for _, m := range lineRx.FindAllSubmatch(f.Data, -1) {
			n, err := strconv.Atoi(string(m[1]))
			if err != nil || n < 1 {
				return fmt.Errorf("%s: bad line number in %q", f.Path, m[0])
			}
			if len(m[2]) == 0 {
				continue
			}
			name, err := strconv.Unquote(string(m[2]))
			if err != nil {
				return fmt.Errorf("%s: bad file name in %q", f.Path, m[0])
			}
			switch {
			case name == File:
				if n > lines(src)+1 {
					return fmt.Errorf("%s: %q is past the end of the source (%d lines)", f.Path, m[0], lines(src))
				}
			case strings.HasPrefix(name, "$GOCPP/"):
			case gen[name] > 0:
				if n > gen[name]+1 {
					return fmt.Errorf("%s: %q is past the end of %s (%d lines)", f.Path, m[0], path.Base(name), gen[name])
				}
			default:
				return fmt.Errorf("%s: %q names a file that is not part of the program", f.Path, m[0])
			}
		}
	}
	return nil
}

// SameFiles reports a difference between two emits of the same program, which must be equal
// byte for byte (the object cache and reproducible builds rely on it).
func SameFiles(a, b []cppgen.File) error {
	if len(a) != len(b) {
		return fmt.Errorf("%d files, then %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Path != b[i].Path {
			return fmt.Errorf("file %d is %s, then %s", i, a[i].Path, b[i].Path)
		}
		if !bytes.Equal(a[i].Data, b[i].Data) {
			return fmt.Errorf("%s differs between two emits of the same program", a[i].Path)
		}
	}
	return nil
}
