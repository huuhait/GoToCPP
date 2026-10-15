package build

import "strings"

// The flags that gocpp refuses in a package's flags (//gocpp:cflags,
// ldflags, link and the output of pkg-config), on top of the go command's
// allow-list (security.go), each with the reason that the error message
// gives: the choices for the whole program (the system root, the toolchain,
// the linker, the target, the ABI and the C++ standard library). GOCPP_CFLAGS_ALLOW and GOCPP_LDFLAGS_ALLOW can still allow them,
// and the flags of the environment are never checked.

// A refusedFlag is a flag (a regexp that must match it all) that gocpp
// refuses, and why.
type refusedFlag struct {
	rx  *lazyRegexp
	why string
}

const (
	// whyWholeProgram is the reason for the flags that choose the system
	// root or the toolchain, and whyLinker for those that choose the
	// linker: choices for the whole program, which belong to the user.
	whyWholeProgram = "a package can't choose the system root or the toolchain: they are choices for the whole program; set them in GOCPP_CFLAGS or GOCPP_LDFLAGS"
	whyLinker       = "a package can't choose the linker: it is a choice for the whole program; set it in GOCPP_LDFLAGS"
	// whyABI is the reason for the flags that change the ABI.
	whyABI = "it changes the ABI, and the package's code would no longer match gocpp's runtime"
	// whyStdlib is the reason for -stdlib=.
	whyStdlib = "it selects another C++ standard library than the one gocpp's runtime is built with"
)

// refusedFlags are the flags that gocpp refuses. A -D or -U flag is matched
// with its name also when the name is the next argument (-D NAME).
var refusedFlags = []refusedFlag{
	// The system root, the toolchain and the linker: choices for the whole
	// program.
	{re(`--sysroot(=.*)?`), whyWholeProgram},
	{re(`-isysroot.*`), whyWholeProgram},
	{re(`--?gcc-toolchain(=.*)?`), whyWholeProgram},
	{re(`--gcc-install-dir(=.*)?`), whyWholeProgram},
	{re(`--prefix(=.*)?`), whyWholeProgram},
	{re(`-B.*`), whyWholeProgram},
	{re(`--ld-path=.*`), whyLinker},
	{re(`-fuse-ld=.*`), whyLinker},
	{re(`-ccc-.*`), whyWholeProgram},

	// Another target or ABI: the package's objects wouldn't link with the
	// runtime's, or would see its types with another layout.
	{re(`-m32`), whyABI},
	{re(`-m64`), whyABI},
	{re(`-mx32`), whyABI},
	{re(`-target`), whyABI},
	{re(`--target=.*`), whyABI},
	{re(`-arch`), whyABI},
	{re(`-mabi=.*`), whyABI},
	{re(`-mfloat-abi=.*`), whyABI},
	{re(`-m(soft|single|double)-float`), whyABI},
	{re(`-m(no-)?ms-bitfields`), whyABI},
	{re(`-?-stdlib=.*`), whyStdlib},

	// Macros that change the ABI of the C++ standard library: the debug
	// containers of libstdc++ (_GLIBCXX_DEBUG, not _GLIBCXX_ASSERTIONS,
	// which only adds checks), its old string ABI, and libc++'s ABI
	// settings. Other macros (-D_REENTRANT, -D_FILE_OFFSET_BITS=64) are
	// allowed.
	{re(`(-Wp,)?-[DU]_GLIBCXX_DEBUG(_PEDANTIC)?(=.*)?`), whyABI},
	{re(`(-Wp,)?-[DU]_GLIBCXX_USE_CXX11_ABI(=.*)?`), whyABI},
	{re(`(-Wp,)?-[DU]_LIBCPP_ABI_[A-Za-z0-9_]*(=.*)?`), whyABI},
	{re(`(-Wp,)?-[DU]_LIBCPP_DEBUG(=.*)?`), whyABI},
}

// refused reports whether gocpp refuses list[i] (see refusedFlags): it
// returns the flag as the message shows it (with the next argument for -D
// NAME and -U NAME) and the reason, or "" when the flag is not refused.
func refused(list []string, i int) (shown, why string) {
	shown = list[i]
	if (shown == "-D" || shown == "-U") && i+1 < len(list) {
		shown += list[i+1]
	}
	for _, r := range refusedFlags {
		if r.rx.FindString(shown) == shown {
			return shown, r.why
		}
	}
	// -Wp,-DX,-DY: each part.
	if rest, ok := strings.CutPrefix(shown, "-Wp,"); ok {
		for part := range strings.SplitSeq(rest, ",") {
			for _, r := range refusedFlags {
				if r.rx.FindString(part) == part {
					return shown, r.why
				}
			}
		}
	}
	return shown, ""
}
