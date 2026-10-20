package build

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// The flags that the user gives to the whole build (Phase 06 step 6.12, ADR-044).
//
// A package's //gocpp: directives go through the allow-list (security.go); the user's own flags
// do not: the user chose them. They come from three places, in this order (a later flag wins where
// the last one wins):
//
//   - the mode: -race (ThreadSanitizer), -release (-O2 -flto);
//   - the environment: GOCPP_CFLAGS (every C and C++ compile), GOCPP_CXXFLAGS (every C++ compile)
//     and GOCPP_LDFLAGS (the link);
//   - the build flags: -cflags, -cxxflags and -ldflags of gocpp build, which add to the environment's.
//
// All of them go to every compile of the program, the runtime's too (ReadEnvFlags), so they are
// the way to load a compiler plugin: -cxxflags=-fpass-plugin=x.so (clang's LLVM pass plugins),
// -fplugin=x.so (g++'s and clang's front-end plugins). A plugin is code that runs in the compiler,
// so it is never taken from a package's directives (Phase 04's allow-list refuses -fplugin there).

// Layered returns e with the flags of the mode and of the build flags added around it: the
// mode's first, then the environment's (e), then the build flags' (cflags, cxxflags, ldflags).
func (e EnvFlags) Layered(modeCompile, modeLink, cflags, cxxflags, ldflags []string) EnvFlags {
	return EnvFlags{
		CFlags:   slices.Concat(modeCompile, e.CFlags, cflags),
		CXXFlags: slices.Concat(e.CXXFlags, cxxflags),
		LDFlags:  slices.Concat(modeLink, e.LDFlags, ldflags),
	}
}

// A Profile is the kind of build that the flags -debug, -release and -race ask for.
type Profile struct {
	Debug   bool // -debug: -O0 -g, which is what a build is without a flag
	Release bool // -release: -O2 -flto
	Race    bool // -race: ThreadSanitizer
}

// ProfileFlags returns the compile flags and the link flags of the mode for the
// operating system goos, or an error for a combination that does not make sense: -debug with
// -release, -race with -release (a race run wants the code as written, not as -O2 -flto
// reorders it), -race with -asan (the compilers refuse both sanitizers in one program), and
// -race on a system without ThreadSanitizer, and -asan for windows. On linux the race mode adds Go's report (a reporter
// thread in the runtime's OS layer, which only linux has); darwin gets ThreadSanitizer's own text.
func ProfileFlags(m Profile, asan bool, goos string) (compile, link []string, err error) {
	switch {
	case m.Debug && m.Release:
		return nil, nil, errors.New("-debug and -release can't be used together")
	case m.Race && asan:
		return nil, nil, errors.New("-race and -asan can't be used together (ThreadSanitizer and AddressSanitizer don't work in one program)")
	case m.Race && m.Release:
		return nil, nil, errors.New("-race and -release can't be used together")
	case m.Race && goos == "windows":
		return nil, nil, errors.New("-race is not supported for windows (the MinGW toolchains have no ThreadSanitizer)")
	case asan && goos == "windows":
		return nil, nil, errors.New("-asan is not supported for windows (the MinGW GCC of WinLibs, MSYS2 and the dev shell has no libsanitizer; clang's MinGW target has an AddressSanitizer that gocpp has not tried). Run the sanitizers on Linux")
	}
	if m.Race {
		compile = []string{"-fsanitize=thread", "-fno-omit-frame-pointer"}
		link = []string{"-fsanitize=thread"}
		if goos == "linux" {
			// Go's report, exit code 66 and GORACE (Phase 08 step 8.12, docs/guides/race.md): the
			// define switches the runtime to the race mode (gocpp/race.hpp), and gocpp/nofortify.h,
			// which every compile of the program and of the runtime includes first, then includes
			// gocpp/race_atomicity.hpp. The link exports the function that carries GORACE to TSan.
			compile = append(compile, "-DGOCPP_RACE_MODE=1")
			link = append(link, "-Wl,--export-dynamic-symbol=__tsan_default_options")
		}
	}
	if m.Release {
		compile = []string{"-O2", "-flto"}
		link = []string{"-O2", "-flto"}
	}
	return compile, link, nil
}

// pluginFlagFile returns the plugin file that the compiler flag f loads, and whether f loads one:
// -fplugin=file (g++ and clang) and -fpass-plugin=file (clang's LLVM pass plugins).
func pluginFlagFile(f string) (string, bool) {
	for _, p := range []string{"-fplugin=", "-fpass-plugin="} {
		if rest, ok := strings.CutPrefix(f, p); ok {
			return rest, true
		}
	}
	return "", false
}

// xclangPlugin reports whether the word w, the one after -Xclang, belongs to a plugin of clang's
// front end: -load <file>, -add-plugin <name>, -plugin <name> and -plugin-arg-<name> <arg>. Each
// of them is followed by its value as another -Xclang pair.
func xclangPlugin(w string) bool {
	return w == "-load" || w == "-add-plugin" || w == "-plugin" || strings.HasPrefix(w, "-plugin-arg-")
}

// IsPluginFlag reports whether the flag f (alone) loads or configures a compiler plugin:
// -fplugin=, -fplugin-arg-, -fpass-plugin=. The pair forms of clang (-Xclang -load -Xclang file)
// are seen by DropPluginFlags and PluginFiles, which look at two words.
func IsPluginFlag(f string) bool {
	return strings.HasPrefix(f, "-fplugin=") || strings.HasPrefix(f, "-fplugin-arg-") || strings.HasPrefix(f, "-fpass-plugin=")
}

// DropPluginFlags returns flags without the flags that load or configure compiler plugins (the
// forms of IsPluginFlag, and clang's -Xclang -load -Xclang <file> and the like): the probes of the
// C/C++ headers must not load a plugin, which the user meant for the program's compiles (a plugin
// that counts functions would count the probe files too, and the probes' cache would not know
// the plugin's file).
func DropPluginFlags(flags []string) []string {
	if !slices.ContainsFunc(flags, func(f string) bool { return IsPluginFlag(f) || f == "-Xclang" }) {
		return flags
	}
	out := make([]string, 0, len(flags))
	for i := 0; i < len(flags); i++ {
		f := flags[i]
		switch {
		case IsPluginFlag(f):
		case f == "-Xclang" && i+1 < len(flags) && xclangPlugin(flags[i+1]):
			i++ // the word of the plugin; its value is the next -Xclang pair, which goes the same way
			if i+2 < len(flags) && flags[i+1] == "-Xclang" {
				i += 2
			}
		default:
			out = append(out, f)
		}
	}
	return out
}

// PluginFiles returns the plugin files that flags load, in order: those of -fplugin=,
// -fpass-plugin= and clang's -Xclang -load -Xclang <file>.
func PluginFiles(flags []string) []string {
	var files []string
	for i := 0; i < len(flags); i++ {
		if f, ok := pluginFlagFile(flags[i]); ok {
			files = append(files, f)
			continue
		}
		if flags[i] == "-Xclang" && i+3 < len(flags) && flags[i+1] == "-load" && flags[i+2] == "-Xclang" {
			files = append(files, flags[i+3])
			i += 3
		}
	}
	return files
}

// flagFileArg reports whether the flag word f takes its file from the next word (-include,
// -imacros), so that the file is an input of the compile.
func flagFileArg(f string) bool {
	return f == "-include" || f == "-imacros"
}

// FlagFiles returns the files that the flags name as inputs of the compile other than plugins: a
// response file (@file, whose content is more flags), and -include / -imacros files. The compiler
// reads them, so a cache key has their content (the object of a compile that follows -MD knows
// the -include files, but not a response file, and the runtime's keys know neither). The files
// that a response file names in turn are not followed.
func FlagFiles(flags []string) []string {
	var files []string
	for i := 0; i < len(flags); i++ {
		switch f := flags[i]; {
		case len(f) > 1 && f[0] == '@':
			files = append(files, f[1:])
		case flagFileArg(f) && i+1 < len(flags):
			i++
			files = append(files, flags[i])
		}
	}
	return files
}

// AbsPluginPaths returns flags with the relative paths of -fplugin= and -fpass-plugin= made
// absolute from dir: the compiles run in the work folder, where a path relative to the user's
// folder would name nothing. A plugin that is a bare name (g++ looks -fplugin=name up in its
// plugin folder) stays as it is. The same goes for response files (@file) and the files of
// -include and -imacros that exist under dir (another name is searched in the include paths, so
// it stays).
func AbsPluginPaths(flags []string, dir string) []string {
	var out []string
	set := func(i int, v string) {
		if out == nil {
			out = slices.Clone(flags)
		}
		out[i] = v
	}
	here := func(name string) (string, bool) {
		if name == "" || filepath.IsAbs(name) {
			return "", false
		}
		p := filepath.Join(dir, name)
		fi, err := os.Stat(p)
		return p, err == nil && fi.Mode().IsRegular()
	}
	for i, f := range flags {
		switch {
		case len(f) > 1 && f[0] == '@':
			if p, ok := here(f[1:]); ok {
				set(i, "@"+p)
			}
			continue
		case i > 0 && flagFileArg(flags[i-1]):
			if p, ok := here(f); ok {
				set(i, p)
			}
			continue
		}
		for _, p := range []string{"-fplugin=", "-fpass-plugin="} {
			rest, ok := strings.CutPrefix(f, p)
			if !ok || filepath.IsAbs(rest) || !strings.ContainsAny(rest, `/\`) {
				continue
			}
			set(i, p+filepath.Join(dir, rest))
		}
	}
	if out == nil {
		return flags
	}
	return out
}

// pathFlagsSeparate are the flags whose value is the next word and names a folder (or a file), and
// pathFlagsJoined those that can have it joined to them (-Iinc, -Linc) or after an equals sign.
var (
	pathFlagsSeparate = []string{"-I", "-L", "-isystem", "-iquote", "-idirafter", "-isysroot", "-iprefix", "-iwithprefix", "-iwithprefixbefore", "-imultilib"}
	pathFlagsJoined   = []string{"-I", "-L"}
	pathFlagsEquals   = []string{"--sysroot=", "-isysroot=", "--gcc-toolchain=", "--gcc-install-dir=", "-fprofile-sample-use="}
)

// AbsUserPaths returns flags (the user's -cflags, -cxxflags, -ldflags or GOCPP_* variable) with the
// relative paths in them made absolute from dir, the user's folder: the compiles and the link run
// in gocpp's private work folder, where a name relative to the user's folder names nothing, or
// (-Wl,-Map,x.map) writes into a folder that is deleted. These are the folders of -I, -L,
// -isystem, -iquote, -idirafter and --sysroot=, the output of -Wl,-Map, and what AbsPluginPaths
// handles (plugins, @file, -include, -imacros). A library or an object file named in the flags that
// exists under dir is made absolute too. Not made absolute: -Wl,-rpath (a path for the program's
// run), -l names, and -I=dir (relative to the sysroot).
func AbsUserPaths(flags []string, dir string) []string {
	flags = AbsPluginPaths(flags, dir)
	var out []string
	set := func(i int, v string) {
		if out == nil {
			out = slices.Clone(flags)
		}
		out[i] = v
	}
	abs := func(name string) (string, bool) {
		if name == "" || filepath.IsAbs(name) || strings.HasPrefix(name, "=") || strings.HasPrefix(name, "$") || name == "-" {
			return "", false
		}
		return filepath.Join(dir, name), true
	}
	for i, f := range flags {
		switch {
		case i > 0 && slices.Contains(pathFlagsSeparate, flags[i-1]):
			if p, ok := abs(f); ok {
				set(i, p)
			}
			continue
		case strings.HasPrefix(f, "-Wl,"):
			parts := strings.Split(f[4:], ",")
			changed := false
			for j, p := range parts {
				switch {
				case j > 0 && (parts[j-1] == "-Map" || parts[j-1] == "--Map"):
					if a, ok := abs(p); ok {
						parts[j], changed = a, true
					}
				case strings.HasPrefix(p, "-Map=") || strings.HasPrefix(p, "--Map="):
					k := strings.Index(p, "=") + 1
					if a, ok := abs(p[k:]); ok {
						parts[j], changed = p[:k]+a, true
					}
				}
			}
			if changed {
				set(i, "-Wl,"+strings.Join(parts, ","))
			}
			continue
		case strings.HasPrefix(f, "-"):
		default:
			// A library or an object file named by a relative path.
			switch filepath.Ext(f) {
			case ".a", ".o", ".so", ".lib", ".dylib":
				if p, ok := abs(f); ok {
					if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
						set(i, p)
					}
				}
			}
			continue
		}
		done := false
		for _, e := range pathFlagsEquals {
			if rest, ok := strings.CutPrefix(f, e); ok {
				if p, ok := abs(rest); ok {
					set(i, e+p)
				}
				done = true
				break
			}
		}
		if done {
			continue
		}
		for _, j := range pathFlagsJoined {
			if rest, ok := strings.CutPrefix(f, j); ok && rest != "" && rest != "-" {
				if p, ok := abs(rest); ok {
					set(i, j+p)
				}
				break
			}
		}
	}
	if out == nil {
		return flags
	}
	return out
}

// pluginHashes remembers the content hash of the plugin files by their stat, so that a build with
// a plugin in its flags hashes a big shared object once.
var pluginHashes sync.Map // statKey -> string

type pluginStat struct {
	path string
	size int64
	mod  int64
}

// PluginHash returns a hash of the content of the plugin file path, or "missing" when it can't
// be read: the cache keys of the compiles that load it have it, so a plugin that is built again
// (the same flag, another file) never gets an object that the old one made.
func PluginHash(path string) string {
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return "missing"
	}
	k := pluginStat{path, fi.Size(), fi.ModTime().UnixNano()}
	if h, ok := pluginHashes.Load(k); ok {
		return h.(string)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "missing"
	}
	sum := sha256.Sum256(data)
	h := hex.EncodeToString(sum[:])
	pluginHashes.Store(k, h)
	return h
}

// PluginSalt returns the entries of a cache key for the files that flags make the compiler read
// besides the sources: the plugins they load and the FlagFiles (response files, -include files).
// The file name and the hash of its content are one entry each; nil without such a file.
func PluginSalt(flags []string) []string {
	var salt []string
	for _, f := range PluginFiles(flags) {
		salt = append(salt, "plugin", f, PluginHash(f))
	}
	for _, f := range FlagFiles(flags) {
		salt = append(salt, "flagfile", f, PluginHash(f))
	}
	return salt
}

// PluginStamp returns a text for the key of the up-to-date check that changes when a plugin file
// that flags load changes (its name, size and time), so that a plugin that is built again makes
// the next build run: the stamp records the compiles' headers, not the compilers' plugins.
func PluginStamp(flags []string) []string {
	var parts []string
	for _, f := range slices.Concat(PluginFiles(flags), FlagFiles(flags)) {
		fi, err := os.Stat(f)
		if err != nil {
			parts = append(parts, "plugin "+f+" missing")
			continue
		}
		parts = append(parts, fmt.Sprintf("plugin %s %d %d", f, fi.Size(), fi.ModTime().UnixNano()))
	}
	return parts
}
