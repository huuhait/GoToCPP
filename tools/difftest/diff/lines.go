package diff

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/huuhait/gocpp/tools/difftest/gen"
)

var progMarkRx = regexp.MustCompile(`^// ---- program (\d+) `)

// File is one file of a bundle.
type File struct {
	Name string // without extension
	Text string
}

// SplitFiles splits a bundle at its gen.FileMarker lines; a bundle without one is the single
// file "main".
func SplitFiles(src string) []File {
	if !strings.HasPrefix(src, gen.FileMarker) {
		return []File{{"main", src}}
	}
	var files []File
	for _, sec := range strings.Split(src, gen.FileMarker)[1:] {
		name, text, _ := strings.Cut(sec, "\n")
		files = append(files, File{strings.TrimSpace(name), text})
	}
	return files
}

// ProgOfLine returns the id of the program whose declarations contain the 1-based line of the
// bundle source, or -1 (the prelude, or main).
func ProgOfLine(src string, line int) int {
	cur := -1
	for i, l := range strings.Split(src, "\n") {
		if i+1 > line {
			break
		}
		if m := progMarkRx.FindStringSubmatch(l); m != nil {
			cur, _ = strconv.Atoi(m[1])
		}
		if strings.HasPrefix(l, "func main() {") {
			cur = -1
		}
	}
	return cur
}

var errLineRx = regexp.MustCompile(`([A-Za-z0-9_]+)\.g[op]:(\d+)`)

// ProgsOfErrors returns the programs that the error lines of a build output name.
func ProgsOfErrors(src, out string) []int {
	seen := map[int]bool{}
	var ids []int
	files := map[string]string{}
	for _, f := range SplitFiles(src) {
		files[f.Name] = f.Text
	}
	for _, m := range errLineRx.FindAllStringSubmatch(out, -1) {
		text, ok := files[m[1]]
		if !ok {
			continue
		}
		ln, _ := strconv.Atoi(m[2])
		if id := ProgOfLine(text, ln); id >= 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// ErrorOfProg returns the lines of a build output that name the program (and the lines that
// name no program, such as "internal error" headers).
func ErrorOfProg(src, out string, id int) string {
	files := map[string]string{}
	for _, f := range SplitFiles(src) {
		files[f.Name] = f.Text
	}
	var keep []string
	for _, l := range strings.Split(out, "\n") {
		m := errLineRx.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		text, ok := files[m[1]]
		if !ok {
			continue
		}
		ln, _ := strconv.Atoi(m[2])
		if ProgOfLine(text, ln) == id {
			keep = append(keep, l)
		}
	}
	if len(keep) == 0 {
		return out
	}
	return strings.Join(keep, "\n")
}

var (
	numRx = regexp.MustCompile(`[0-9]+`)
	posRx = regexp.MustCompile(`main\.g[op]:\d+(:\d+)?:?`)
	pfxRx = regexp.MustCompile(`\bp\d+_[A-Za-z0-9_]*`)
)

// FirstError returns the first error line of a build output, with positions and generated
// names masked so that equal errors of different programs are equal.
func FirstError(out string) string {
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		return MaskSig(l)
	}
	return ""
}

// MaskSig masks the numbers, positions and generated names in a signature text.
func MaskSig(s string) string {
	s = posRx.ReplaceAllString(s, "main:")
	s = pfxRx.ReplaceAllString(s, "pN_")
	s = numRx.ReplaceAllString(s, "N")
	return clip(s, 300)
}

var iceErrRx = regexp.MustCompile(`(?:fatal )?error: .*`)

// iceSig is the signature of generated C++ that did not compile: the compiler's first error
// (the header line "generated C++ did not compile with the flags below" is the same for all).
func iceSig(out string) string {
	if m := iceErrRx.FindString(out); m != "" {
		return MaskSig(m)
	}
	return FirstError(out)
}
