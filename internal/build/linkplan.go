package build

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// The link command (Phase 06 step 6.9). One function makes it from the parts that the build has
// (linkArgs), so that its order is fixed and the same for the same program:
//
//	driver flags   debug flags, sanitizers, LTO; then -Wl,--gc-sections (not on macOS)
//	objects        the generated units in the order of the packages, then the objects of the
//	               packages' own C and C++ files, then the objects of the modules' libraries
//	               (each once)
//	runtime        libgocpprt.a (rtlib.go), after what uses it
//	libraries      the link flags of the packages, dependents first
//	               (deduplicated, see dedupLinkFlags), then GOCPP_LDFLAGS
//	-o out
//
// A static library is read once, front to back, so what needs a library comes before it: the
// objects before the runtime, the runtime before the -l flags. A flag that names the same
// library twice keeps the later one, because the later one is behind more of its users.

// A linkParts is the link command in its three parts: the driver flags (Pre), the files and flags
// that follow them (Objs, then Post), and the flags of GOCPP_LDFLAGS alone (ldflags, the last of Post).
type linkParts struct {
	pre, objs, post, ldflags []string
}

// linkParts makes the parts of the link command, for the target b.GOOS. objs, libObjs and rtObjs are
// object files or libraries (the runtime's library is one file), link are the link flags of the
// packages in the order that they were collected.
func (b *Builder) linkParts(objs, libObjs, rtObjs, link []string) (linkParts, error) {
	// --gc-sections drops the functions and data that nothing reaches (the compiles put each in
	// a section of its own); macOS's linker does that by itself.
	var gc []string
	if b.GOOS != "darwin" {
		gc = []string{"-Wl,--gc-sections"}
	}
	ldflags := b.autoLTO(b.Env.LDFlags) // a bare -flto becomes -flto=auto for g++ (see autoLTO)
	pre := slices.Concat(b.linkFlags(), gc, b.Mode.LinkFlags(b.Soname, b.VersionScript))
	if b.LTO { // -lto and -flto in GOCPP_LDFLAGS: the flag once
		ldflags = slices.DeleteFunc(slices.Clone(ldflags), func(f string) bool { return slices.Contains(pre, f) && strings.HasPrefix(f, "-flto") })
	}
	return linkParts{
		pre:     pre,
		objs:    dedupFiles(slices.Concat(objs, libObjs, rtObjs)),
		post:    slices.Concat(dedupLinkFlags(link), ldflags),
		ldflags: ldflags,
	}, nil
}

// linkCommand returns the arguments of the link command for the parts: the driver flags, the files
// and flags, and -o out, in a response file when the command is too long.
func (b *Builder) linkCommand(p linkParts, out string) ([]string, error) {
	return b.responseArgs(slices.Concat(p.pre, p.objs, p.post, []string{"-o", out}), len(p.pre))
}

// linkArgs returns the arguments of the link command (linkParts, linkCommand).
func (b *Builder) linkArgs(objs, libObjs, rtObjs, link []string, out string) ([]string, error) {
	p, err := b.linkParts(objs, libObjs, rtObjs, link)
	if err != nil {
		return nil, err
	}
	return b.linkCommand(p, out)
}

// dedupFiles removes the files that are named twice (by their cleaned path), keeping the last
// one: a second object file would define everything twice, and a second library would only be
// read again.
func dedupFiles(files []string) []string {
	last := make(map[string]int, len(files))
	for i, f := range files {
		last[filepath.Clean(f)] = i
	}
	out := make([]string, 0, len(files))
	for i, f := range files {
		if last[filepath.Clean(f)] == i {
			out = append(out, f)
		}
	}
	return out
}

// A linkItem is one flag of the link line, with the words that belong to it (-l m is one
// item, -framework Cocoa is one), as dedupLinkFlags reads them.
type linkItem struct {
	words []string
	kind  int    // itemOther, itemLib, itemDir, itemOnce, itemFile
	key   string // what makes two items the same
}

const (
	itemOther = iota // kept as it comes
	itemLib          // -lname: the last one stays (its users come before it)
	itemDir          // -Ldir: the first one stays (the search order)
	itemOnce         // an idempotent flag (-pthread, -rdynamic, -framework X): the first one stays
	itemFile         // an object file or a library named by its path: the last one stays
)

// pairFlags are the options whose argument is the next word.
var pairFlags = map[string]bool{
	"-arch": true, "-target": true, "-Xlinker": true, "-x": true, "-isysroot": true, "-F": true, "-sysroot": true, "--sysroot": true,
	"-z": true, "-u": true, "-T": true, "-e": true, "-rpath": true, "-soname": true, "-h": true, "-init": true, "-fini": true,
	"-Map": true, "-dynamic-linker": true, "-o": true, "-include": true, "-imacros": true, "-isystem": true, "-iquote": true,
	"-idirafter": true, "-I": true, "-D": true, "-U": true, "-B": true, "-rpath-link": true, "-y": true, "-wrap": true,
	"-version-script": true, "--version-script": true, "-Tdata": true, "-Ttext": true, "-Tbss": true,
}

// onceFlags are the link flags that mean the same however often they come.
var onceFlags = []string{"-pthread", "-rdynamic", "-static", "-static-libstdc++", "-static-libgcc", "-shared", "-pie", "-no-pie",
	"-Wl,--as-needed", "-Wl,--no-as-needed", "-Wl,--no-undefined", "-Wl,-z,relro", "-Wl,-z,now", "-Wl,--export-dynamic", "-Wl,-E", "-Wl,--gc-sections"}

// dedupLinkFlags removes what a link line says twice, without changing its meaning:
//   - -lname (and -l name) once: the last one stays, since GNU ld reads a static library
//     once and takes from it what the files before it need (a library that two packages
//     list, the one that depends on the other first, stays behind both). A library that comes
//     under another linker state (-Wl,-Bstatic against -Wl,-Bdynamic, -Wl,--as-needed, a
//     -Wl,--push-state group) is another one;
//   - -Ldir once: the first one stays, because the search order is the order of the folders;
//   - a file (libfoo.a, x.o) once: the last one stays;
//   - -pthread, -rdynamic, -static, -framework X and the like once: the first one stays;
//   - everything else as it comes (a flag with an argument stays whole).
//
// A line with -Wl,--start-group (or --whole-archive) has an order that this can't judge: the
// libraries of it stay as they are, and only the folders and the idempotent flags are
// deduplicated.
func dedupLinkFlags(flags []string) []string {
	var items []linkItem
	grouped := false
	var state []string // the linker state that -l means: the last of each pair, a stack for push/pop
	var stack [][]string
	curState := func() string { return strings.Join(state, ",") }
	epoch := 0 // counts the changes of the state: a flag that sets the state again is the same flag only within one epoch
	setState := func(on, off string) {
		if !slices.Contains(state, on) {
			epoch++
		}
		state = slices.DeleteFunc(state, func(s string) bool { return s == on || s == off })
		state = append(state, on)
		slices.Sort(state)
	}
	afterOption := false // the last word was an option that may take the next word as its argument
	for i := 0; i < len(flags); i++ {
		f := flags[i]
		switch {
		case f == "-Wl,--start-group" || f == "-Wl,--whole-archive" || f == "-Wl,-(" || f == "-Wl,--no-whole-archive":
			grouped = true
			items = append(items, linkItem{words: []string{f}})
		case f == "-Wl,-Bstatic" || f == "-Wl,--static":
			setState("static", "dynamic")
			items = append(items, linkItem{words: []string{f}})
		case f == "-Wl,-Bdynamic" || f == "-Wl,--dynamic":
			setState("dynamic", "static")
			items = append(items, linkItem{words: []string{f}})
		case f == "-Wl,--as-needed":
			setState("as-needed", "no-as-needed")
			items = append(items, linkItem{words: []string{f}, kind: itemOnce, key: f + "|" + fmt.Sprint(epoch)})
		case f == "-Wl,--no-as-needed":
			setState("no-as-needed", "as-needed")
			items = append(items, linkItem{words: []string{f}, kind: itemOnce, key: f + "|" + fmt.Sprint(epoch)})
		case f == "-Wl,--push-state":
			stack = append(stack, slices.Clone(state))
			items = append(items, linkItem{words: []string{f}})
		case f == "-Wl,--pop-state":
			if n := len(stack); n > 0 {
				state, stack = stack[n-1], stack[:n-1]
			}
			items = append(items, linkItem{words: []string{f}})
		case f == "-l" && i+1 < len(flags):
			items = append(items, linkItem{words: []string{f, flags[i+1]}, kind: itemLib, key: flags[i+1] + "|" + curState()})
			i++
		case strings.HasPrefix(f, "-l") && len(f) > 2:
			items = append(items, linkItem{words: []string{f}, kind: itemLib, key: f[2:] + "|" + curState()})
		case f == "-L" && i+1 < len(flags):
			items = append(items, linkItem{words: []string{f, flags[i+1]}, kind: itemDir, key: filepath.Clean(flags[i+1])})
			i++
		case strings.HasPrefix(f, "-L") && len(f) > 2:
			items = append(items, linkItem{words: []string{f}, kind: itemDir, key: filepath.Clean(f[2:])})
		case f == "-framework" && i+1 < len(flags):
			items = append(items, linkItem{words: []string{f, flags[i+1]}, kind: itemOnce, key: f + " " + flags[i+1]})
			i++
		case strings.HasPrefix(f, "-Wl,-framework,"):
			items = append(items, linkItem{words: []string{f}, kind: itemOnce, key: f})
		case pairFlags[f]:
			// An option with its argument as the next word: kept whole, as it comes, so that the
			// argument is never read as a library or a file (-u sym, -z now, -T script.ld).
			if i+1 < len(flags) {
				items = append(items, linkItem{words: []string{f, flags[i+1]}})
				i++
			} else {
				items = append(items, linkItem{words: []string{f}})
			}
			afterOption = true
			continue
		case slices.Contains(onceFlags, f):
			items = append(items, linkItem{words: []string{f}, kind: itemOnce, key: f})
		case !strings.HasPrefix(f, "-") && f != "" && !afterOption:
			items = append(items, linkItem{words: []string{f}, kind: itemFile, key: filepath.Clean(f)})
		default:
			// Everything else as it comes. A word after a flag that this function doesn't know may
			// be that flag's argument: it is not deduplicated either.
			items = append(items, linkItem{words: []string{f}})
			afterOption = strings.HasPrefix(f, "-") && !strings.ContainsAny(f, "=,")
			continue
		}
		afterOption = false
	}
	// The item that stays for each (kind, key).
	stay := make(map[[2]string]int)
	for i, it := range items {
		if it.key == "" {
			continue
		}
		k := [2]string{string(rune('0' + it.kind)), it.key}
		switch it.kind {
		case itemLib, itemFile:
			stay[k] = i // the last
		default:
			if _, ok := stay[k]; !ok {
				stay[k] = i // the first
			}
		}
	}
	out := make([]string, 0, len(flags))
	for i, it := range items {
		if it.key != "" && (it.kind == itemOnce || it.kind == itemDir || !grouped) {
			if stay[[2]string{string(rune('0' + it.kind)), it.key}] != i {
				continue
			}
		}
		out = append(out, it.words...)
	}
	return out
}

// responseThreshold is how long the link command may get (in bytes, all arguments with their
// separators) before the files and flags of the link go into a response file: Windows takes
// 32 K for a command, and a program of hundreds of units can pass that. A variable, for the tests.
var responseThreshold = 28000

func init() {
	if runtime.GOOS != "windows" {
		responseThreshold = 120000 // far below the limits of Linux and macOS
	}
}

// responseArgs returns args with the part between the driver flags and "-o out" in a response
// file in the work folder when the command is longer than responseThreshold: the compilers read
// @file (gcc and clang, also under a Nix wrapper). The driver flags and -o stay on the command
// line, so that the stamp's -Wl,-t goes where it did. A command that is short comes back as it
// was. The file is named link.rsp in the work folder (kept with -work).
func (b *Builder) responseArgs(args []string, nflags int) ([]string, error) {
	n := 0
	for _, a := range args {
		n += len(a) + 1
	}
	if n <= responseThreshold || len(args) <= nflags+2 {
		return args, nil
	}
	rest := args[nflags : len(args)-2]
	var text strings.Builder
	for _, a := range rest {
		text.WriteString(rspQuote(a, b.GOOS == "windows"))
		text.WriteByte('\n')
	}
	name := filepath.Join(b.Work, "link.rsp")
	if err := os.WriteFile(name, []byte(text.String()), 0o644); err != nil {
		return nil, err
	}
	out := slices.Clone(args[:nflags])
	out = append(out, "@"+name)
	return append(out, args[len(args)-2:]...), nil
}

// rspQuote quotes a for a response file: in double quotes, with a backslash before a backslash
// or a quote (the rule of gcc and of clang's GNU mode). For a Windows target the path
// separators are written as slashes first, which both parsers read the same.
func rspQuote(a string, windows bool) string {
	if windows {
		a = strings.ReplaceAll(a, `\`, "/")
	}
	if a != "" && !strings.ContainsAny(a, " \t\n\r\"'\\") {
		return a
	}
	a = strings.ReplaceAll(a, `\`, `\\`)
	a = strings.ReplaceAll(a, `"`, `\"`)
	return `"` + a + `"`
}
