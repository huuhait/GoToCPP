package gocppmod

import (
	"fmt"
	"strings"
	"sync"
)

// VendorFile is the name of the list of vendored modules, in the vendor
// folder at the top of the main module. It has the format of Go's
// vendor/modules.txt:
//
//	# example.com/lib v1.2.3
//	## explicit
//	example.com/lib
//	example.com/lib/sub
//	# example.com/other v0.1.0 => ../other
//	## explicit
//	example.com/other
//
// A "# path version" line starts a module, "## explicit" marks a module that
// the gocpp.mod file requires itself (not one that it needs through another
// module), " => dir" says that the module is replaced, and the other lines
// are the packages of the module that are vendored. Text after a ";" in a
// "##" line is for tools and ignored.
const VendorFile = "modules.txt"

// A VendorModule is a module of vendor/modules.txt.
type VendorModule struct {
	Path, Version string
	Explicit      bool
	Replaced      bool // " => " follows the version
	NewPath       string
	NewVersion    string
	Packages      []string // the vendored packages, as listed
	Line          int      // the line of the module in modules.txt
}

// VendorModules is a parsed vendor/modules.txt.
type VendorModules struct {
	Name    string
	Modules []*VendorModule

	once   sync.Once
	byPath map[string][]*VendorModule // in file order
}

func (v *VendorModules) index() {
	v.once.Do(func() {
		v.byPath = make(map[string][]*VendorModule, len(v.Modules))
		for _, m := range v.Modules {
			v.byPath[m.Path] = append(v.byPath[m.Path], m)
		}
	})
}

// Module returns the first vendored module with the path, or nil.
func (v *VendorModules) Module(path string) *VendorModule {
	v.index()
	if ms := v.byPath[path]; len(ms) > 0 {
		return ms[0]
	}
	return nil
}

// ParseVendor reads the text of a vendor/modules.txt. name is the file name
// in the errors.
func ParseVendor(name string, data []byte) (*VendorModules, ErrorList) {
	v := &VendorModules{Name: name}
	if len(data) > maxFileSize {
		return v, ErrorList{{Filename: name, Pos: Position{Line: 1, LineRune: 1},
			Err: fmt.Sprintf("file is too large (%d bytes, the limit is %d)", len(data), maxFileSize)}}
	}
	var errs ErrorList
	fail := func(line int, format string, args ...any) {
		errs = append(errs, Error{Filename: name, Pos: Position{Line: line, LineRune: 1}, Err: fmt.Sprintf(format, args...)})
	}
	var cur *VendorModule
	for i, text := range strings.Split(string(data), "\n") {
		line := i + 1
		text = strings.TrimSpace(text)
		switch {
		case text == "":
		case strings.HasPrefix(text, "## "):
			if cur == nil {
				fail(line, "malformed line: annotation before the first module: %s", text)
				continue
			}
			for _, a := range strings.Split(strings.TrimPrefix(text, "## "), ";") {
				if strings.TrimSpace(a) == "explicit" {
					cur.Explicit = true
				}
			}
		case strings.HasPrefix(text, "# "):
			m := &VendorModule{Line: line}
			f := strings.Fields(strings.TrimPrefix(text, "# "))
			arrow := -1
			for j, w := range f {
				if w == "=>" {
					arrow = j
				}
			}
			head := f
			if arrow >= 0 {
				head = f[:arrow]
				if len(head) == 0 {
					fail(line, "malformed line: %s", text)
					continue
				}
				tail := f[arrow+1:]
				if len(tail) < 1 || len(tail) > 2 {
					fail(line, "malformed line: %s", text)
					continue
				}
				m.Replaced = true
				m.NewPath = tail[0]
				if len(tail) == 2 {
					m.NewVersion = tail[1]
				}
				if (m.NewVersion == "") != IsDirectoryPath(m.NewPath) {
					fail(line, "malformed line: the replacement of %s must be a folder, or a module with a version: %s", head[0], text)
					continue
				}
			}
			switch len(head) {
			case 2:
				m.Path, m.Version = head[0], head[1]
			case 1:
				// "# path => dir": a replaced module that gocpp.mod doesn't require.
				if !m.Replaced {
					fail(line, "malformed line: %s", text)
					continue
				}
				m.Path = head[0]
			default:
				fail(line, "malformed line: %s", text)
				continue
			}
			if m.Version != "" && !ValidVersion(m.Version) {
				fail(line, "malformed line: invalid module version %q: %s", m.Version, text)
				continue
			}
			m.Version, m.NewVersion = CanonicalVersion(m.Version), CanonicalVersion(m.NewVersion)
			v.Modules = append(v.Modules, m)
			cur = m
		case strings.HasPrefix(text, "#"):
			// A comment.
		default:
			if cur == nil {
				fail(line, "malformed line: package before the first module: %s", text)
				continue
			}
			cur.Packages = append(cur.Packages, text)
		}
	}
	return v, errs
}

// CheckVendor compares vendor/modules.txt with the gocpp.mod file f, as
// "go mod vendor" and the go command do, and returns one message for each
// difference. There are none if the vendor folder is in sync:
//
//   - every require of f is in modules.txt with the same version, marked
//     explicit;
//   - every module that modules.txt marks explicit is required by f;
//   - every replace of f is the replacement that modules.txt says, and
//     every replacement in modules.txt is a replace of f.
func CheckVendor(f *File, v *VendorModules) []string {
	var out []string
	v.index()
	vendored := make(map[string]*VendorModule, len(v.Modules))
	for _, m := range v.Modules {
		vendored[m.Path+"@"+m.Version] = m
	}
	required := make(map[string]*Require, len(f.Require))
	for _, r := range f.Require {
		required[r.Path+"@"+r.Version] = r
		m := vendored[r.Path+"@"+r.Version]
		switch other := v.Module(r.Path); {
		case m != nil && m.Explicit:
		case m != nil:
			out = append(out, fmt.Sprintf("%s@%s: is explicitly required in %s, but not marked as explicit in vendor/%s", r.Path, r.Version, FileName, VendorFile))
		case other != nil && other.Version != "":
			out = append(out, fmt.Sprintf("%s@%s: is required in %s, but vendor/%s has version %s", r.Path, r.Version, FileName, VendorFile, other.Version))
		default:
			out = append(out, fmt.Sprintf("%s@%s: is explicitly required in %s, but not marked as explicit in vendor/%s", r.Path, r.Version, FileName, VendorFile))
		}
	}
	for _, m := range v.Modules {
		if m.Explicit && required[m.Path+"@"+m.Version] == nil {
			out = append(out, fmt.Sprintf("%s@%s: is marked as explicit in vendor/%s, but not explicitly required in %s", m.Path, m.Version, VendorFile, FileName))
		}
	}
	for _, r := range f.Replace { // a replace of a module that is not vendored does nothing
		for _, m := range v.byPath[r.OldPath] {
			if r.OldVersion != "" && r.OldVersion != m.Version {
				continue
			}
			switch {
			case !m.Replaced:
				out = append(out, fmt.Sprintf("%s: is replaced in %s, but not marked as replaced in vendor/%s", what(r.OldPath, m.Version), FileName, VendorFile))
			case m.NewPath != r.NewPath || m.NewVersion != r.NewVersion:
				out = append(out, fmt.Sprintf("%s: is replaced by %s in %s, but marked as replaced by %s in vendor/%s",
					what(r.OldPath, m.Version), what(r.NewPath, r.NewVersion), FileName, what(m.NewPath, m.NewVersion), VendorFile))
			}
		}
	}
	for _, m := range v.Modules {
		if m.Replaced && f.FindReplace(m.Path, m.Version) == nil {
			out = append(out, fmt.Sprintf("%s: is marked as replaced by %s in vendor/%s, but not replaced in %s",
				what(m.Path, m.Version), what(m.NewPath, m.NewVersion), VendorFile, FileName))
		}
	}
	return out
}

// FileName is the name of the module file.
const FileName = "gocpp.mod"

func what(path, version string) string {
	if version == "" {
		return path
	}
	return path + "@" + version
}
