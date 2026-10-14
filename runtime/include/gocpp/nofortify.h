// gocpp/nofortify.h: turns _FORTIFY_SOURCE off when the compiler doesn't optimize.
//
// A Nix shell's compiler wrapper turns on "hardening" by default and adds -D_FORTIFY_SOURCE
// after the user's flags, so -U_FORTIFY_SOURCE doesn't help. glibc then warns at every file
// built with -O0 ("_FORTIFY_SOURCE requires compiling with optimization"), and -Werror makes
// that an error. Fortify checks need optimization anyway, so at -O0 they do nothing.
//
// This file must come before every system header: gocpp/rt.hpp includes it first, the
// runtime's .cpp files include it first, and the driver passes it with -include to the user's
// own C/C++ files (Phase 04). It is plain C as well as C++, and including it twice is fine.
#if !defined(__OPTIMIZE__)
#undef _FORTIFY_SOURCE
#endif
