package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// indexName is the machine-readable list of the ported packages, in the
// output folder (stdlib/PORTED.txt).
const indexName = "PORTED.txt"

const indexHeader = `# The packages of stdlib/ that tools/portstd ports from Go's source, one line each:
#
#	<import path> <Go version> os=<GOOS list> arch=<GOARCH> tags=<build tags> files=<n> natives=<n> patches=<n>
#
# Written by go run ./tools/portstd; do not edit by hand (make portstd-check).
`

// sumsName is the list of the SHA-256 of every file the ports write, in the output
// folder (stdlib/PORTED.sha256, the format of sha256sum). It lets -check find a hand
// edit on a machine that has no copy of Go's source.
const sumsName = "PORTED.sha256"

const sumsHeader = `# The SHA-256 of every file that tools/portstd writes into stdlib/ (the .gp files and PORT.md),
# in the format of sha256sum. "go run ./tools/portstd -check" compares the files with it, also
# without Go's source in the cache; with the source it compares the bytes with what the tool writes.
# Written by go run ./tools/portstd; do not edit by hand (make portstd-check).
`

// outputs returns the files a port writes in its folder: the ported files and PORT.md.
func outputs(r *Result) map[string][]byte {
	m := map[string][]byte{"PORT.md": []byte(portMD(r))}
	for _, f := range r.Files {
		m[f.Name] = f.Content
	}
	return m
}

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// sumsOf returns the lines of PORTED.sha256 of a port: path under the output folder -> hash.
func sumsOf(r *Result) map[string]string {
	m := make(map[string]string)
	for name, b := range outputs(r) {
		m[path.Join(r.Pkg, name)] = hashOf(b)
	}
	return m
}

// readSums returns PORTED.sha256: path -> hash.
func readSums(out string) (map[string]string, error) {
	b, err := os.ReadFile(filepath.Join(out, sumsName))
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	m := make(map[string]string)
	for n, line := range configLines(string(b)) {
		f := strings.Fields(line)
		if len(f) != 2 || !sha256Rx.MatchString(f[0]) {
			return nil, fmt.Errorf("%s:%d: want: <sha256>  <path>", sumsName, n)
		}
		m[f[1]] = f[0]
	}
	return m, nil
}

// sumsText returns the text of PORTED.sha256 for the entries.
func sumsText(m map[string]string) string {
	var b strings.Builder
	b.WriteString(sumsHeader)
	b.WriteString("\n")
	for _, k := range sortedMapKeys(m) {
		fmt.Fprintf(&b, "%s  %s\n", m[k], k)
	}
	return b.String()
}

// sumsByPkg groups the entries of PORTED.sha256 by the package (the folder) they are in.
func sumsByPkg(m map[string]string) map[string]map[string]string {
	out := make(map[string]map[string]string)
	for p, h := range m {
		pkg := path.Dir(p)
		if out[pkg] == nil {
			out[pkg] = make(map[string]string)
		}
		out[pkg][path.Base(p)] = h
	}
	return out
}

// updateSums puts the hashes of the ports into PORTED.sha256 (replacing the entries of those packages).
func updateSums(out string, rs []*Result) error {
	m, err := readSums(out)
	if err != nil {
		return err
	}
	for _, r := range rs {
		for p := range m {
			if path.Dir(p) == r.Pkg {
				delete(m, p)
			}
		}
		for p, h := range sumsOf(r) {
			m[p] = h
		}
	}
	return writeFile(filepath.Join(out, sumsName), []byte(sumsText(m)))
}

func listOrNone(l []string) string {
	if len(l) == 0 {
		return "-"
	}
	return strings.Join(l, ",")
}

// indexLine returns the line of PORTED.txt for a port.
func indexLine(r *Result) string {
	return fmt.Sprintf("%s %s os=%s arch=%s tags=%s files=%d natives=%d patches=%d",
		r.Pkg, r.Pin.Version, listOrNone(r.Opts.OS), r.Opts.Arch, listOrNone(r.Opts.Tags), len(r.Files), len(r.Natives), len(r.Patches))
}

// portMD returns the PORT.md of a port: what the package is made of and what
// differs from Go's source. It depends on the port alone, never on the state
// of the rest of stdlib/, so that the check mode can compare it byte for byte.
func portMD(r *Result) string {
	var b strings.Builder
	pr := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }
	license := strings.Repeat("../", pkgDepth(r.Pkg)) + "LICENSE"
	pr("# %s\n\n", r.Pkg)
	pr("Ported by `tools/portstd` from %s's `src/%s` (BSD-3-Clause, see `%s`). Do not edit the `.gp` files by hand: ", r.Pin.Version, r.Pkg, license)
	pr("the tool writes them again, and `make portstd-check` fails when a file differs from what it writes. ")
	pr("A change goes into `tools/portstd/patches/%s` (a rule with a reason), or into the tool.\n\n", patchFile(r.Pkg))
	pr("| | |\n|---|---|\n")
	pr("| Source | %s, `%s` (SHA-256 of the tarball `%s`) |\n", r.Pin.Version, path0(r.Pin.URL), r.Pin.SHA256)
	pr("| Systems (GOOS) | %s |\n", strings.Join(r.Opts.OS, ", "))
	pr("| Architecture (GOARCH) | `%s`: no architecture-specific file, so no assembly variant |\n", r.Opts.Arch)
	pr("| Build tags | %s |\n", codeList(r.Opts.Tags))
	pr("| Regenerate | `go run ./tools/portstd %s` |\n\n", r.Pkg)

	pr("## Files taken\n\n")
	for _, f := range r.Files {
		note := ""
		if len(r.Opts.OS) > 1 && len(f.For) < len(r.Opts.OS) {
			note = " (" + strings.Join(f.For, ", ") + " only)"
		}
		pr("- `%s`, from `%s`%s\n", f.Name, f.From, note)
	}
	pr("\n## Files not taken\n\n")
	if len(r.Skipped) == 0 && r.Tests == 0 {
		pr("None.\n")
	}
	for _, s := range r.Skipped {
		pr("- `%s`: %s\n", s.File, s.Reason)
	}
	if r.Tests > 0 {
		pr("- %d `_test.go` files: Go's tests of the package are run by step 9.10, not copied here\n", r.Tests)
	}

	pr("\n## Patches\n\n")
	if len(r.Patches) == 0 {
		pr("None: the files are Go's, with the header line%s.\n", plural(r.Natives, " and the native declarations below"))
	}
	for _, p := range r.Patches {
		times := ""
		if p.Count > 1 {
			times = fmt.Sprintf(" (%d places)", p.Count)
		}
		pr("- `%s`%s: %s\n", p.File, times, p.Why)
	}

	pr("\n## Native functions\n\n")
	if len(r.Natives) == 0 {
		pr("None.\n")
	} else {
		pr("| Function | Go linkname | Native |\n|---|---|---|\n")
		for _, n := range r.Natives {
			impl := "`//gocpp:native " + n.Impl + "`"
			if n.Impl == "local" {
				impl = "`//gocpp:native` (the body is in a `.cpp` file next to the `.gp` file)"
			}
			link := n.Link
			if link == "" {
				link = "(none: no body upstream)"
			}
			pr("| `%s` (`%s`) | %s | %s |\n", n.Func, n.File, link, impl)
		}
	}

	pr("\n## Findings\n\n")
	var any bool
	for _, f := range r.Findings {
		any = true
		where := f.File
		if f.Line > 0 {
			where = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		sev := ""
		if f.Blocker {
			sev = " **blocker**"
		}
		pr("- `%s`%s, %s: %s\n", f.Kind, sev, where, f.Detail)
	}
	if !any {
		pr("None.\n")
	}

	pr("\n## Known divergences from Go\n\n")
	if len(r.Notes) == 0 {
		pr("None.\n")
	}
	for _, n := range r.Notes {
		pr("- %s\n", n)
	}
	return b.String()
}

// path0 returns the file name of a URL.
func path0(u string) string {
	return u[strings.LastIndex(u, "/")+1:]
}

func codeList(l []string) string {
	if len(l) == 0 {
		return "none"
	}
	var q []string
	for _, s := range l {
		q = append(q, "`"+s+"`")
	}
	return strings.Join(q, ", ")
}

func plural(n []Native, s string) string {
	if len(n) == 0 {
		return ""
	}
	return s
}

// pkgDir returns the folder of a package in the output tree.
func pkgDir(out, pkg string) string {
	return filepath.Join(out, filepath.FromSlash(pkg))
}

// portedFiles returns the .gp files of the folder that carry the header of
// a ported file (the others are hand-written: natives, shims).
func portedFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".gp" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		if bytes.HasPrefix(b, []byte(portedMarker)) {
			out = append(out, e.Name())
		}
	}
	slices.Sort(out)
	return out, nil
}

// writeFile writes a file when its bytes change (so file times and builds
// stay quiet when nothing changed).
func writeFile(name string, content []byte) error {
	if old, err := os.ReadFile(name); err == nil && bytes.Equal(old, content) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	return os.WriteFile(name, content, 0o644)
}

// stubMarker is in the header of a file that tools/stubgen wrote: a package that becomes real
// by a port has no use for it.
const stubMarker = "// Code generated by tools/stubgen"

// foreignFiles returns the .gp files of dir that tools/portstd did not write: the stubs of
// tools/stubgen, and the hand-written ones.
func foreignFiles(dir string) (stubs, hand []string, err error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".gp" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, nil, err
		}
		switch {
		case bytes.HasPrefix(b, []byte(portedMarker)):
		case bytes.Contains(firstLines(b, 12), []byte(stubMarker)):
			stubs = append(stubs, e.Name())
		default:
			hand = append(hand, e.Name())
		}
	}
	return stubs, hand, nil
}

func firstLines(b []byte, n int) []byte {
	for i := range b {
		if b[i] == '\n' {
			if n--; n == 0 {
				return b[:i]
			}
		}
	}
	return b
}

// prepareWrite looks at the folder of a port before it is written. A stub of tools/stubgen is
// removed (the package is real now: see "Replacing a stub" in docs/guides/porting.md). A
// hand-written .gp file that the port overwrites, or that stays beside the port (and would
// declare the same names twice), is refused unless force is set; with force the first is
// overwritten and the second is kept, with a warning.
func prepareWrite(out string, r *Result, force bool, log io.Writer) error {
	dir := pkgDir(out, r.Pkg)
	stubs, hand, err := foreignFiles(dir)
	if err != nil {
		return err
	}
	writes := make(map[string]bool)
	for _, f := range r.Files {
		writes[f.Name] = true
	}
	var clash, stale []string
	for _, h := range hand {
		if writes[h] {
			clash = append(clash, h)
		} else {
			stale = append(stale, h)
		}
	}
	if (len(clash) > 0 || len(stale) > 0) && !force {
		var b strings.Builder
		fmt.Fprintf(&b, "%s: not ported: the folder %s holds hand-written .gp files (-force writes the port anyway):", r.Pkg, filepath.ToSlash(dir))
		for _, h := range clash {
			fmt.Fprintf(&b, "\n    %s: the port overwrites it (delete it first if the port replaces it)", h)
		}
		for _, h := range stale {
			fmt.Fprintf(&b, "\n    %s: the port does not write it, so it stays beside the port and would declare the same names twice (an old stub?)", h)
		}
		return errors.New(b.String())
	}
	for _, h := range stale {
		fmt.Fprintf(log, "portstd: warning: %s: %s is hand-written and stays beside the port: delete it if it declares what the port declares\n", r.Pkg, filepath.ToSlash(filepath.Join(dir, h)))
	}
	for _, s := range stubs {
		if !writes[s] {
			if err := os.Remove(filepath.Join(dir, s)); err != nil {
				return err
			}
			fmt.Fprintf(log, "portstd: %s: removed the stub %s (tools/stubgen: set Real: true for the package, see docs/guides/porting.md)\n", r.Pkg, filepath.ToSlash(filepath.Join(dir, s)))
		}
	}
	return nil
}

// writePort writes the files and the PORT.md of a port into out, and
// removes the files of an earlier port that the new one does not have.
func writePort(out string, r *Result) error {
	dir := pkgDir(out, r.Pkg)
	old, err := portedFiles(dir)
	if err != nil {
		return err
	}
	keep := make(map[string]bool)
	for _, f := range r.Files {
		keep[f.Name] = true
		if err := writeFile(filepath.Join(dir, f.Name), f.Content); err != nil {
			return err
		}
	}
	for _, n := range old {
		if !keep[n] {
			if err := os.Remove(filepath.Join(dir, n)); err != nil {
				return err
			}
		}
	}
	return writeFile(filepath.Join(dir, "PORT.md"), []byte(portMD(r)))
}

// readIndex returns the lines of PORTED.txt by import path.
func readIndex(out string) (map[string]string, error) {
	b, err := os.ReadFile(filepath.Join(out, indexName))
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	m := make(map[string]string)
	for _, line := range configLines(string(b)) {
		m[strings.Fields(line)[0]] = line
	}
	return m, nil
}

// indexText returns the text of PORTED.txt for the lines.
func indexText(m map[string]string) string {
	var b strings.Builder
	b.WriteString(indexHeader)
	b.WriteString("\n")
	for _, k := range sortedMapKeys(m) {
		b.WriteString(m[k])
		b.WriteString("\n")
	}
	return b.String()
}

func sortedMapKeys(m map[string]string) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// updateIndex puts the lines of the ports into PORTED.txt.
func updateIndex(out string, rs []*Result) error {
	m, err := readIndex(out)
	if err != nil {
		return err
	}
	for _, r := range rs {
		m[r.Pkg] = indexLine(r)
	}
	return writeFile(filepath.Join(out, indexName), []byte(indexText(m)))
}
