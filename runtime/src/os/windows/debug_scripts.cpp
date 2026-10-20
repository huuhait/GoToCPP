// debug_scripts.cpp: the GDB pretty printers of gocpp, inside every Windows program (Phase 14, step
// 14.6; the Linux version, src/os/linux/debug_scripts.cpp, says what the section is and why).
//
// A PE file keeps DWARF in sections with long names (the linker writes them into the string table),
// and gdb looks for .debug_gdb_scripts in them as it does in an ELF file. The text is the same as
// on Linux (src/os/gocpp_gdb.inc: a type byte 4, the name, a newline, the script, a NUL byte). The
// section is marked as debug information ("d"), so it is not loaded into memory and the linker's
// --gc-sections leaves it alone; strip removes it with the rest of the debug information.
//
// The assembler of COFF has no .pushsection, so the statement ends with .text. That is safe: the
// compiler writes a section directive of its own before the next function or variable (checked with
// g++ and clang++ and -ffunction-sections, whose functions are in .text$name).
namespace gp {

// A symbol, so that the object file has a global one like every runtime file (TestSymbols); it says
// that the program carries the gdb script. A debugger can read it: `info variables debug_script`.
extern const char debug_script_present[] = "gocpp-gdb.py";

} // namespace gp

__asm__(".section .debug_gdb_scripts, \"dr\"\n"
        ".byte 4\n"
        ".ascii \"gocpp-gdb.py\\n\"\n"
#include "../gocpp_gdb.inc"
        ".byte 0\n"
        ".text\n");
