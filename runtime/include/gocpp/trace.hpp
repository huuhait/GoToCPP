// gocpp/trace.hpp: the stack trace that an unrecovered panic prints after
// "goroutine 1 [running]:", in Go's format:
//
//     main.f(...)
//             /home/me/prog/main.gp:9 +0x4d
//     main.main()
//             /home/me/prog/main.gp:14 +0x7a
//
// Each frame is the function's Go name, with "(...)" when it has parameters (Go prints the
// argument words there; gocpp doesn't know them) and "()" when it has none, then a tab, the
// .gp file and line of the call (the #line directives put them into the DWARF line table)
// and the offset of the return address in the function, as Go prints it.
//
// How: a panic_error records the return addresses of the stack when it is made (os::stack, a
// cheap walk of the unwind tables, as the C++ unwinding then removes the frames), and only
// when the panic ends the program, append_trace reads the program's own file (os::read_exe):
// the symbol table gives each address its function, func_name turns the mangled C++ name into
// the Go name, and the DWARF line table gives the file and line. The runtime's own frames
// (gp::, std::, the C++ ABI, libc, gocpp's helper functions, and C++'s main, where the walk
// stops) are left out, like Go leaves out its runtime's frames.
//
// This header is for the runtime's own files and its tests: generated code doesn't include it.
#pragma once

#include <cstddef>
#include <cstdint>
#include <string>
#include <string_view>
#include <utility>
#include <vector>

namespace gp {
struct panic_error; // gocpp/panic.hpp
} // namespace gp

namespace gp::detail {

// exe_symbol is a function of the program's symbol table: its address in the file (before
// relocation), its size in bytes (0 when unknown) and its symbol name, as the compiler mangled
// it.
struct exe_symbol {
    uintptr_t addr;
    uintptr_t size;
    std::string name;
};

// exe_image is what a stack trace needs of the running program's own file (os::read_exe).
struct exe_image {
    // bias is what the loader added to the file's addresses (a position-independent program
    // is loaded anywhere): a run-time address minus bias is the address in the file, which the
    // symbol table and the DWARF line table use.
    uintptr_t bias = 0;
    // code are the run-time address ranges [first, second) of the program's own code, so a
    // frame in a shared library (libc, libstdc++) can be told apart.
    std::vector<std::pair<uintptr_t, uintptr_t>> code;
    // funcs are the functions of the symbol table (.symtab; none in a stripped program), sorted
    // by address.
    std::vector<exe_symbol> funcs;
    // The DWARF sections that the line table needs: .debug_line and the string sections that
    // DWARF 5 file names point into. Each is empty when the file doesn't have it (or has it
    // compressed).
    std::string debug_line, debug_line_str, debug_str;
    // The DWARF sections of the inlined calls (inline_frames): the units and their abbreviations,
    // and the tables that DWARF 5 forms point into. Read only in a -release -trace program
    // (read_inlines): a debug build's .debug_info is most of its file, and at -O0 nothing of the
    // program is inlined.
    std::string debug_info, debug_abbrev, debug_addr, debug_str_offsets, debug_rnglists,
        debug_ranges;

    // dwarf_section returns the member that keeps the section called name, or null for a section
    // that the trace does not read (the OS layer's read_exe asks for each section of the file).
    std::string* dwarf_section(std::string_view name) noexcept;
};

// read_inlines says whether the trace shows the calls that the compiler inlined as frames, as Go
// does, from the DWARF of the inlined calls (DW_TAG_inlined_subroutine): in a -release -trace
// program (the driver defines GOCPP_TRACE; the runtime is built with the program's flags), where
// the line of a call that was inlined would be the line of the inlined function's code (often a
// line of a runtime header). Only there: the first trace reads all of .debug_info, which costs
// about 100 ms more for the 48 MB of a big -O2 -g program, and its index stays in memory; the
// .debug_info of -g1 is a tenth of that. A plain -release program has no DWARF at all, and
// without the reader its file is smaller.
#ifdef GOCPP_TRACE
inline constexpr bool read_inlines = true;
#else
inline constexpr bool read_inlines = false;
#endif

// go_func is how the trace shows the function of a frame.
struct go_func {
    // name is the Go name ("main.f", "main.T.M", "main.main.func1", "main.init.0",
    // "github.com/a/b.F"); for a function that isn't gocpp's, the C++ name.
    std::string name;
    // shown is false for a frame that the trace leaves out: the runtime's own functions (gp::,
    // std::, __cxa_*, names that C++ reserves) and gocpp's generated helpers (gp_init_funcs,
    // gp_init_all, the generic lambdas of std::apply).
    bool shown = false;
    // params: the function has parameters, so Go prints "(...)" after the name; else "()".
    bool params = false;
    // raw: name is a C++ name with its own parameter list ("ns::f(int)"), or a C name: the
    // trace prints it as it is (a C name with "(...)").
    bool raw = false;
};

// func_name returns how the trace shows the function whose symbol name is symbol: a C++ name
// mangled by the Itanium C++ ABI (GCC and Clang, also on MinGW) or a C name. The Go name of a
// gocpp function comes from its C++ name (ADR-010): the namespace gpkg_<escaped import path>
// gives the package path back, a name that starts with gp_ loses the _ that escaped it, and
//
//     gpkg_main::f                             main.f
//     gpkg_main::T::M (a const member)         main.T.M (a value method, step 3.10)
//     gpkg_main::T::PM (a static member)       main.(*T).PM (a pointer method)
//     gpkg_main::F<...>                        main.F[...]
//     a lambda in gpkg_main::main (C++'s #k)   main.main.funck, and inside it main.main.funck.j
//     gpkg_main::gp_init_3                     main.init.2 (Go's numbering starts at 0)
//     gpkg_main::gp_var_init                   main.init (package variables, as in Go)
//
// A name escaped for a C++ keyword or macro (Go's new is new_) keeps its _: the runtime
// doesn't have the emitter's lists. A lambda's number is its number in the C++ function (g++,
// and clang++ inside a lambda), which is Go's func number while every C++ lambda there is a Go
// func literal (step 3.11). clang++ numbers the lambdas of a function that isn't inline in
// the whole file ($_0, $_1), so there the number is unknown: main.main.func?.
go_func func_name(std::string_view symbol);

// A line_query asks for the source line of the code at addr, an address in the program's file
// (before relocation). lines_of sets file and line, or leaves line 0 when the line table
// doesn't have addr.
struct line_query {
    uint64_t addr = 0;
    std::string file;
    int64_t line = 0;
    // near_lo and near_hi, when near_hi > near_lo, are the range of the inlined call that addr is
    // in: if the row of addr has no line (0, as clang gives code it merged from several places),
    // indexed_lines_of takes the nearest row of that range that has one, before addr first.
    uint64_t near_lo = 0, near_hi = 0;
};

// lines_of answers the queries from the DWARF line table, versions 2 to 5: debug_line is the
// .debug_line section, and line_str and str the .debug_line_str and .debug_str sections that
// DWARF 5 file names point into. It reads the whole table once, for all the queries, and
// stops at the first bad header or opcode of a unit (it never reads outside the sections).
void lines_of(std::string_view debug_line, std::string_view line_str, std::string_view str,
              line_query* queries, std::size_t n);

// indexed_lines_of does what lines_of does, but reads the whole table only once (the first call
// keeps the address ranges of every row, in memory, for the life of the program) and then answers
// a query by a binary search: for a program that asks for lines again and again (runtime.Caller,
// the testing package), where lines_of costs a pass over the table for every call. The sections
// must be the same in every call. It is safe to call from several threads.
void indexed_lines_of(std::string_view debug_line, std::string_view line_str, std::string_view str,
                      line_query* queries, std::size_t n);

// append_trace appends the trace of the return addresses pcs (innermost first, from
// os::stack) to out, like Go 1.25: every frame of a stack of up to 100 frames that the trace
// shows; for a deeper one, the inner 50, "...N frames elided..." and the outer 50 (main.main
// last). complete is false when pcs is only the top of the stack: then the inner 50 come, and
// Go's older "...additional frames elided..." line. It reads the program's file, so it is for
// the end of the program; when the file has no symbol table (a stripped program), or no line
// table and none of the frames is one that the trace shows, it prints each address of the
// program's code as "?()" and "\t?:0 pc=0x..." (an address in the file, for addr2line); with
// both tables a stack of only the runtime's frames shows no frame. An address 0 is the
// panic frame between a deferred call and the panic that ran it (panic_report puts it in
// between the stacks of a panic chain): "panic(...)" and "\t?:0", where Go shows
// runtime.gopanic as "panic({0x...?, 0x...?})" and its line in runtime/panic.go. cached is the
// program's file when the caller has it already (program_image, below: runtime.Stack asks often);
// null reads it again, for the end of a program.
void append_trace(std::string& out, const uintptr_t* pcs, std::size_t n, bool complete,
                  const exe_image* cached = nullptr);

struct goroutine; // gocpp/goroutine.hpp

// append_created_by appends Go's line for the goroutine that started g, after its trace:
//
//     created by main.main in goroutine 1
//             /path/main.go:8 +0x65
//
// from the address that the go statement left in g (gp::go, go_fn and go_call pass their return
// address to go_start). Nothing is appended for the main goroutine, a thread that no go statement
// started, or a go statement in the runtime's own code (the function of an AfterFunc).
void append_created_by(std::string& out, const goroutine* g);
// The same from the two values of the record (a report that reads the record of a running
// goroutine copies them first, in one place that the race detector does not watch).
void append_created_by(std::string& out, const void* create_pc, uint64_t parent_id);

// An inlined call at an address (inline_frames): the function inlined (not shown when it has no
// linkage name, or is the runtime's), and the file and line of the call.
struct inline_level {
    go_func fn;
    std::string file;
    int64_t line = 0;
    uint64_t lo = 0, hi = 0; // the range of the inlined code that has the address
};

// inline_frames sets out to the calls that the compiler inlined at call (an address in the file)
// in the function [func_lo, func_hi), outermost first, from the DWARF of the inlined calls of img
// (DW_TAG_inlined_subroutine). It is empty without that DWARF, and when read_inlines is false.
// The first call reads all of .debug_info and keeps what it needs for the life of the program,
// like indexed_lines_of; it is safe to call from several threads.
void inline_frames(const exe_image& img, uint64_t call, uint64_t func_lo, uint64_t func_hi,
                   std::vector<inline_level>& out);
// read_inline_frames is inline_frames whatever read_inlines says (the unit tests' entry point).
void read_inline_frames(const exe_image& img, uint64_t call, uint64_t func_lo, uint64_t func_hi,
                        std::vector<inline_level>& out);

// find_symbol returns the function of img that contains addr (an address in the file, before
// relocation), or null when addr is before the first function or between two of them.
const exe_symbol* find_symbol(const exe_image& img, uint64_t addr) noexcept;

// fatal_trace appends the stack trace of the code that called it to out (for gp::detail::fatal,
// panic.hpp), as a panic's trace shows it. In a deferred call that a panic runs, it shows Go's
// shape, as panic_report does for a panic there: the deferred call's frames, a "panic" frame
// and the stack of the running panic (and of its chain), found with run_marks. Without memory,
// out keeps what it had.
void fatal_trace(std::string& out) noexcept;

// The traceback setting (Go's GOTRACEBACK and debug.SetTraceback; gocpp has no runtime frames to
// add, so system and crash print what all prints, and crash ends with code 2 as a panic does).
// traceback_level is 0 for none (a panic or a fatal error prints only its text: no "goroutine N
// [running]:" and no trace), 1 for single, the default, and more for all, system, crash and the
// numbers; traceback_all says whether a panic also prints the other goroutines (all, system,
// crash, and any number but 0; a word that Go does not know is "all" with level 0). A fatal error
// prints them at every level but 0, as Go does for a throw.
// set_traceback_level is debug.SetTraceback: the new level replaces the last one (also a lower
// one), but the GOTRACEBACK variable is a floor: what it sets stays set. The timeout of a test
// uses it.
bool traceback_all() noexcept;
int traceback_level() noexcept;
void set_traceback_level(std::string_view level) noexcept;

// append_other_goroutines appends the report of every goroutine but the caller's, as Go's
// traceback all does: "goroutine N [reason]:", the stack and "created by", after a blank line. A
// goroutine that is blocked in the runtime (a channel, a lock, a sleep, a select) gives its stack;
// one that runs, or waits in code that is not the runtime's, is listed without one (a thread
// cannot walk another's stack portably). The goroutines go on while the report is made.
void append_other_goroutines(std::string& out) noexcept;

// A run_mark is a call of defer_frame::gp_run (defer.cpp) that is running a deferred call. The
// marks of a thread are a list, the innermost first (run_marks), each on its gp_run's stack:
// gp_panic is the panic that is running (null for a normal return), and gp_ret gp_run's
// return address, the pc of its caller's frame in a recorded stack.
struct run_mark {
    panic_error* gp_panic; // recover() marks it recovered (gp::recover)
    uintptr_t gp_ret;
    const run_mark* gp_next;
};

// The running thread's innermost run_mark, or null (defer_frame::gp_run sets it; inline, so that a
// deferred call can look at it cheaply: gocpp/defer.hpp).
inline constinit thread_local const run_mark* top_run = nullptr;

// run_marks returns the running thread's innermost run_mark, or null.
const run_mark* run_marks() noexcept;

// A defer_link is the place of a defer_frame in the list of the frames that a thread has now (the
// innermost first). A panic_error records its stack only up to the innermost one of them
// (panic.hpp), which is on the stack of the thread, as every local variable is. (Not with
// AddressSanitizer: it may move the locals to a "fake stack", where their addresses say nothing
// about the real one, so no frame is listed and a panic records its whole stack.)
//
// A frame keeps the distance to the next frame out (the frames are deeper in the stack the later
// they are made) in 32 bits, which is all that fits in the frame's 96 bytes; a longer distance, or
// an older frame that is not above it, ends the list (a panic then records more of its stack than
// it needs, nothing else).
struct defer_link {
    std::uint32_t gp_up = 0; // the distance in bytes to the next one, 0 for none

    // gp_set makes this link the first of the list that begins at next.
    void gp_set(const defer_link* next) noexcept {
        // (No next one: the difference wraps around to a huge number, and the list ends here.)
        auto d = reinterpret_cast<std::uintptr_t>(next) - reinterpret_cast<std::uintptr_t>(this);
        gp_up = d <= 0xffffffffu ? static_cast<std::uint32_t>(d) : 0;
    }
    const defer_link* gp_next() const noexcept {
        return gp_up == 0 ? nullptr
                          : reinterpret_cast<const defer_link*>(
                                reinterpret_cast<std::uintptr_t>(this) + gp_up);
    }
};
inline constinit thread_local const defer_link* top_defer = nullptr;

#if defined(_WIN32) && !defined(__x86_64__) && !defined(_M_X64)
// (Without an unwinder loop of our own, a panic records its whole stack: os/windows/trace.cpp.)
inline constexpr bool defer_links_listed = false;
#elif defined(__SANITIZE_ADDRESS__)
inline constexpr bool defer_links_listed = false;
#elif defined(__has_feature)
#if __has_feature(address_sanitizer)
inline constexpr bool defer_links_listed = false;
#else
inline constexpr bool defer_links_listed = true;
#endif
#else
inline constexpr bool defer_links_listed = true;
#endif

// --- the running goroutine's own stack: runtime.Caller, Callers, Stack, FuncForPC (step 8.10) ---

// program_image returns the program's own file (os::read_exe), read the first time and kept for
// the life of the process, so that a program that asks for its callers often (a logger) doesn't
// read its file each time. It is null when the file can't be read. Thread-safe.
const exe_image* program_image();

// A frame_info is what is known about the code at one address: ok says that the address is in a
// function of the program's symbol table; fn is how the trace names it (go_func), entry the
// run-time address of the function's first instruction, and file and line the source position
// (line 0: the line table has none).
struct frame_info {
    bool ok = false;
    go_func fn;
    std::string file;
    int64_t line = 0;
    uintptr_t entry = 0;
};

// resolve_pcs describes the n run-time addresses pcs. With call set they are return addresses
// (the addresses that os::stack and runtime.Callers give): the call is one byte before. The answers
// are kept by address, so asking for the same call again costs a hash lookup; the DWARF line table
// is read once for all the addresses that were not asked before, not once for each.
void resolve_pcs(const uintptr_t* pcs, std::size_t n, bool call, frame_info* out);

// shown_frames returns the return addresses of the calling thread's stack that a Go trace shows,
// innermost first, at most want of them (fewer when the stack ends). The runtime's own frames, the
// C++ library's and gocpp's helpers are left out, and the walk ends at C++'s main. A native
// function of the package runtime calls it, so the first address is in the Go function that called
// that native function.
std::vector<uintptr_t> shown_frames(std::size_t want);

} // namespace gp::detail
