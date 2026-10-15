// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// gocpp: this file is the flag allow-list of the go command, from Go
// 1.25.14: cmd/go/internal/work/security.go, with splitPkgConfigOutput
// from cmd/go/internal/work/exec.go and SafeArg from cmd/go/internal/load. gocpp checks the flags of
// //gocpp:cflags, //gocpp:ldflags and //gocpp:link and the output of
// pkg-config with it, before any compiler, probe or linker gets them
// (Phase 04 README §2.9, step 4.15; see pkgflags.go). The tables are Go's:
// they already have the C++ flags (-std=, -stdlib=, -f(no-)rtti,
// -f(no-)exceptions, -fsanitize=). The changes:
//   - the system-root flags (--sysroot, --sysroot=, -isysroot) are not in
//     the tables;
//   - checkFlags refuses the flags of refusedFlags (refused.go) first, with
//     the reason in the message: the choices for the whole program (the
//     system root, the toolchain, the linker, the target and the ABI:
//     --sysroot, -fuse-ld=, -m32, -stdlib=, -D_GLIBCXX_DEBUG, ...);
//   - it is in package build, and the regexps are compiled on first use by
//     lazyRegexp (Go uses its internal/lazyregexp);
//   - the overrides are read from GOCPP_<name>_ALLOW and
//     GOCPP_<name>_DISALLOW (Go: CGO_<name>_ALLOW), with os.Getenv;
//   - there are no pkg-config flags: //gocpp:pkg-config takes only package
//     names (load.ParseDirectives), so validPkgConfigFlags,
//     checkPkgConfigFlags and checkCompilerFlagsForInternalLink are left
//     out;
//   - the messages have no "(see https://golang.org/s/invalidflag)";
//   - go/build's makePathsAbsolute is replaced by absPaths (pkgflags.go),
//     which handles every flag with a path that the tables allow.
//
// Upstream comment:
//
// Checking of compiler and linker flags.
// We must avoid flags like -fplugin=, which can allow
// arbitrary code execution during the build.
// Do not make changes here without carefully
// considering the implications.
// (That's why the code is isolated in a file named security.go.)
//
// Note that -Wl,foo means split foo on commas and pass to
// the linker, so that -Wl,-foo,bar means pass -foo bar to
// the linker. Similarly -Wa,foo for the assembler and so on.
// If any of these are permitted, the wildcard portion must
// disallow commas.
//
// Note also that GNU binutils accept any argument @foo
// as meaning "read more flags from the file foo", so we must
// guard against any command-line argument beginning with @,
// even things like "-I @foo".
// We use load.SafeArg (which is even more conservative)
// to reject these.
//
// Even worse, gcc -I@foo (one arg) turns into cc1 -I @foo (two args),
// so although gcc doesn't expand the @foo, cc1 will.
// So out of paranoia, we reject @ at the beginning of every
// flag argument that might be split into its own argument.

package build

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

// A lazyRegexp is a regexp that is compiled when it is first used, like
// Go's internal/lazyregexp: most runs of gocpp check no flags.
type lazyRegexp struct {
	str  string
	once sync.Once
	rx   *regexp.Regexp
}

func (r *lazyRegexp) FindString(s string) string {
	r.once.Do(func() { r.rx = regexp.MustCompile(r.str) })
	return r.rx.FindString(s)
}

func re(str string) *lazyRegexp { return &lazyRegexp{str: str} }

var validCompilerFlags = []*lazyRegexp{
	re(`-D([A-Za-z_][A-Za-z0-9_]*)(=[^@\-]*)?`),
	re(`-U([A-Za-z_][A-Za-z0-9_]*)`),
	re(`-F([^@\-].*)`),
	re(`-I([^@\-].*)`),
	re(`-O`),
	re(`-O([^@\-].*)`),
	re(`-W`),
	re(`-W([^@,]+)`), // -Wall but not -Wa,-foo.
	re(`-Wa,-mbig-obj`),
	re(`-Wp,-D([A-Za-z_][A-Za-z0-9_]*)(=[^@,\-]*)?`),
	re(`-Wp,-U([A-Za-z_][A-Za-z0-9_]*)`),
	re(`-ansi`),
	re(`-f(no-)?asynchronous-unwind-tables`),
	re(`-f(no-)?blocks`),
	re(`-f(no-)builtin-[a-zA-Z0-9_]*`),
	re(`-f(no-)?common`),
	re(`-f(no-)?constant-cfstrings`),
	re(`-fdebug-prefix-map=([^@]+)=([^@]+)`),
	re(`-fdiagnostics-show-note-include-stack`),
	re(`-ffile-prefix-map=([^@]+)=([^@]+)`),
	re(`-fno-canonical-system-headers`),
	re(`-f(no-)?eliminate-unused-debug-types`),
	re(`-f(no-)?exceptions`),
	re(`-f(no-)?fast-math`),
	re(`-f(no-)?inline-functions`),
	re(`-finput-charset=([^@\-].*)`),
	re(`-f(no-)?fat-lto-objects`),
	re(`-f(no-)?keep-inline-dllexport`),
	re(`-f(no-)?lto`),
	re(`-fmacro-backtrace-limit=(.+)`),
	re(`-fmessage-length=(.+)`),
	re(`-f(no-)?modules`),
	re(`-f(no-)?objc-arc`),
	re(`-f(no-)?objc-nonfragile-abi`),
	re(`-f(no-)?objc-legacy-dispatch`),
	re(`-f(no-)?omit-frame-pointer`),
	re(`-f(no-)?openmp(-simd)?`),
	re(`-f(no-)?permissive`),
	re(`-f(no-)?(pic|PIC|pie|PIE)`),
	re(`-f(no-)?plt`),
	re(`-f(no-)?rtti`),
	re(`-f(no-)?split-stack`),
	re(`-f(no-)?stack-(.+)`),
	re(`-f(no-)?strict-aliasing`),
	re(`-f(un)signed-char`),
	re(`-f(no-)?use-linker-plugin`), // safe if -B is not used; we don't permit -B
	re(`-f(no-)?visibility-inlines-hidden`),
	re(`-fsanitize=(.+)`),
	re(`-fsanitize-undefined-strip-path-components=(-)?[0-9]+`),
	re(`-ftemplate-depth-(.+)`),
	re(`-ftls-model=(global-dynamic|local-dynamic|initial-exec|local-exec)`),
	re(`-fvisibility=(.+)`),
	re(`-g([^@\-].*)?`),
	re(`-m32`),
	re(`-m64`),
	re(`-m(abi|arch|cpu|fpu|simd|tls-dialect|tune)=([^@\-].*)`),
	re(`-m(no-)?v?aes`),
	re(`-marm`),
	re(`-m(no-)?avx[0-9a-z]*`),
	re(`-mcmodel=[0-9a-z-]+`),
	re(`-mfloat-abi=([^@\-].*)`),
	re(`-m(soft|single|double)-float`),
	re(`-mfpmath=[0-9a-z,+]*`),
	re(`-m(no-)?avx[0-9a-z.]*`),
	re(`-m(no-)?ms-bitfields`),
	re(`-m(no-)?stack-(.+)`),
	re(`-mmacosx-(.+)`),
	re(`-m(no-)?relax`),
	re(`-m(no-)?strict-align`),
	re(`-m(no-)?(lsx|lasx|frecipe|div32|lam-bh|lamcas|ld-seq-sa)`),
	re(`-mios-simulator-version-min=(.+)`),
	re(`-miphoneos-version-min=(.+)`),
	re(`-mlarge-data-threshold=[0-9]+`),
	re(`-mtvos-simulator-version-min=(.+)`),
	re(`-mtvos-version-min=(.+)`),
	re(`-mwatchos-simulator-version-min=(.+)`),
	re(`-mwatchos-version-min=(.+)`),
	re(`-mnop-fun-dllimport`),
	re(`-m(no-)?sse[0-9.]*`),
	re(`-m(no-)?ssse3`),
	re(`-mthumb(-interwork)?`),
	re(`-mthreads`),
	re(`-mwindows`),
	re(`-no-canonical-prefixes`),
	re(`--param=ssp-buffer-size=[0-9]*`),
	re(`-pedantic(-errors)?`),
	re(`-pipe`),
	re(`-pthread`),
	re(`--static`),
	re(`-?-std=([^@\-].*)`),
	re(`-?-stdlib=([^@\-].*)`),
	re(`-w`),
	re(`-x([^@\-].*)`),
	re(`-v`),
}

var validCompilerFlagsWithNextArg = []string{
	"-arch",
	"-D",
	"-U",
	"-I",
	"-F",
	"-framework",
	"-include",
	"-isystem",
	"-target",
	"-x",
}

var invalidLinkerFlags = []*lazyRegexp{
	// On macOS this means the linker loads and executes the next argument.
	// Have to exclude separately because -lfoo is allowed in general.
	re(`-lto_library`),
}

var validLinkerFlags = []*lazyRegexp{
	re(`-F([^@\-].*)`),
	re(`-l([^@\-].*)`),
	re(`-L([^@\-].*)`),
	re(`-O`),
	re(`-O([^@\-].*)`),
	re(`-f(no-)?(pic|PIC|pie|PIE)`),
	re(`-f(no-)?openmp(-simd)?`),
	re(`-fsanitize=([^@\-].*)`),
	re(`-flat_namespace`),
	re(`-g([^@\-].*)?`),
	re(`-headerpad_max_install_names`),
	re(`-m(abi|arch|cpu|fpu|simd|tls-dialect|tune)=([^@\-].*)`),
	re(`-mcmodel=[0-9a-z-]+`),
	re(`-mfloat-abi=([^@\-].*)`),
	re(`-m(soft|single|double)-float`),
	re(`-m(no-)?relax`),
	re(`-m(no-)?strict-align`),
	re(`-m(no-)?(lsx|lasx|frecipe|div32|lam-bh|lamcas|ld-seq-sa)`),
	re(`-mmacosx-(.+)`),
	re(`-mios-simulator-version-min=(.+)`),
	re(`-miphoneos-version-min=(.+)`),
	re(`-mthreads`),
	re(`-mwindows`),
	re(`-(pic|PIC|pie|PIE)`),
	re(`-pthread`),
	re(`-rdynamic`),
	re(`-shared`),
	re(`-?-static([-a-z0-9+]*)`),
	re(`-?-stdlib=([^@\-].*)`),
	re(`-v`),

	// Note that any wildcards in -Wl need to exclude comma,
	// since -Wl splits its argument at commas and passes
	// them all to the linker uninterpreted. Allowing comma
	// in a wildcard would allow tunneling arbitrary additional
	// linker arguments through one of these.
	re(`-Wl,--(no-)?allow-multiple-definition`),
	re(`-Wl,--(no-)?allow-shlib-undefined`),
	re(`-Wl,--(no-)?as-needed`),
	re(`-Wl,-Bdynamic`),
	re(`-Wl,-berok`),
	re(`-Wl,-Bstatic`),
	re(`-Wl,-Bsymbolic-functions`),
	re(`-Wl,-O[0-9]+`),
	re(`-Wl,-d[ny]`),
	re(`-Wl,--disable-new-dtags`),
	re(`-Wl,-e[=,][a-zA-Z0-9]+`),
	re(`-Wl,--enable-new-dtags`),
	re(`-Wl,--end-group`),
	re(`-Wl,--(no-)?export-dynamic`),
	re(`-Wl,-E`),
	re(`-Wl,-framework,[^,@\-][^,]*`),
	re(`-Wl,--hash-style=(sysv|gnu|both)`),
	re(`-Wl,-headerpad_max_install_names`),
	re(`-Wl,--no-undefined`),
	re(`-Wl,--pop-state`),
	re(`-Wl,--push-state`),
	re(`-Wl,-R,?([^@\-,][^,@]*$)`),
	re(`-Wl,--just-symbols[=,]([^,@\-][^,@]*)`),
	re(`-Wl,-rpath(-link)?[=,]([^,@\-][^,]*)`),
	re(`-Wl,-s`),
	re(`-Wl,-search_paths_first`),
	re(`-Wl,-sectcreate,([^,@\-][^,]*),([^,@\-][^,]*),([^,@\-][^,]*)`),
	re(`-Wl,--start-group`),
	re(`-Wl,-?-static`),
	re(`-Wl,-?-subsystem,(native|windows|console|posix|xbox)`),
	re(`-Wl,-syslibroot[=,]([^,@\-][^,]*)`),
	re(`-Wl,-undefined[=,]([^,@\-][^,]*)`),
	re(`-Wl,-?-unresolved-symbols=[^,]+`),
	re(`-Wl,--(no-)?warn-([^,]+)`),
	re(`-Wl,-?-wrap[=,][^,@\-][^,]*`),
	re(`-Wl(,-z,(relro|now|(no)?execstack))+`),

	re(`[a-zA-Z0-9_/].*\.(a|o|obj|dll|dylib|so|tbd)`), // direct linker inputs: x.o or libfoo.so (but not -foo.o or @foo.o)
	re(`\./.*\.(a|o|obj|dll|dylib|so|tbd)`),
}

var validLinkerFlagsWithNextArg = []string{
	"-arch",
	"-F",
	"-l",
	"-L",
	"-framework",
	"-target",
	"-Wl,-framework",
	"-Wl,-rpath",
	"-Wl,-R",
	"-Wl,--just-symbols",
	"-Wl,-undefined",
}

func checkCompilerFlags(name, source string, list []string) error {
	checkOverrides := true
	return checkFlags(name, source, list, nil, validCompilerFlags, validCompilerFlagsWithNextArg, checkOverrides)
}

func checkLinkerFlags(name, source string, list []string) error {
	checkOverrides := true
	return checkFlags(name, source, list, invalidLinkerFlags, validLinkerFlags, validLinkerFlagsWithNextArg, checkOverrides)
}

func checkFlags(name, source string, list []string, invalid, valid []*lazyRegexp, validNext []string, checkOverrides bool) error {
	// Let users override rules with $GOCPP_CFLAGS_ALLOW, $GOCPP_CFLAGS_DISALLOW, etc.
	var (
		allow    *regexp.Regexp
		disallow *regexp.Regexp
	)
	if checkOverrides {
		if env := os.Getenv("GOCPP_" + name + "_ALLOW"); env != "" {
			r, err := regexp.Compile(env)
			if err != nil {
				return fmt.Errorf("parsing $GOCPP_%s_ALLOW: %v", name, err)
			}
			allow = r
		}
		if env := os.Getenv("GOCPP_" + name + "_DISALLOW"); env != "" {
			r, err := regexp.Compile(env)
			if err != nil {
				return fmt.Errorf("parsing $GOCPP_%s_DISALLOW: %v", name, err)
			}
			disallow = r
		}
	}

Args:
	for i := 0; i < len(list); i++ {
		arg := list[i]
		if disallow != nil && disallow.FindString(arg) == arg {
			goto Bad
		}
		if allow != nil && allow.FindString(arg) == arg {
			continue Args
		}
		// gocpp: the flags that gocpp refuses, with the reason.
		if shown, why := refused(list, i); why != "" && (allow == nil || allow.FindString(shown) != shown) {
			return fmt.Errorf("invalid flag in %s: %s (%s; $GOCPP_%s_ALLOW can allow it)", source, shown, why, name)
		}
		for _, re := range invalid {
			if re.FindString(arg) == arg { // must be complete match
				goto Bad
			}
		}
		for _, re := range valid {
			if match := re.FindString(arg); match == arg { // must be complete match
				continue Args
			} else if strings.HasPrefix(arg, "-Wl,--push-state,") {
				// Examples for --push-state are written
				//     -Wl,--push-state,--as-needed
				// Support other commands in the same -Wl arg.
				args := strings.Split(arg, ",")
				for _, a := range args[1:] {
					a = "-Wl," + a
					var found bool
					for _, re := range valid {
						if re.FindString(a) == a {
							found = true
							break
						}
					}
					if !found {
						goto Bad
					}
					for _, re := range invalid {
						if re.FindString(a) == a {
							goto Bad
						}
					}
				}
				continue Args
			}
		}
		for _, x := range validNext {
			if arg == x {
				if i+1 < len(list) && safeArg(list[i+1]) {
					i++
					continue Args
				}

				// Permit -Wl,-framework -Wl,name.
				if i+1 < len(list) &&
					strings.HasPrefix(arg, "-Wl,") &&
					strings.HasPrefix(list[i+1], "-Wl,") &&
					safeArg(list[i+1][4:]) &&
					!strings.Contains(list[i+1][4:], ",") {
					i++
					continue Args
				}

				// Permit -I= /path, -I $SYSROOT.
				if i+1 < len(list) && arg == "-I" {
					if (strings.HasPrefix(list[i+1], "=") || strings.HasPrefix(list[i+1], "$SYSROOT")) &&
						safeArg(list[i+1][1:]) {
						i++
						continue Args
					}
				}

				if i+1 < len(list) {
					return fmt.Errorf("invalid flag in %s: %s %s", source, arg, list[i+1])
				}
				return fmt.Errorf("invalid flag in %s: %s without argument", source, arg)
			}
		}
	Bad:
		return fmt.Errorf("invalid flag in %s: %s", source, arg)
	}
	return nil
}

// safeArg reports whether name is a safe argument (Go's load.SafeArg):
// not a flag (-x) and not a response file (@x), whatever comes after its
// first character.
func safeArg(name string) bool {
	if name == "" {
		return false
	}
	c := name[0]
	return '0' <= c && c <= '9' || 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || c == '.' || c == '_' || c == '/' || c >= utf8.RuneSelf
}

// splitPkgConfigOutput parses the pkg-config output into a slice of flags.
// This implements the shell quoting semantics described in
// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_02,
// except that it does not support parameter or arithmetic expansion or command
// substitution and hard-codes the <blank> delimiters instead of reading them
// from LC_LOCALE.
func splitPkgConfigOutput(out []byte) ([]string, error) {
	if len(out) == 0 {
		return nil, nil
	}
	var flags []string
	flag := make([]byte, 0, len(out))
	didQuote := false // was the current flag parsed from a quoted string?
	escaped := false  // did we just read `\` in a non-single-quoted context?
	quote := byte(0)  // what is the quote character around the current string?

	for _, c := range out {
		if escaped {
			if quote == '"' {
				// “The <backslash> shall retain its special meaning as an escape
				// character … only when followed by one of the following characters
				// when considered special:”
				switch c {
				case '$', '`', '"', '\\', '\n':
					// Handle the escaped character normally.
				default:
					// Not an escape character after all.
					flag = append(flag, '\\', c)
					escaped = false
					continue
				}
			}

			if c == '\n' {
				// “If a <newline> follows the <backslash>, the shell shall interpret
				// this as line continuation.”
			} else {
				flag = append(flag, c)
			}
			escaped = false
			continue
		}

		if quote != 0 && c == quote {
			quote = 0
			continue
		}
		switch quote {
		case '\'':
			// “preserve the literal value of each character”
			flag = append(flag, c)
			continue
		case '"':
			// “preserve the literal value of all characters within the double-quotes,
			// with the exception of …”
			switch c {
			case '`', '$', '\\':
			default:
				flag = append(flag, c)
				continue
			}
		}

		// “The application shall quote the following characters if they are to
		// represent themselves:”
		switch c {
		case '|', '&', ';', '<', '>', '(', ')', '$', '`':
			return nil, fmt.Errorf("unexpected shell character %q in pkgconf output", c)

		case '\\':
			// “A <backslash> that is not quoted shall preserve the literal value of
			// the following character, with the exception of a <newline>.”
			escaped = true
			continue

		case '"', '\'':
			quote = c
			didQuote = true
			continue

		case ' ', '\t', '\n':
			if len(flag) > 0 || didQuote {
				flags = append(flags, string(flag))
			}
			flag, didQuote = flag[:0], false
			continue
		}

		flag = append(flag, c)
	}

	// Prefer to report a missing quote instead of a missing escape. If the string
	// is something like `"foo\`, it's ambiguous as to whether the trailing
	// backslash is really an escape at all.
	if quote != 0 {
		return nil, errors.New("unterminated quoted string in pkgconf output")
	}
	if escaped {
		return nil, errors.New("broken character escaping in pkgconf output")
	}

	if len(flag) > 0 || didQuote {
		flags = append(flags, string(flag))
	}
	return flags, nil
}
