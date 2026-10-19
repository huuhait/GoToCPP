// The race detector's side of the Linux OS layer (gocpp/race.hpp): only compiled into a program
// that was built with -race (ThreadSanitizer); in every other build this file is empty.
//
// ThreadSanitizer prints its own reports; gocpp prints Go's. TSan writes its text to a "report
// file", which init points at an anonymous file instead of stderr; after TSan has printed a report
// it calls __tsan_on_report. That call is made with TSan's locks held, in the thread that found
// the race. So the callback is built from code that TSan does not instrument and from system calls
// only: no allocation, no libc function (TSan intercepts most of them and takes its own locks
// there), no std::atomic (an instrumented atomic can need the lock that this thread holds: the
// program then hangs, for ever, depending on the layout of the stack). It copies the report's
// pieces (the accesses, the threads with the stacks that created them, as TSan's report API gives
// them) into a ring of records in shared memory and wakes the reporter with a futex.
//
// The reporter is a process, not a thread: init forks it, before the program has a thread of its
// own, and it shares the ring (an anonymous shared mapping) with the program. ThreadSanitizer
// counts what the program's threads do, memory allocations included, in a budget of events per
// thread slot, and when the budget is gone it forgets every access it has seen; a reporter that
// reads a 7 MB program file, names thousands of symbols, and formats the report of each race in
// the program's own process would use that budget up, and a race whose first access came before
// would not be found. In its own process the reporter costs the program nothing.
//
// The reporter turns a record into Go's text ("WARNING: DATA RACE", goroutine names, the `.gp`
// lines of the #line directives) and writes it to stderr in one write, in the order of the
// callbacks; the frames go through the code of the panic trace (gp::detail::append_trace, with the
// program's file read and indexed once, and not again for every stack): the Go names
// ("main.main.func1") and no frames of the runtime. The reporter is a copy of the program as it was
// at the fork, so it has the same addresses. Like Go, it prints a race once: a report whose two
// accesses have the stacks (as Go shows them) of a report already printed is dropped. The reporter
// reads the program's file at once, so that the first report does not wait for it, and it ends
// when the program does (its parent dies).
//
// The callback waits until the reporter has written the report (for at most three seconds), as
// Go's detector prints a report before the program goes on: a race that is followed by a crash, a
// kill or an exit is on the screen. finish (the exit of the program, also the exit of a panic and
// of a fatal error) waits for the reports that are left, for at most three seconds, and prints
// "Found N data race(s)". It allocates nothing, so that the code of a fatal error can call it.
//
// What TSan prints that is not a report (a failed check, "thread limit exceeded") stays in the
// report file, and so does the text of a report that the reporter has not written when the process
// dies (halt_on_error=1: TSan dies right after the callback, which holds TSan's locks): the death
// callback gives the reporter a moment and copies the unwritten rest of the file to stderr, so
// that a fatal message and the report that stopped the program are never lost.
//
// GORACE is Go's variable for the race detector's options (halt_on_error=1, exitcode=N,
// log_path=file, history_size=N, strip_path_prefix=...), and they are TSan's options under the
// same names: __tsan_default_options returns gocpp's defaults followed by GORACE, and TSAN_OPTIONS
// (which TSan reads after the defaults) wins over both. log_path sends TSan's own text to the file
// and keeps stderr free of reports; strip_path_prefix also shortens the file names of Go's form.
#include <gocpp/nofortify.h>
#include <gocpp/os.hpp>
#include <gocpp/race.hpp>
#include <gocpp/trace.hpp>

#if GOCPP_RACE

#include <algorithm>
#include <atomic>
#include <condition_variable>
#include <mutex>
#include <cerrno>
#include <csignal>
#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <memory>
#include <set>
#include <string>
#include <vector>

#include <fcntl.h>
#include <sys/mman.h>
#include <sys/prctl.h>
#include <sys/syscall.h>
#include <unistd.h>

#ifdef __clang__
// No instrumentation at all, not even the entry and exit hooks of TSan, which crash before TSan
// has started (clang's no_sanitize("thread") leaves those in).
#define GOCPP_NO_TSAN __attribute__((disable_sanitizer_instrumentation))
#else
#define GOCPP_NO_TSAN __attribute__((no_sanitize("thread")))
#endif

// The part of TSan's interface that this file uses (sanitizer/tsan_interface.h and
// sanitizer/common_interface_defs.h declare the same).
extern "C" {
int __tsan_get_report_data(void* report, const char** description, int* count, int* stack_count,
                           int* mop_count, int* loc_count, int* mutex_count, int* thread_count,
                           int* unique_tid_count, void** sleep_trace, unsigned long trace_size);
int __tsan_get_report_stack(void* report, unsigned long idx, void** trace,
                            unsigned long trace_size);
int __tsan_get_report_mop(void* report, unsigned long idx, int* tid, void** addr, int* size,
                          int* write, int* atomic, void** trace, unsigned long trace_size);
int __tsan_get_report_thread(void* report, unsigned long idx, int* tid, uint64_t* os_id,
                             int* running, const char** name, int* parent_tid, void** trace,
                             unsigned long trace_size);
void __sanitizer_set_report_fd(void* fd);
void __sanitizer_set_death_callback(void (*callback)(void));
void AnnotateThreadName(const char* file, int line, const char* name);
}

namespace gp::race {

namespace {

// raw_syscall makes a system call without libc: TSan's runtime intercepts functions of libc, and
// __tsan_default_options runs before it can. Where the architecture is not one of the two that
// gocpp targets, the call fails (-1).
GOCPP_NO_TSAN long raw_syscall(long nr, long a, long b, long c, long d = 0) {
#if defined(__x86_64__)
    long ret;
    register long r10 asm("r10") = d;
    asm volatile("syscall"
                 : "=a"(ret)
                 : "a"(nr), "D"(a), "S"(b), "d"(c), "r"(r10)
                 : "rcx", "r11", "memory");
    return ret;
#elif defined(__aarch64__)
    register long x8 asm("x8") = nr;
    register long x0 asm("x0") = a;
    register long x1 asm("x1") = b;
    register long x2 asm("x2") = c;
    register long x3 asm("x3") = d;
    asm volatile("svc 0" : "+r"(x0) : "r"(x8), "r"(x1), "r"(x2), "r"(x3) : "memory");
    return x0;
#else
    (void)nr;
    (void)a;
    (void)b;
    (void)c;
    (void)d;
    return -1;
#endif
}

constexpr std::size_t max_frames = 64;
constexpr std::size_t max_mops = 4;
constexpr std::size_t max_threads = 8;
constexpr std::size_t max_stacks = 2;
constexpr std::size_t ring_size = 256;

// A thread of a report: TSan's tid, how it is named, and the stack that created it.
struct thread_rec {
    int tid;
    int running;
    uint64_t goroutine; // 0: not a goroutine
    void* trace[max_frames];
};

// An access of a report.
struct mop_rec {
    int tid;
    int write;
    int atomic;
    void* addr;
    void* trace[max_frames];
};

// A record is what the callback copies out of a report, in static memory (see the top).
struct record {
    char desc[40];
    int nmops, nthreads, nstacks;
    long raw_end; // where TSan's own text of the report ends in the report file
    mop_rec mops[max_mops];
    thread_rec threads[max_threads];
    void* stacks[max_stacks][max_frames];
};

// What the program and the reporter share: an anonymous shared mapping that init makes before it
// forks. It is only touched with the __atomic builtins or by one side at a time: the callback (one
// at a time: TSan serializes its reports) fills ring[produced % ring_size] and then counts it in
// produced; the reporter reads it and then counts it in written.
struct shared_state {
    uint32_t produced;      // records the callback has filled (a futex word)
    uint32_t written;       // records the reporter has finished (a futex word)
    uint32_t dropped;       // reports that did not fit in the ring
    uint32_t printed;       // data races the reporter has printed
    uint32_t up;            // 1 when the reporter has read the program's file and runs
    long raw_done;          // the text of the report file before this offset is not needed
    char strip_prefix[256]; // strip_path_prefix of GORACE or TSAN_OPTIONS, or ""
    record ring[ring_size];
};
shared_state* S = nullptr;
int report_fd = -1; // the anonymous file that holds TSan's own text, or -1
long reporter_pid = 0;

GOCPP_NO_TSAN uint32_t load(const uint32_t* p) {
    return __atomic_load_n(p, __ATOMIC_ACQUIRE);
}

// (The futex calls are not "private": the two sides are two processes.)
GOCPP_NO_TSAN void futex_wake(uint32_t* p) {
    raw_syscall(SYS_futex, reinterpret_cast<long>(p), 1 /* FUTEX_WAKE */, 1);
}

// futex_wait sleeps while *p == v, for at most ms milliseconds (ms < 0: for ever).
GOCPP_NO_TSAN void futex_wait(uint32_t* p, uint32_t v, long ms) {
    long ts[2] = {ms / 1000, (ms % 1000) * 1000000};
    raw_syscall(SYS_futex, reinterpret_cast<long>(p), 0 /* FUTEX_WAIT */, static_cast<long>(v),
                ms < 0 ? 0 : reinterpret_cast<long>(ts));
}

// now_ms is the monotonic clock in milliseconds.
GOCPP_NO_TSAN long now_ms() {
    long ts[2] = {0, 0};
    raw_syscall(SYS_clock_gettime, 1 /* CLOCK_MONOTONIC */, reinterpret_cast<long>(ts), 0);
    return ts[0] * 1000 + ts[1] / 1000000;
}

GOCPP_NO_TSAN void raw_write(const char* p, std::size_t n) {
    while (n > 0) {
        long w = raw_syscall(SYS_write, 2, reinterpret_cast<long>(p), static_cast<long>(n));
        if (w == -EINTR) {
            continue;
        }
        if (w <= 0) {
            return;
        }
        p += w;
        n -= static_cast<std::size_t>(w);
    }
}

// drain waits (for at most ms milliseconds) until the reporter has written every record.
GOCPP_NO_TSAN bool drain(long ms) {
    if (S == nullptr) {
        return true;
    }
    long end = now_ms() + ms;
    for (;;) {
        uint32_t w = load(&S->written);
        if (w == load(&S->produced)) {
            return true;
        }
        long left = end - now_ms();
        if (left <= 0) {
            return false;
        }
        futex_wait(&S->written, w, left < 20 ? left : 20);
    }
}

// copy_raw writes the text of the report file from offset from on, to stderr.
GOCPP_NO_TSAN void copy_raw(long from) {
    if (report_fd < 0) {
        return;
    }
    static char buf[1 << 14];
    long off = from;
    for (;;) {
        long got =
            raw_syscall(SYS_pread64, report_fd, reinterpret_cast<long>(buf), sizeof buf, off);
        if (got <= 0) {
            return;
        }
        raw_write(buf, static_cast<std::size_t>(got));
        off += got;
    }
}

// append_number writes n in decimal at buf[at...] and returns the new end.
GOCPP_NO_TSAN std::size_t append_number(char* buf, std::size_t at, uint32_t n) {
    char digits[12];
    std::size_t k = 0;
    do {
        digits[k++] = static_cast<char>('0' + n % 10);
        n /= 10;
    } while (n > 0);
    while (k > 0) {
        buf[at++] = digits[--k];
    }
    return at;
}

GOCPP_NO_TSAN std::size_t append_text(char* buf, std::size_t at, const char* s) {
    while (*s != '\0') {
        buf[at++] = *s++;
    }
    return at;
}

// print_found writes the closing lines ("Found N data race(s)") when there was a report.
GOCPP_NO_TSAN void print_found() {
    uint32_t shown = load(&S->printed), lost = load(&S->dropped);
    if (shown + lost == 0) {
        return;
    }
    char buf[160];
    std::size_t n = 0;
    if (lost > 0) {
        n = append_text(buf, n, "(");
        n = append_number(buf, n, lost);
        n = append_text(buf, n, " more reports were not shown)\n");
    }
    n = append_text(buf, n, "Found ");
    n = append_number(buf, n, shown + lost);
    n = append_text(buf, n, " data race(s)\n");
    raw_write(buf, n);
}

// on_death is TSan's death callback (halt_on_error=1, a failed check): the reports that the
// reporter has not written yet, a moment for it, and what TSan itself printed after that.
GOCPP_NO_TSAN void on_death() {
    if (S == nullptr) {
        return;
    }
    drain(1000);
    copy_raw(__atomic_load_n(&S->raw_done, __ATOMIC_ACQUIRE));
    // (No count: Go's halt_on_error=1 prints the report and ends with 66.)
}

// --- the finished goroutines ---

// ThreadSanitizer remembers the accesses of a thread in the thread's trace, and needs the trace of
// the thread that made an earlier access to name it in a report. The trace of a thread that has
// ended is the first that ThreadSanitizer gives to another thread when it needs more, and a race
// with the earlier access of a goroutine that has finished is then dropped, without a word (the
// report would have no stack for it): after a few hundred allocations of the other goroutine. So
// a thread that ends keeps its place: it parks, in a ring of the last 64 (keep_default, or
// GOCPP_RACE_KEEP) finished goroutines, and the next one to finish after it releases it, a
// 64 goroutines later. A program that has more than that many goroutines finish between the
// two accesses of a race can still lose the race.
constexpr std::size_t keep_default = 64;

struct park_slot {
    std::mutex mu;
    std::condition_variable cv;
    uint64_t ticket =
        0; // the ticket of the thread parked here; 0 for none; ~0 for released for good
};

park_slot* park_slots = nullptr;
std::size_t park_count = 0;
std::atomic<uint64_t> park_tickets{1};

void keep_init() {
    std::size_t n = keep_default;
    if (const char* e = std::getenv("GOCPP_RACE_KEEP")) {
        n = static_cast<std::size_t>(std::strtoull(e, nullptr, 10));
    }
    if (n > 0) {
        park_slots = new park_slot[n];
        park_count = n;
    }
}

// --- the reporter: Go's text ---

// add_stack appends the frames of the trace (null-terminated or max_frames long) in Go's form,
// indented, and says whether there was a frame of the program. top says that the first address is
// where an instruction ran, not a return address. A stack of no frame of the program (a thread
// that the runtime started) is "?()" lines with addresses, which only an access shows.
bool add_stack(std::string& out, void* const* trace, bool top, bool addresses) {
    uintptr_t pcs[max_frames];
    std::size_t n = 0;
    while (n < max_frames && trace[n] != nullptr) {
        pcs[n] = reinterpret_cast<uintptr_t>(trace[n]);
        n++;
    }
    if (top && n > 0) {
        pcs[0] += 1; // append_trace looks up the call at pc-1, as for a return address
    }
    std::string text;
    gp::detail::append_trace(text, pcs, n, true, gp::detail::program_image());
    if (!addresses && (text.empty() || text.compare(0, 3, "?()") == 0)) {
        return false;
    }
    std::size_t pl = std::strlen(S->strip_prefix);
    std::size_t at = 0;
    while (at < text.size()) {
        std::size_t nl = text.find('\n', at);
        if (nl == std::string::npos) {
            nl = text.size();
        }
        std::string line = text.substr(at, nl - at);
        at = nl + 1;
        if (!line.empty() && line[0] == '\t') {
            line.erase(0, 1);
            if (pl > 0 && line.compare(0, pl, S->strip_prefix) == 0) {
                line.erase(0, pl);
            }
            out += "      ";
            out += line;
        } else {
            // Go's race report shows a function without its arguments: main.work(), where a
            // panic's trace has main.work(...).
            if (line.size() > 5 && line.compare(line.size() - 5, 5, "(...)") == 0) {
                line.erase(line.size() - 4, 3);
            }
            out += "  ";
            out += line;
        }
        out += '\n';
    }
    return true;
}

// who is how a report names the thread: "main goroutine", "goroutine 8".
std::string who(const thread_rec* t, int tid) {
    if (t != nullptr && t->goroutine == 1) {
        return "main goroutine";
    }
    if (t != nullptr && t->goroutine != 0) {
        return "goroutine " + std::to_string(t->goroutine);
    }
    return "thread T" + std::to_string(tid);
}

std::string capital(std::string s) {
    if (!s.empty() && s[0] >= 'a' && s[0] <= 'z') {
        s[0] = static_cast<char>(s[0] - 'a' + 'A');
    }
    return s;
}

struct formatted {
    std::string text;
    std::string key; // what two reports of one race have in common: the stacks of the accesses
    bool race = false;
};

// format_go returns the report in Go's form.
formatted format_go(const record& r) {
    formatted f;
    std::string& out = f.text;
    out = "==================\n";
    // "data-race" and "data-race-vptr" (a race on the pointer to the vtable of an object, clang)
    // are both Go's data race.
    f.race = std::strncmp(r.desc, "data-race", 9) == 0;
    if (f.race) {
        out += "WARNING: DATA RACE\n";
    } else {
        out += "WARNING: ThreadSanitizer: ";
        out += r.desc;
        out += '\n';
    }
    auto find = [&](int tid) -> const thread_rec* {
        for (int i = 0; i < r.nthreads; i++) {
            if (r.threads[i].tid == tid) {
                return &r.threads[i];
            }
        }
        return nullptr;
    };
    std::vector<std::string> keys;
    for (int i = 0; i < r.nmops; i++) {
        const mop_rec& m = r.mops[i];
        std::string kind = std::string(i == 0 ? "" : "previous ") +
                           (m.atomic != 0 ? "atomic " : "") + (m.write != 0 ? "write" : "read");
        char buf[64];
        std::snprintf(buf, sizeof buf, " at 0x%012llx by ",
                      static_cast<unsigned long long>(reinterpret_cast<uintptr_t>(m.addr)));
        if (i > 0) {
            out += '\n';
        }
        out += capital(kind) + buf + who(find(m.tid), m.tid) + ":\n";
        std::string stack;
        add_stack(stack, m.trace, true, true);
        out += stack;
        keys.push_back((m.write != 0 ? "w" : "r") + stack);
    }
    std::sort(keys.begin(), keys.end());
    for (const std::string& k : keys) {
        f.key += k;
        f.key += '\x01';
    }
    if (!f.race) {
        f.key += r.desc;
        for (int i = 0; i < r.nstacks; i++) {
            out += '\n';
            std::string stack;
            add_stack(stack, r.stacks[i], false, true);
            out += stack;
            f.key += stack;
        }
    }
    for (int i = 0; i < r.nthreads; i++) {
        const thread_rec& t = r.threads[i];
        if (t.trace[0] == nullptr || t.goroutine == 1) {
            continue; // the main goroutine, or a thread that no go statement created
        }
        std::string stack;
        if (!add_stack(stack, t.trace, false, false)) {
            continue; // a goroutine that the runtime started (a timer), with no frame to show
        }
        out += '\n' + capital(who(&t, t.tid)) + (t.running != 0 ? " (running)" : " (finished)") +
               " created at:\n";
        out += stack;
    }
    out += "==================\n";
    return f;
}

// reporter_main is the reporter process: it writes the records as they come, until its parent
// dies.
[[noreturn]] void reporter_main(long parent) {
    prctl(PR_SET_PDEATHSIG, SIGKILL);
    if (getppid() != parent) {
        _exit(0); // the program ended before this ran
    }
    // The program's signals (^C, a kill of the group) are the program's: it drains the reporter
    // before it dies of them.
    for (int sig : {SIGTERM, SIGINT, SIGHUP, SIGQUIT, SIGPIPE}) {
        signal(sig, SIG_IGN);
    }
    // Read the program's file and its line table now: the first report then costs a lookup and
    // not a parse, and a program that dies right after its race still has said where it is. The
    // program waits for this at its start (init).
    if (const gp::detail::exe_image* img = gp::detail::program_image()) {
        gp::detail::line_query q;
        gp::detail::indexed_lines_of(img->debug_line, img->debug_line_str, img->debug_str, &q, 1);
    }
    __atomic_store_n(&S->up, 1, __ATOMIC_RELEASE);
    futex_wake(&S->up);
    std::set<std::string> seen;
    for (;;) {
        uint32_t w = load(&S->written);
        uint32_t p = load(&S->produced);
        if (w == p) {
            if (getppid() != parent) {
                _exit(0);
            }
            futex_wait(&S->produced, p, 50);
            continue;
        }
        const record& r = S->ring[w % ring_size];
        formatted f = format_go(r);
        if (seen.insert(f.key).second) {
            raw_write(f.text.data(), f.text.size());
            if (f.race) {
                __atomic_fetch_add(&S->printed, 1, __ATOMIC_ACQ_REL);
            }
        }
        __atomic_store_n(&S->raw_done, r.raw_end, __ATOMIC_RELEASE);
        __atomic_store_n(&S->written, w + 1, __ATOMIC_RELEASE);
        futex_wake(&S->written);
    }
}

// option finds name= in the options text s (spaces, colons or commas between options) and copies
// its value into out.
void option(const char* s, const char* name, char* out, std::size_t cap) {
    if (s == nullptr) {
        return;
    }
    std::size_t nl = std::strlen(name);
    for (const char* p = std::strstr(s, name); p != nullptr; p = std::strstr(p + 1, name)) {
        if (p != s && p[-1] != ' ' && p[-1] != ':' && p[-1] != ',') {
            continue;
        }
        const char* v = p + nl;
        std::size_t k = 0;
        while (*v != '\0' && *v != ' ' && *v != ':' && *v != ',' && k + 1 < cap) {
            out[k++] = *v++;
        }
        out[k] = '\0';
    }
}

} // namespace

void init() noexcept {
    if constexpr (!mode) {
        return; // a TSan build that is not -race (the runtime's own tests): TSan's own text
    }
    keep_init();
    const char* raw = std::getenv("GOCPP_RACE_RAW");
    if (raw != nullptr && raw[0] != '\0' && raw[0] != '0') {
        return; // TSan's own text, to stderr
    }
    for (const char* var : {"GORACE", "TSAN_OPTIONS"}) {
        const char* opts = std::getenv(var);
        if (opts != nullptr && std::strstr(opts, "log_path=") != nullptr) {
            return; // the reports go to the file, in TSan's own text
        }
    }
    FILE* f = std::tmpfile();
    if (f != nullptr) {
        report_fd = dup(fileno(f));
        std::fclose(f);
    }
    if (report_fd >= 0) {
        void* m = mmap(nullptr, sizeof(shared_state), PROT_READ | PROT_WRITE,
                       MAP_SHARED | MAP_ANONYMOUS, -1, 0);
        if (m != MAP_FAILED) {
            S = static_cast<shared_state*>(m);
            for (const char* var : {"GORACE", "TSAN_OPTIONS"}) {
                option(std::getenv(var), "strip_path_prefix=", S->strip_prefix,
                       sizeof S->strip_prefix);
            }
            long parent = getpid();
            std::fflush(nullptr); // the child must not write what the program has buffered
            pid_t pid = fork();
            if (pid == 0) {
                reporter_main(parent);
            }
            if (pid > 0) {
                reporter_pid = pid;
                // The reporter reads the program's file (a quarter of a second for a big one).
                // The program starts when it is ready, so that every race has its report in time.
                long end = now_ms() + 10000;
                while (load(&S->up) == 0 && now_ms() < end) {
                    futex_wait(&S->up, 0, 50);
                }
            }
            if (pid < 0) {
                munmap(m, sizeof(shared_state));
                S = nullptr;
            }
        }
        if (S != nullptr) {
            __sanitizer_set_report_fd(reinterpret_cast<void*>(static_cast<intptr_t>(report_fd)));
            __sanitizer_set_death_callback(on_death);
        } else {
            close(report_fd); // no reporter: TSan's own text, to stderr
            report_fd = -1;
        }
    }
    goroutine_started(1);
}

void finish(int code) noexcept {
    if (S == nullptr) {
        return;
    }
    drain(3000);
    if (code == 0) {
        // Go's os.Exit(0) after a race: the count (TSan makes the exit code 66; it leaves another
        // code of the program alone).
        print_found();
    }
}

void finished_goroutine() noexcept {
    if (park_count == 0) {
        return;
    }
    uint64_t t = park_tickets.fetch_add(1, std::memory_order_relaxed);
    park_slot& s = park_slots[t % park_count];
    std::unique_lock<std::mutex> lk(s.mu);
    if (s.ticket > t) {
        return; // a later goroutine has parked here already, or the ring is released
    }
    s.ticket = t; // the goroutine that parked here before is released
    s.cv.notify_all();
    s.cv.wait(lk, [&] { return s.ticket != t; });
}

void release_finished() noexcept {
    for (std::size_t i = 0; i < park_count; i++) {
        std::lock_guard<std::mutex> lk(park_slots[i].mu);
        park_slots[i].ticket = ~uint64_t(0);
        park_slots[i].cv.notify_all();
    }
}

long reporter_process() noexcept {
    return reporter_pid;
}

void goroutine_started(uint64_t id) noexcept {
    if constexpr (!mode) {
        return;
    }
    char name[32];
    std::snprintf(name, sizeof name, "goroutine %llu", static_cast<unsigned long long>(id));
    AnnotateThreadName(__FILE__, __LINE__, name);
}

namespace {

// on_report is __tsan_on_report: it copies the report into a record. It runs with TSan's locks
// held, in a thread that TSan watches: nothing here is instrumented, and nothing calls libc.
GOCPP_NO_TSAN void on_report(void* report) noexcept {
    if (S == nullptr) {
        return; // raw mode
    }
    uint32_t at = load(&S->produced);
    if (at - load(&S->written) >= ring_size) {
        __atomic_fetch_add(&S->dropped, 1, __ATOMIC_ACQ_REL); // only TSan's own text shows this one
        return;
    }
    record& r = S->ring[at % ring_size];
    const char* desc = "";
    int count = 0, stacks = 0, mops = 0, locs = 0, mutexes = 0, threads = 0, uniq = 0;
    void* sleep_trace[max_frames];
    __tsan_get_report_data(report, &desc, &count, &stacks, &mops, &locs, &mutexes, &threads, &uniq,
                           sleep_trace, max_frames);
    std::size_t k = 0;
    for (; desc != nullptr && desc[k] != '\0' && k < sizeof r.desc - 1; k++) {
        r.desc[k] = desc[k];
    }
    r.desc[k] = '\0';
    r.nmops = mops < static_cast<int>(max_mops) ? mops : static_cast<int>(max_mops);
    r.nthreads = threads < static_cast<int>(max_threads) ? threads : static_cast<int>(max_threads);
    r.nstacks = stacks < static_cast<int>(max_stacks) ? stacks : static_cast<int>(max_stacks);
    for (int i = 0; i < r.nmops; i++) {
        mop_rec& m = r.mops[i];
        for (auto& pc : m.trace) {
            pc = nullptr;
            asm volatile("" ::: "memory"); // keep the loop a loop: a memset would be TSan's
        }
        int size = 0;
        __tsan_get_report_mop(report, static_cast<unsigned long>(i), &m.tid, &m.addr, &size,
                              &m.write, &m.atomic, m.trace, max_frames);
    }
    for (int i = 0; i < r.nthreads; i++) {
        thread_rec& t = r.threads[i];
        for (auto& pc : t.trace) {
            pc = nullptr;
            asm volatile("" ::: "memory");
        }
        const char* name = nullptr;
        uint64_t os_id = 0;
        int parent = 0;
        __tsan_get_report_thread(report, static_cast<unsigned long>(i), &t.tid, &os_id, &t.running,
                                 &name, &parent, t.trace, max_frames);
        t.goroutine = 0;
        // "goroutine <id>", parsed by hand
        const char prefix[] = "goroutine ";
        std::size_t j = 0;
        while (name != nullptr && prefix[j] != '\0' && name[j] == prefix[j]) {
            j++;
        }
        if (name != nullptr && prefix[j] == '\0') {
            for (const char* d = name + j; *d >= '0' && *d <= '9'; d++) {
                t.goroutine = t.goroutine * 10 + static_cast<uint64_t>(*d - '0');
            }
        }
    }
    for (int i = 0; i < r.nstacks; i++) {
        for (auto& pc : r.stacks[i]) {
            pc = nullptr;
            asm volatile("" ::: "memory");
        }
        __tsan_get_report_stack(report, static_cast<unsigned long>(i), r.stacks[i], max_frames);
    }
    r.raw_end = raw_syscall(SYS_lseek, report_fd, 0, SEEK_CUR);
    __atomic_store_n(&S->produced, at + 1, __ATOMIC_RELEASE);
    futex_wake(&S->produced);
    // Like Go, report before the program goes on: a race that is followed by a crash, a kill or an
    // exit is still on the screen. The reporter is another process that needs none of TSan's
    // locks, which this thread holds, so waiting for it is safe; the wait has a limit for a
    // reporter that does not answer.
    long end = now_ms() + 3000;
    while (static_cast<int32_t>(load(&S->written) - (at + 1)) < 0) { // not written yet
        long left = end - now_ms();
        if (left <= 0) {
            break;
        }
        futex_wait(&S->written, load(&S->written), left < 20 ? left : 20);
    }
}

} // namespace

} // namespace gp::race

// __tsan_on_report is called by TSan after it printed a report.
extern "C" GOCPP_NO_TSAN void __tsan_on_report(void* report) {
    gp::race::on_report(report);
}

// __tsan_default_options: gocpp's defaults, then GORACE (see the top). TSan calls it while it
// starts, before its interceptors and before libc has set `environ` (it is null), so it is not
// instrumented, calls no function that TSan intercepts (no strlen, no memcpy, no read: the loops
// are by hand and the file is read with the system calls) and finds GORACE in /proc/self/environ.
// The program's link must export it (-Wl,--export-dynamic-symbol=__tsan_default_options, in
// internal/build/userflags.go): a libtsan that is a shared library only calls an executable's
// definition of a weak symbol that the executable exports.
namespace gp::race {
namespace {

// The helpers of __tsan_default_options, by hand and not lambdas: a lambda is a function of its
// own, which a build with ThreadSanitizer instruments (the runtime's unit tests), and
// ThreadSanitizer calls __tsan_default_options before it can run instrumented code.
GOCPP_NO_TSAN void add_text(char* buf, std::size_t cap, std::size_t& n, const char* text) {
    for (const char* p = text; *p != '\0' && n < cap - 1; p++) {
        buf[n++] = *p;
    }
}

GOCPP_NO_TSAN bool starts_with(const char* v, const char* name) {
    std::size_t k = 0;
    while (name[k] != '\0' && v[k] == name[k]) {
        k++;
    }
    return name[k] == '\0';
}

GOCPP_NO_TSAN bool contains_word(const char* v, const char* word) {
    for (; *v != '\0'; v++) {
        if (starts_with(v, word)) {
            return true;
        }
    }
    return false;
}

} // namespace
} // namespace gp::race

extern "C" GOCPP_NO_TSAN const char* __tsan_default_options() {
    static char buf[1024];
    static char env[1 << 16];
    std::size_t n = 0;
    gp::race::add_text(buf, sizeof buf, n,
                       "detect_deadlocks=0 report_thread_leaks=0 report_signal_unsafe=0 "
                       "die_after_fork=0 atexit_sleep_ms=0 exitcode=66 ");
    long fd = gp::race::raw_syscall(
        SYS_openat, AT_FDCWD, reinterpret_cast<long>("/proc/self/environ"), O_RDONLY | O_CLOEXEC);
    if (fd < 0) {
        buf[n] = '\0';
        return buf;
    }
    std::size_t have = 0;
    for (;;) {
        long got = gp::race::raw_syscall(SYS_read, fd, reinterpret_cast<long>(env + have),
                                         static_cast<long>(sizeof env - 1 - have));
        if (got <= 0) {
            break;
        }
        have += static_cast<std::size_t>(got);
    }
    gp::race::raw_syscall(SYS_close, fd, 0, 0);
    env[have] = '\0';
    // The entries are separated by NUL characters. Wanted: the value of GORACE, and whether TSan's
    // own text is what the program should print (GOCPP_RACE_RAW, or a log_path): the report that
    // gocpp writes needs no symbols from TSan, which are slow (an external symbolizer, a process
    // that TSan starts and talks to while it holds its locks), so TSan is told not to look them up.
    const char* gorace = nullptr;
    bool own_text = false;
    for (std::size_t at = 0; at < have;) {
        const char* v = env + at;
        if (gp::race::starts_with(v, "GORACE=")) {
            gorace = v + 7;
            own_text = own_text || gp::race::contains_word(gorace, "log_path=");
        } else if (gp::race::starts_with(v, "TSAN_OPTIONS=")) {
            own_text = own_text || gp::race::contains_word(v + 13, "log_path=");
        } else if (gp::race::starts_with(v, "GOCPP_RACE_RAW=")) {
            own_text = own_text || (v[15] != '\0' && v[15] != '0');
        }
        while (env[at] != '\0') {
            at++;
        }
        at++;
    }
    if (!own_text) {
        gp::race::add_text(buf, sizeof buf, n, "symbolize=0 ");
    }
    if (gorace != nullptr) {
        gp::race::add_text(buf, sizeof buf, n, gorace);
    }
    buf[n] = '\0';
    return buf;
}

#endif // GOCPP_RACE
