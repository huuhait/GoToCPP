package build

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// The archiver (Phase 06 step 6.8, 6.9): the runtime is kept as a static library (rtlib.go), and a
// library of LTO objects needs an archiver that can read them. GNU ar without its plugin writes
// the archive without a symbol index ("plugin needed to handle lto object" is printed, the exit
// status is 0, and the link fails later with "archive has no index" or an undefined reference:
// seen on this machine with both compilers). So the archiver is the one that belongs to the
// compiler:
//
//   - without LTO: the ar of the compiler's family and target ("x86_64-w64-mingw32-ar" for
//     x86_64-w64-mingw32-g++), found next to the compiler first, then in PATH;
//   - with LTO and g++: gcc-ar (a wrapper of ar that loads the LTO plugin), or ar with the plugin that
//     g++ names (-print-file-name=liblto_plugin.so);
//   - with LTO and clang++: llvm-ar (which reads bitcode itself), or GNU ar with the plugin
//     that clang loads at the link (LLVMgold.so, from -print-file-name or from the
//     -plugin argument of the link command that clang -### prints: a Nix wrapper adds it that way).
//
// GOCPP_AR names the archiver (and its arguments) and replaces the search. When none is found,
// a build without LTO links the runtime's object files as before, and one with LTO stops with an
// error that says what was looked for.

// An Archiver is a program that makes a static library: Path, Args (such as --plugin <file> for
// a GNU ar), then "rcsD" (rcs on macOS, whose ar has no D), the library and the members.
type Archiver struct {
	Path string
	Args []string
	Name string // for messages and keys: the kind of archiver ("ar", "gcc-ar", "llvm-ar", "ar+plugin", "GOCPP_AR")
}

// cxx returns the archiver as a command that Builder.run starts.
func (a Archiver) cxx() CXX { return CXX{Path: a.Path, Args: a.Args} }

// identity returns what tells the archiver apart in a cache key: its name, arguments, and the real
// path, size and time of its program file.
func (a Archiver) identity() []string {
	id := []string{"archiver", a.Name, a.Path}
	id = append(id, a.Args...)
	if st, err := stampOf(a.Path); err == nil {
		id = append(id, st.real, fmt.Sprint(st.size), fmt.Sprint(st.modTime))
	}
	return id
}

// archiveFlags are the operation letters of the archiver: replace, create (no message), write a
// symbol index, and (where there is one) zero the times, owners and modes so that the library is
// the same file whenever it is made (the D of GNU ar and llvm-ar; Apple's ar has none).
func archiveFlags(goos string) string {
	if goos == "darwin" || (goos == "" && runtime.GOOS == "darwin") {
		return "rcs"
	}
	return "rcsD"
}

// compilerPrefix returns the target prefix of a compiler's program name ("x86_64-w64-mingw32-" of
// x86_64-w64-mingw32-g++), or "".
func compilerPrefix(c CXX) string {
	base := strings.TrimSuffix(filepath.Base(c.Path), ".exe")
	for _, tail := range []string{"clang++", "g++", "c++", "clang", "gcc", "cc"} {
		if p, ok := strings.CutSuffix(base, tail); ok {
			return p
		}
		// g++-15, clang++-21
		if i := strings.LastIndex(base, tail+"-"); i >= 0 {
			return base[:i]
		}
	}
	return ""
}

// lookTool finds the program name: in the folder of the compiler (as the compiler path says, and
// where its symbolic links lead), then in PATH. It returns "" when there is none.
func lookTool(c CXX, name string) string {
	var dirs []string
	dirs = append(dirs, filepath.Dir(c.Path))
	if real, err := filepath.EvalSymlinks(c.Path); err == nil && filepath.Dir(real) != dirs[0] {
		dirs = append(dirs, filepath.Dir(real))
	}
	for _, dir := range dirs {
		for _, n := range []string{name, name + ".exe"} {
			if p := filepath.Join(dir, n); isExecutable(p) {
				return p
			}
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
	}
	return ""
}

func isExecutable(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular() && (runtime.GOOS == "windows" || fi.Mode().Perm()&0o111 != 0)
}

// FindArchiver returns the archiver for libraries of objects that the compiler cxx made, with LTO
// objects when lto is set (see the comment at the top).
func FindArchiver(ctx context.Context, cxx CXX, lto bool) (Archiver, error) {
	if v := os.Getenv("GOCPP_AR"); v != "" {
		words, err := SplitCommand(v)
		if err != nil || len(words) == 0 {
			return Archiver{}, fmt.Errorf("GOCPP_AR=%q: not a command", v)
		}
		p := words[0]
		if !strings.ContainsAny(p, `/\`) {
			found, err := exec.LookPath(p)
			if err != nil {
				return Archiver{}, fmt.Errorf("GOCPP_AR=%q: %s is not in PATH", v, p)
			}
			p = found
		}
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		if !isExecutable(p) {
			return Archiver{}, fmt.Errorf("GOCPP_AR=%q: %s is not an executable file", v, p)
		}
		return Archiver{Path: p, Args: words[1:], Name: "GOCPP_AR"}, nil
	}
	prefix := compilerPrefix(cxx)
	if !lto {
		if p := lookTool(cxx, prefix+"ar"); p != "" {
			return Archiver{Path: p, Name: "ar"}, nil
		}
		if prefix != "" {
			if p := lookTool(cxx, "ar"); p != "" { // a cross compiler whose binutils have no prefix
				return Archiver{Path: p, Name: "ar"}, nil
			}
		}
		return Archiver{}, fmt.Errorf("no archiver (ar) found next to %s or in PATH", cxx.Path)
	}
	var tried []string
	// A name that only looks like a target prefix ("my-g++", a symbolic link) has no tools of its
	// own: the unprefixed ones are the second try, as for the archiver without LTO.
	withFallback := func(names ...string) []string {
		out := slices.Clone(names)
		if prefix != "" {
			for _, n := range names {
				out = append(out, strings.TrimPrefix(n, prefix))
			}
		}
		return out
	}
	switch cxx.Family {
	case GCC:
		for _, n := range withFallback(prefix+"gcc-ar", prefix+"gcc-ar-"+fmt.Sprint(cxx.Major)) {
			if p := lookTool(cxx, n); p != "" {
				return Archiver{Path: p, Name: "gcc-ar"}, nil
			}
			tried = append(tried, n)
		}
		if plugin := compilerFile(ctx, cxx, "liblto_plugin.so"); plugin != "" {
			for _, n := range withFallback(prefix + "ar") {
				if p := lookTool(cxx, n); p != "" {
					return Archiver{Path: p, Args: []string{"--plugin", plugin}, Name: "ar+plugin"}, nil
				}
			}
		}
		tried = append(tried, "ar with liblto_plugin.so")
	case Clang:
		for _, n := range []string{prefix + "llvm-ar", "llvm-ar", "llvm-ar-" + fmt.Sprint(cxx.Major)} {
			if p := lookTool(cxx, n); p != "" {
				return Archiver{Path: p, Name: "llvm-ar"}, nil
			}
			tried = append(tried, n)
		}
		plugin := compilerFile(ctx, cxx, "LLVMgold.so")
		if plugin == "" {
			plugin = linkPlugin(ctx, cxx)
		}
		if plugin != "" {
			for _, n := range withFallback(prefix + "ar") {
				if p := lookTool(cxx, n); p != "" {
					return Archiver{Path: p, Args: []string{"--plugin", plugin}, Name: "ar+plugin"}, nil
				}
			}
		}
		tried = append(tried, "ar with LLVMgold.so")
	default:
		return Archiver{}, fmt.Errorf("-lto: no LTO archiver is known for the compiler %s", cxx.Path)
	}
	hint := "gcc-ar"
	if cxx.Family == Clang {
		hint = "llvm-ar"
	}
	return Archiver{}, fmt.Errorf("-lto needs an archiver that reads the LTO objects of %s (a plain ar writes a library without an index: \"plugin needed to handle lto object\"); looked for %s next to %s and in PATH: install %s, or set GOCPP_AR",
		cxx.Name(), strings.Join(tried, ", "), cxx.Path, hint)
}

// compilerFile asks the compiler where it keeps the file name (-print-file-name): "" when it
// doesn't have it (it then prints the bare name).
func compilerFile(ctx context.Context, c CXX, name string) string {
	if p := compilerPrint(ctx, c, "-print-file-name="+name); p != "" {
		return p
	}
	// g++ keeps its plugin in libexec, which it finds as a program, not as a library file
	// (-print-file-name prints the bare name for it).
	return compilerPrint(ctx, c, "-print-prog-name="+name)
}

// compilerPrint runs the compiler with a -print-... flag and returns the absolute path that it
// prints, if the file exists.
func compilerPrint(ctx context.Context, c CXX, flag string) string {
	out, err := c.Command(ctx, flag).Output()
	if err != nil {
		return ""
	}
	p := strings.TrimSpace(string(out))
	if filepath.IsAbs(p) {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// linkPlugin returns the plugin that clang passes to the linker for LTO ("-plugin" "<file>" in
// the command that -### prints for a link), or "".
func linkPlugin(ctx context.Context, c CXX) string {
	out, err := c.Command(ctx, "-flto", "-###", "-x", "c++", "-").CombinedOutput()
	_ = err // -### with no input file may fail after printing
	words := strings.Fields(strings.ReplaceAll(string(out), `"`, ""))
	i := slices.Index(words, "-plugin")
	if i < 0 || i+1 >= len(words) || !filepath.IsAbs(words[i+1]) {
		return ""
	}
	if _, err := os.Stat(words[i+1]); err != nil {
		return ""
	}
	return words[i+1]
}

// makeLib makes the library out from the object files members, with the archiver a, in the work
// folder. A failure is a *CompileError (What "the archiver").
func (b *Builder) makeLib(ctx context.Context, a Archiver, out string, members []string) error {
	args := slices.Concat([]string{archiveFlags(b.GOOS), out}, members)
	return b.run(ctx, a.cxx(), "the archiver", false, args)
}
