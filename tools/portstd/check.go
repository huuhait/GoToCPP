package main

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// The check mode (make portstd-check): the committed stdlib/<pkg> is what the
// tool writes now. It catches a hand edit of a ported file (the change must
// go into a patch rule), a ported file that the tool does not write any more,
// a PORT.md or an index line that is out of date.

// firstDiff describes where two texts first differ.
func firstDiff(want, have []byte) string {
	wl := strings.Split(string(want), "\n")
	hl := strings.Split(string(have), "\n")
	for i := 0; i < max(len(wl), len(hl)); i++ {
		var w, h string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(hl) {
			h = hl[i]
		}
		if i >= len(wl) || i >= len(hl) || w != h {
			return fmt.Sprintf("line %d: the file has %q, the port writes %q", i+1, clip(h), clip(w))
		}
	}
	return "the texts differ"
}

func clip(s string) string {
	if len(s) > 80 {
		return s[:77] + "..."
	}
	return s
}

// checkPort returns what is wrong with the committed files of the port r
// under out, as one message each.
func checkPort(out string, r *Result) []string {
	dir := pkgDir(out, r.Pkg)
	rel := func(name string) string {
		return filepath.ToSlash(filepath.Join(filepath.Base(out), filepath.FromSlash(r.Pkg), name))
	}
	var probs []string
	want := make(map[string][]byte)
	for _, f := range r.Files {
		want[f.Name] = f.Content
	}
	want["PORT.md"] = []byte(portMD(r))
	for _, name := range sortedBytesKeys(want) {
		have, err := os.ReadFile(filepath.Join(dir, name))
		switch {
		case os.IsNotExist(err):
			probs = append(probs, fmt.Sprintf("%s: missing (go run ./tools/portstd %s writes it)", rel(name), r.Pkg))
		case err != nil:
			probs = append(probs, fmt.Sprintf("%s: %v", rel(name), err))
		case !bytes.Equal(have, want[name]):
			probs = append(probs, fmt.Sprintf("%s: is not what the port writes (a hand edit? put a rule in tools/portstd/patches/%s); %s",
				rel(name), patchFile(r.Pkg), firstDiff(want[name], have)))
		}
	}
	have, err := portedFiles(dir)
	if err != nil {
		return append(probs, err.Error())
	}
	for _, name := range have {
		if _, ok := want[name]; !ok {
			probs = append(probs, fmt.Sprintf("%s: has the header of a ported file, but the port does not write it any more (remove it, or fix the patches)", rel(name)))
		}
	}
	return append(probs, stubProblems(out, r.Pkg)...)
}

// stubProblems reports the stubs of tools/stubgen beside a port: they declare what the port
// declares, and the build fails with "redeclared".
func stubProblems(out, pkg string) []string {
	stubs, _, err := foreignFiles(pkgDir(out, pkg))
	if err != nil {
		return []string{err.Error()}
	}
	var probs []string
	for _, s := range stubs {
		probs = append(probs, fmt.Sprintf("%s: is a stub of tools/stubgen beside the port of %s (remove it; stubgen's Real: true, see docs/guides/porting.md)",
			filepath.ToSlash(filepath.Join(filepath.Base(out), filepath.FromSlash(pkg), s)), pkg))
	}
	return probs
}

// checkSums is the check without Go's source: the files of every port that PORTED.txt lists
// have the SHA-256 that PORTED.sha256 records, and no file of a port is missing from it. It
// catches a hand edit of a ported file; it cannot tell that the tool would write something
// else (the cache is needed for that), and an edit of PORTED.sha256 together with the file hides it.
func checkSums(out string) (n int, probs []string) {
	index, err := readIndex(out)
	if err != nil {
		return 0, []string{err.Error()}
	}
	sums, err := readSums(out)
	if err != nil {
		return 0, []string{err.Error()}
	}
	by := sumsByPkg(sums)
	name := filepath.ToSlash(filepath.Join(filepath.Base(out), sumsName))
	for _, pkg := range sortedMapKeys(index) {
		files := by[pkg]
		if len(files) == 0 {
			probs = append(probs, fmt.Sprintf("%s: no hashes for %s (go run ./tools/portstd %s writes them)", name, pkg, pkg))
			continue
		}
		n++
		dir := pkgDir(out, pkg)
		rel := func(f string) string {
			return filepath.ToSlash(filepath.Join(filepath.Base(out), filepath.FromSlash(pkg), f))
		}
		for _, f := range sortedMapKeys(files) {
			b, err := os.ReadFile(filepath.Join(dir, f))
			switch {
			case os.IsNotExist(err):
				probs = append(probs, fmt.Sprintf("%s: missing (go run ./tools/portstd %s writes it)", rel(f), pkg))
			case err != nil:
				probs = append(probs, fmt.Sprintf("%s: %v", rel(f), err))
			case hashOf(b) != files[f]:
				probs = append(probs, fmt.Sprintf("%s: SHA-256 is not the one in %s (a hand edit? put a rule in tools/portstd/patches/%s)", rel(f), name, patchFile(pkg)))
			}
		}
		have, err := portedFiles(dir)
		if err != nil {
			probs = append(probs, err.Error())
		}
		for _, f := range have {
			if _, ok := files[f]; !ok {
				probs = append(probs, fmt.Sprintf("%s: has the header of a ported file, but %s has no hash for it (a new file by hand?)", rel(f), name))
			}
		}
		probs = append(probs, stubProblems(out, pkg)...)
	}
	for _, pkg := range sortedMapKeys(mapKeysSet(by)) {
		if _, ok := index[pkg]; !ok {
			probs = append(probs, fmt.Sprintf("%s: hashes for %s, which %s does not list", name, pkg, indexName))
		}
	}
	return n, probs
}

func mapKeysSet(m map[string]map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k := range m {
		out[k] = ""
	}
	return out
}

// checkSumsFile compares PORTED.sha256 with the hashes of the ports r (with Go's source: the
// recorded hashes must be those of what the tool writes). all says the ports are the whole manifest.
func checkSumsFile(out string, rs []*Result, all bool) []string {
	have, err := readSums(out)
	if err != nil {
		return []string{err.Error()}
	}
	name := filepath.ToSlash(filepath.Join(filepath.Base(out), sumsName))
	want := make(map[string]string)
	var probs []string
	for _, r := range rs {
		for p, h := range sumsOf(r) {
			want[p] = h
			if have[p] != h {
				probs = append(probs, fmt.Sprintf("%s: the hash of %s is %q, the port writes %q", name, p, have[p], h))
			}
		}
		for p := range have {
			if path.Dir(p) == r.Pkg {
				if _, ok := want[p]; !ok {
					probs = append(probs, fmt.Sprintf("%s: lists %s, which the port does not write", name, p))
				}
			}
		}
	}
	if all {
		for _, p := range sortedMapKeys(have) {
			if _, ok := want[p]; !ok && !slices.ContainsFunc(rs, func(r *Result) bool { return r.Pkg == path.Dir(p) }) {
				probs = append(probs, fmt.Sprintf("%s: lists %s, of a package ports.txt does not have", name, p))
			}
		}
		if text, err := os.ReadFile(filepath.Join(out, sumsName)); err == nil && len(probs) == 0 && string(text) != sumsText(have) {
			probs = append(probs, name+": is not in the form the tool writes (sorted lines, header)")
		}
	}
	return probs
}

func sortedBytesKeys(m map[string][]byte) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// checkIndex compares PORTED.txt with the lines of the ports. all says the
// ports are the whole manifest: then a line of another package is a problem.
func checkIndex(out string, rs []*Result, all bool) []string {
	have, err := readIndex(out)
	if err != nil {
		return []string{err.Error()}
	}
	name := filepath.ToSlash(filepath.Join(filepath.Base(out), indexName))
	var probs []string
	want := make(map[string]string)
	for _, r := range rs {
		want[r.Pkg] = indexLine(r)
	}
	for _, pkg := range sortedMapKeys(want) {
		if have[pkg] != want[pkg] {
			probs = append(probs, fmt.Sprintf("%s: line of %s is %q, the port writes %q", name, pkg, have[pkg], want[pkg]))
		}
	}
	if all {
		for _, pkg := range sortedMapKeys(have) {
			if _, ok := want[pkg]; !ok {
				probs = append(probs, fmt.Sprintf("%s: lists %s, which ports.txt does not", name, pkg))
			}
		}
		if text, err := os.ReadFile(filepath.Join(out, indexName)); err == nil && len(probs) == 0 && string(text) != indexText(have) {
			probs = append(probs, name+": is not in the form the tool writes (sorted lines, header)")
		}
	}
	return probs
}
