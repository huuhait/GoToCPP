// The native functions of the packages runtime and runtime/debug (gocpp/runtimepkg.hpp, step
// 8.10). Standard C++ only: the OS is reached through gp::os (gocpp/os.hpp).
#include <gocpp/rt.hpp>
#include <gocpp/goroutine.hpp>
#include <gocpp/os.hpp>
#include <gocpp/trace.hpp>

#include <algorithm>
#include <atomic>
#include <cmath>
#include <cstdint>
#include <cstdlib>
#include <limits>
#include <mutex>
#include <random>
#include <string>
#include <string_view>
#include <thread>
#include <tuple>
#include <vector>

namespace gp::runtime {

namespace {

// The GOMAXPROCS setting: 0 until the first function that needs it runs.
constinit std::atomic<int64_t> procs{0};

// parse_positive reads a decimal number that is more than 0 and fits, as Go reads the
// GOMAXPROCS variable (atoi32); anything else is 0.
int64_t parse_positive(const char* s) {
    if (s == nullptr || *s == '\0') {
        return 0;
    }
    int64_t n = 0;
    for (; *s != '\0'; s++) {
        if (*s < '0' || *s > '9') {
            return 0;
        }
        n = n * 10 + (*s - '0');
        if (n > std::numeric_limits<int32_t>::max()) {
            return 0;
        }
    }
    return n;
}

// initial is the setting a program starts with: GOMAXPROCS from the environment, else the default.
int64_t initial() {
    if (int64_t n = parse_positive(std::getenv("GOMAXPROCS")); n > 0) {
        return n;
    }
    return default_gomaxprocs();
}

// ensure_procs makes the setting exist and returns it.
int64_t ensure_procs() {
    int64_t cur = procs.load(std::memory_order_acquire);
    if (cur == 0) {
        int64_t want = initial();
        if (procs.compare_exchange_strong(cur, want, std::memory_order_acq_rel)) {
            return want;
        }
        // another thread set it first: cur is its value
    }
    return cur;
}

constinit std::atomic<int64_t> mutex_profile_fraction{0};

// The settings of runtime/debug.
constinit std::atomic<int64_t> gc_percent{-2}; // -2: not read from GOGC yet
constinit std::atomic<int64_t> memory_limit{-2};
constinit std::atomic<int64_t> max_stack{1000000000};
constinit std::atomic<int64_t> max_threads{10000};
constinit std::atomic<bool> panic_on_fault{false};

// parse_gogc reads GOGC: a number, or "off" (-1); 100 when it is not set or not valid.
int64_t parse_gogc() {
    const char* s = std::getenv("GOGC");
    if (s == nullptr || *s == '\0') {
        return 100;
    }
    if (std::string_view(s) == "off") {
        return -1;
    }
    bool neg = *s == '-';
    int64_t n = parse_positive(neg ? s + 1 : s);
    if (n == 0 && !(s[neg ? 1 : 0] == '0' && s[neg ? 2 : 1] == '\0')) {
        return 100;
    }
    return neg ? -n : n;
}

// parse_memlimit reads GOMEMLIMIT: a number of bytes with an optional B, KiB, MiB, GiB or TiB, or
// "off"; MaxInt64 (no limit) when it is not set or not valid.
int64_t parse_memlimit() {
    constexpr int64_t none = std::numeric_limits<int64_t>::max();
    const char* env = std::getenv("GOMEMLIMIT");
    if (env == nullptr || *env == '\0') {
        return none;
    }
    std::string_view s(env);
    if (s == "off") {
        return none;
    }
    std::size_t i = 0;
    int64_t n = 0;
    for (; i < s.size() && s[i] >= '0' && s[i] <= '9'; i++) {
        if (n > (std::numeric_limits<int64_t>::max() - 9) / 10) {
            return none;
        }
        n = n * 10 + (s[i] - '0');
    }
    if (i == 0) {
        return none;
    }
    std::string_view unit = s.substr(i);
    int shift = -1;
    if (unit.empty() || unit == "B") {
        shift = 0;
    } else if (unit == "KiB") {
        shift = 10;
    } else if (unit == "MiB") {
        shift = 20;
    } else if (unit == "GiB") {
        shift = 30;
    } else if (unit == "TiB") {
        shift = 40;
    }
    if (shift < 0 || n > (std::numeric_limits<int64_t>::max() >> shift)) {
        return none;
    }
    return n << shift;
}

// swap_setting stores value in the setting and returns the old one; the first call reads the
// environment (init).
int64_t swap_setting(std::atomic<int64_t>& setting, int64_t value, int64_t (*init)()) {
    int64_t cur = setting.load(std::memory_order_acquire);
    while (cur == -2) {
        int64_t want = init();
        if (setting.compare_exchange_weak(cur, want, std::memory_order_acq_rel)) {
            cur = want;
        }
    }
    if (value == -2) {
        return cur; // only to read
    }
    return setting.exchange(value, std::memory_order_acq_rel);
}

// The table of Func values (stdlib/runtime).
// (No constinit: with the posix thread model of MinGW-w64 (WinLibs, MSYS2) std::mutex has no
// constexpr constructor; with libstdc++ on Linux and the mcf model it is constant-initialized
// anyway.)
std::mutex funcs_mu;

} // namespace

void gosched() noexcept {
    std::this_thread::yield();
}

int64_t num_goroutine() noexcept {
    return gp::num_goroutine();
}

int64_t adjust_for_cgroup(int64_t ncpu, double limit) noexcept {
    if (limit > 0) {
        // Go 1.25: the limit rounded up, and at least 2 (a limit of 1.5 CPUs or less is 2).
        auto l = static_cast<int64_t>(std::ceil(limit));
        l = l < 2 ? 2 : l;
        if (l < ncpu) {
            return l;
        }
    }
    return ncpu;
}

int64_t default_gomaxprocs() noexcept {
    return adjust_for_cgroup(num_cpu(), os::cpu_limit());
}

int64_t gomaxprocs(int64_t n) noexcept {
    int64_t cur = ensure_procs();
    if (n < 1) {
        return cur;
    }
    return procs.exchange(n, std::memory_order_acq_rel);
}

void set_default_gomaxprocs() noexcept {
    procs.store(default_gomaxprocs(), std::memory_order_release);
}

int64_t num_cpu() noexcept {
    static const int64_t n = os::cpu_count();
    return n;
}

int64_t num_cgo_call() noexcept {
    return 0;
}

void lock_os_thread() noexcept {
    detail::this_goroutine().locked++; // only for the deadlock report: "locked to thread"
}
void unlock_os_thread() noexcept {
    detail::goroutine& g = detail::this_goroutine();
    if (g.locked > 0) {
        g.locked--;
    }
}

void gc() noexcept {}
void set_finalizer(const gp::any&, const gp::any&) noexcept {}
void keep_alive(const gp::any&) noexcept {}

std::tuple<uint64_t, uint64_t, uint64_t> mem_info() noexcept {
    os::mem_stats m = os::mem_info();
    return {m.gp_in_use, m.gp_sys, m.gp_idle};
}

int64_t set_mutex_profile_fraction(int64_t rate) noexcept {
    if (rate < 0) {
        return mutex_profile_fraction.load(std::memory_order_acquire);
    }
    return mutex_profile_fraction.exchange(rate, std::memory_order_acq_rel);
}

gp::string version() {
    return gp::string::gp_copy("go1.25.14");
}

gp::string goroot() {
    const char* s = std::getenv("GOROOT");
    return gp::string::gp_copy(s != nullptr ? s : "");
}

std::tuple<uint64_t, gp::string, int64_t, bool> caller(int64_t skip) {
    if (skip < 0) {
        return {0, gp::string(), 0, false};
    }
    std::vector<uintptr_t> pcs = detail::shown_frames(static_cast<std::size_t>(skip) + 1);
    if (pcs.size() <= static_cast<std::size_t>(skip)) {
        return {0, gp::string(), 0, false};
    }
    uintptr_t pc = pcs[static_cast<std::size_t>(skip)];
    detail::frame_info fi;
    detail::resolve_pcs(&pc, 1, true, &fi);
    if (!fi.ok) {
        return {0, gp::string(), 0, false};
    }
    return {pc, gp::string::gp_copy(fi.file.empty() ? "?" : fi.file), fi.line, true};
}

int64_t callers(int64_t skip, gp::slice<uint64_t> out) {
    int64_t room = out.gp_len();
    if (room <= 0) {
        return 0;
    }
    if (skip < 0) {
        skip = 0;
    }
    std::size_t n = 0;
    auto put = [&](uintptr_t pc) {
        if (static_cast<int64_t>(n) < room) {
            out.gp_data()[n++] = pc;
        }
    };
    if (skip == 0) {
        // Frame 0 is Callers itself: an address inside this function (resolve_pcs names it).
        put(reinterpret_cast<uintptr_t>(&callers) + 1);
    }
    std::size_t lskip = skip > 0 ? static_cast<std::size_t>(skip) - 1 : 0;
    std::vector<uintptr_t> pcs = detail::shown_frames(lskip + static_cast<std::size_t>(room));
    for (std::size_t k = lskip; k < pcs.size(); k++) {
        put(pcs[k]);
    }
    return static_cast<int64_t>(n);
}

void pc_infos(gp::slice<uint64_t> pcs, bool call, gp::slice<gp::string> names,
              gp::slice<gp::string> files, gp::slice<int64_t> lines, gp::slice<uint64_t> entries) {
    int64_t n =
        std::min({pcs.gp_len(), names.gp_len(), files.gp_len(), lines.gp_len(), entries.gp_len()});
    if (n <= 0) {
        return;
    }
    std::vector<uintptr_t> in(pcs.gp_data(), pcs.gp_data() + n);
    std::vector<detail::frame_info> info(static_cast<std::size_t>(n));
    detail::resolve_pcs(in.data(), in.size(), call, info.data());
    for (std::size_t i = 0; i < in.size(); i++) {
        const detail::frame_info& f = info[i];
        names.gp_data()[i] = gp::string::gp_copy(f.ok ? f.fn.name : std::string());
        files.gp_data()[i] = gp::string::gp_copy(f.file);
        lines.gp_data()[i] = f.line;
        entries.gp_data()[i] = f.ok ? f.entry : 0;
    }
}

void funcs_lock() noexcept {
    funcs_mu.lock();
}

void funcs_unlock() noexcept {
    funcs_mu.unlock();
}

int64_t stack(gp::slice<uint8_t> buf, bool) {
    std::string out = "goroutine ";
    out += std::to_string(detail::goroutine_id());
    out += " [running]:\n";
    // The whole stack of this thread, to its end, for Go's "...N frames elided..." lines.
    std::vector<uintptr_t> all;
    bool complete = false;
    for (std::size_t size = 1024; size <= (std::size_t(1) << 20) && !complete; size *= 8) {
        all.resize(size);
        int n = os::stack(all.data(), static_cast<int>(size));
        all.resize(static_cast<std::size_t>(n < 0 ? 0 : n));
        complete = static_cast<std::size_t>(n) < size;
    }
    detail::append_trace(out, all.data(), all.size(), complete, detail::program_image());
    auto n = std::min<std::size_t>(out.size(), static_cast<std::size_t>(buf.gp_len()));
    for (std::size_t i = 0; i < n; i++) {
        buf.gp_data()[i] = static_cast<uint8_t>(out[i]);
    }
    return static_cast<int64_t>(n);
}

void set_panic_values(const gp::any& nil_error, gp::func<gp::any(gp::string)> assertion) {
    detail::set_panic_values(nil_error, std::move(assertion));
}

int64_t set_gc_percent(int64_t percent) noexcept {
    int64_t v = percent < 0 ? -1 : percent;
    return swap_setting(gc_percent, v, parse_gogc);
}

int64_t set_memory_limit(int64_t limit) noexcept {
    if (limit < 0) {
        return swap_setting(memory_limit, -2, parse_memlimit);
    }
    return swap_setting(memory_limit, limit, parse_memlimit);
}

uint64_t splitmix64(uint64_t& state) noexcept {
    state += 0x9E3779B97F4A7C15ull;
    uint64_t z = state;
    z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9ull;
    z = (z ^ (z >> 27)) * 0x94D049BB133111EBull;
    return z ^ (z >> 31);
}

uint64_t runtime_rand() noexcept {
    // The state is per thread; its seed mixes two draws of the system's source with the address of
    // the state.
    thread_local uint64_t state = [] {
        std::random_device rd;
        uint64_t a = rd(), b = rd();
        thread_local char anchor;
        return (a << 32) ^ b ^ reinterpret_cast<uintptr_t>(&anchor) ^ 0x9E3779B97F4A7C15ull;
    }();
    return splitmix64(state);
}

thread_local uintptr_t stack_floor = 0;

int64_t stack_room_slow() noexcept {
    // The lowest address that a frame may use plus the reserve, worked out once for each thread
    // (the stack of a thread does not move). A thread whose stack is not known (yet) asks again
    // each time.
    uintptr_t lo = 0, hi = 0;
    if (!gp::os::stack_bounds(lo, hi)) {
        return std::numeric_limits<int64_t>::max();
    }
    auto size = static_cast<int64_t>(hi - lo);
    stack_floor = lo + static_cast<uintptr_t>(
                           std::clamp<int64_t>(size / 16, int64_t(128) << 10, int64_t(512) << 10));
    return stack_room();
}

int64_t set_max_stack(int64_t bytes) noexcept {
    return max_stack.exchange(bytes, std::memory_order_acq_rel);
}

int64_t set_max_threads(int64_t threads) noexcept {
    return max_threads.exchange(threads, std::memory_order_acq_rel);
}

bool set_panic_on_fault(bool enabled) noexcept {
    return panic_on_fault.exchange(enabled, std::memory_order_acq_rel);
}

void set_traceback(const gp::string&) noexcept {}

void free_os_memory() noexcept {
    os::trim_memory();
}

void write_stderr(gp::slice<uint8_t> bytes) noexcept {
    if (bytes.gp_len() > 0) {
        gp::detail::write_stderr(std::string_view(reinterpret_cast<const char*>(bytes.gp_data()),
                                                  static_cast<std::size_t>(bytes.gp_len())));
    }
}

} // namespace gp::runtime
