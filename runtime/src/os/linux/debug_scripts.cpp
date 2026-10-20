// debug_scripts.cpp: the GDB pretty printers of gocpp, inside every program (Phase 13, step 13.2).
//
// GDB looks for a section named .debug_gdb_scripts in each program that it loads, and runs the
// scripts in it. This one holds the text of runtime/debug/gocpp-gdb.py (src/os/gocpp_gdb.inc, made
// from it by `go test ./runtime -run TestDebugScript -update-debug-script`), so gdb shows
// gocpp's values and Go names without anything installed next to the program, and a program
// that is moved to another machine keeps its printers. The text is the script's own type,
// type 4 in GDB's list (Python text): a name, a newline, the text, a NUL byte.
//
// The section is not loaded into memory (it has no "a" flag), so the program is not slower and
// uses no more memory; strip removes it with the rest of the debug information. The R flag
// (retain) keeps it through the link's --gc-sections. GDB runs it only for a safe path:
// `gocpp debug` passes it, docs/guides/debugging.md shows the line for plain gdb.
namespace gp {

// A symbol, so that the object file has a global one like every runtime file (TestSymbols); it says
// that the program carries the gdb script. A debugger can read it: `info variables debug_script`.
extern const char debug_script_present[] = "gocpp-gdb.py";

} // namespace gp

#if defined(__ELF__)
__asm__(".pushsection \".debug_gdb_scripts\", \"MSR\", %progbits, 1\n"
        ".byte 4\n"
        ".ascii \"gocpp-gdb.py\\n\"\n"
#include "../gocpp_gdb.inc"
        ".byte 0\n"
        ".popsection\n");
#endif
