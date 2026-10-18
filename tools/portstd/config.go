package main

import (
	"bufio"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
)

// The configuration of the tool is a folder of small text files next to
// the code (embedded in the tool; -config dir reads another folder):
//
//	PINS          the Go version, the URL and the SHA-256 of its source tarball
//	ports.txt     the packages that stdlib/ holds as ports, and their options
//	natives.txt   what a //go:linkname declaration becomes (a native function)
//	boundary.txt  the packages that are not ported, and who provides them
//	tiers.txt     named package lists for -list (the tier-1 list of step 9.4)
//	patches/      one rules file per package (patch.go)
//
//go:embed PINS ports.txt natives.txt boundary.txt tiers.txt all:patches
var embeddedConfig embed.FS

// Options are the choices of one port: the target systems, the architecture
// and the extra build tags.
type Options struct {
	OS   []string // GOOS values, sorted: the files are the union over them
	Arch string   // GOARCH value for the file selection: "generic" is none of Go's, so that no assembly variant is chosen
	Tags []string // build tags that are true, sorted
}

// String returns the options as PORTED.txt writes them.
func (o Options) String() string {
	return fmt.Sprintf("os=%s arch=%s tags=%s", strings.Join(o.OS, ","), o.Arch, strings.Join(o.Tags, ","))
}

// A PortSpec is one line of ports.txt.
type PortSpec struct {
	Pkg  string
	Opts Options
}

// A Boundary is a package that the tool does not port.
type Boundary struct {
	Pkg   string
	Owner string // a step of the plan, or "builtin"
	What  string
}

// A NativeEntry says what a bodyless function becomes.
type NativeEntry struct {
	Key  string
	Impl string // a C++ name; "local" for the form without an argument; "" with TODO
	TODO string // the reason, when no native exists yet
}

// Config is everything the tool reads besides the Go source.
type Config struct {
	Pin      Pin
	Defaults Options
	Ports    []PortSpec
	Options  map[string]Options // per-package options of "options" lines
	Natives  map[string]NativeEntry
	Boundary map[string]Boundary
	Tiers    map[string][]string
	Patches  fs.FS // the patches/ folder, or nil

	// Warnings are what is wrong with the files without being an error: a rules file
	// that no package of ports.txt uses (a typo in its name would drop the patch silently).
	Warnings []string
}

// LoadConfig reads the configuration files from fsys.
func LoadConfig(fsys fs.FS) (*Config, error) {
	c := &Config{
		Natives:  make(map[string]NativeEntry),
		Boundary: make(map[string]Boundary),
		Tiers:    make(map[string][]string),
	}
	b, err := fs.ReadFile(fsys, "PINS")
	if err != nil {
		return nil, err
	}
	if c.Pin, err = parsePin(string(b)); err != nil {
		return nil, fmt.Errorf("PINS: %v", err)
	}
	b, err = fs.ReadFile(fsys, "ports.txt")
	if err != nil {
		return nil, err
	}
	if err := c.parsePorts(string(b)); err != nil {
		return nil, fmt.Errorf("ports.txt: %v", err)
	}
	for _, f := range []struct {
		name  string
		parse func(line string) error
	}{
		{"natives.txt", c.parseNative},
		{"boundary.txt", c.parseBoundary},
		{"tiers.txt", c.parseTier},
	} {
		b, err := fs.ReadFile(fsys, f.name)
		if err != nil {
			return nil, err
		}
		for n, line := range configLines(string(b)) {
			if err := f.parse(line); err != nil {
				return nil, fmt.Errorf("%s:%d: %v", f.name, n, err)
			}
		}
	}
	if sub, err := fs.Sub(fsys, "patches"); err == nil {
		c.Patches = sub
		c.warnOrphanPatches()
	}
	return c, nil
}

// warnOrphanPatches adds a warning for each rules file whose package is not in ports.txt.
func (c *Config) warnOrphanPatches() {
	entries, err := fs.ReadDir(c.Patches, ".")
	if err != nil {
		return
	}
	ported := make(map[string]bool)
	for _, ps := range c.Ports {
		ported[patchFile(ps.Pkg)] = true
	}
	for _, e := range entries {
		if e.IsDir() || path.Ext(e.Name()) != ".rules" || ported[e.Name()] {
			continue
		}
		c.Warnings = append(c.Warnings, fmt.Sprintf("patches/%s is the rules file of no package of ports.txt (a typo in the name? the file is ignored)", e.Name()))
	}
}

// configLines returns the lines of a configuration file without the empty
// lines and the comments (# at the start of a line), by line number.
func configLines(text string) map[int]string {
	out := make(map[int]string)
	sc := bufio.NewScanner(strings.NewReader(text))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			out[n] = line
		}
	}
	return out
}

// sortedLines is configLines in line order.
func sortedLines(m map[int]string) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// parsePorts reads ports.txt: a "default" line with the options of every
// package, "options <package> ..." lines with the options of a package that
// is not ported yet (the -list closure uses them), then one package per line
// whose options replace the others.
func (c *Config) parsePorts(text string) error {
	lines := configLines(text)
	c.Defaults = Options{OS: []string{"linux"}, Arch: "generic", Tags: []string{"purego"}}
	c.Options = make(map[string]Options)
	seen := make(map[string]bool)
	order := sortedLines(lines)
	// Pass 1: default and options lines (the default must come first).
	for _, n := range order {
		f := strings.Fields(lines[n])
		switch f[0] {
		case "default":
			if len(c.Options) > 0 || seen["package"] {
				return fmt.Errorf("line %d: the default line must come before the others", n)
			}
			o, err := parseOptions(c.Defaults, f[1:])
			if err != nil {
				return fmt.Errorf("line %d: %v", n, err)
			}
			c.Defaults = o
		case "options":
			if len(f) < 2 {
				return fmt.Errorf("line %d: usage: options <package> [key=value]", n)
			}
			o, err := parseOptions(c.Defaults, f[2:])
			if err != nil {
				return fmt.Errorf("line %d: %v", n, err)
			}
			c.Options[f[1]] = o
		default:
			seen["package"] = true
		}
	}
	// Pass 2: the packages.
	for _, n := range order {
		f := strings.Fields(lines[n])
		if f[0] == "default" || f[0] == "options" {
			continue
		}
		base := c.optionsFor(f[0])
		o, err := parseOptions(base, f[1:])
		if err != nil {
			return fmt.Errorf("line %d: %v", n, err)
		}
		if seen[f[0]] {
			return fmt.Errorf("line %d: %s twice", n, f[0])
		}
		seen[f[0]] = true
		c.Ports = append(c.Ports, PortSpec{Pkg: f[0], Opts: o})
	}
	slices.SortFunc(c.Ports, func(a, b PortSpec) int { return strings.Compare(a.Pkg, b.Pkg) })
	return nil
}

// optionsFor returns the options of a package: its "options" line, else the
// default.
func (c *Config) optionsFor(pkg string) Options {
	if o, ok := c.Options[pkg]; ok {
		return o
	}
	return c.Defaults
}

// parseOptions applies key=value fields to base.
func parseOptions(base Options, fields []string) (Options, error) {
	o := base
	for _, f := range fields {
		k, v, ok := strings.Cut(f, "=")
		if !ok {
			return o, fmt.Errorf("%q is not key=value", f)
		}
		switch k {
		case "os":
			o.OS = splitList(v)
		case "arch":
			o.Arch = v
		case "tags":
			o.Tags = splitList(v)
		default:
			return o, fmt.Errorf("unknown option %q (os, arch, tags)", k)
		}
	}
	if len(o.OS) == 0 || o.Arch == "" {
		return o, fmt.Errorf("os and arch must not be empty")
	}
	return o, nil
}

// splitList splits "a,b" into a sorted list without duplicates; "-" is none.
func splitList(s string) []string {
	if s == "-" || s == "" {
		return nil
	}
	l := strings.Split(s, ",")
	slices.Sort(l)
	return slices.Compact(l)
}

// parseNative reads a line of natives.txt: key, then a C++ name, "local", or
// "TODO: reason".
func (c *Config) parseNative(line string) error {
	key, rest, _ := strings.Cut(line, " ")
	rest = strings.TrimSpace(rest)
	if !strings.Contains(key, ".") || rest == "" {
		return fmt.Errorf("want: <package>.<name> <C++ name | local | TODO: reason>")
	}
	if _, dup := c.Natives[key]; dup {
		return fmt.Errorf("%s twice", key)
	}
	e := NativeEntry{Key: key}
	switch {
	case strings.HasPrefix(rest, "TODO"):
		e.TODO = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(rest, "TODO"), ":"))
		if e.TODO == "" {
			return fmt.Errorf("TODO needs a reason")
		}
	case rest == "local":
		e.Impl = "local"
	case strings.ContainsAny(rest, " \t"):
		return fmt.Errorf("%q is not a C++ name", rest)
	default:
		e.Impl = rest
	}
	c.Natives[key] = e
	return nil
}

func (c *Config) parseBoundary(line string) error {
	f := strings.Fields(line)
	if len(f) < 3 {
		return fmt.Errorf("want: <import path> <owner> <what>")
	}
	c.Boundary[f[0]] = Boundary{Pkg: f[0], Owner: f[1], What: strings.Join(f[2:], " ")}
	return nil
}

// parseTier reads "name: pkg pkg ..." lines; a name can continue on
// several lines.
func (c *Config) parseTier(line string) error {
	name, rest, ok := strings.Cut(line, ":")
	if !ok {
		return fmt.Errorf("want: <name>: <import path> [<import path>]")
	}
	c.Tiers[name] = append(c.Tiers[name], strings.Fields(rest)...)
	return nil
}

// patchFile returns the rules file name of a package: math/bits is
// math__bits.rules.
func patchFile(pkg string) string {
	return strings.ReplaceAll(pkg, "/", "__") + ".rules"
}

// pkgDepth returns how many folders deep a package is: unicode/utf8 is 2.
func pkgDepth(pkg string) int {
	return len(strings.Split(path.Clean(pkg), "/"))
}
