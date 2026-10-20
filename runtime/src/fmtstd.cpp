// The standard streams of package os, as fmt sees them (gocpp/fmt.hpp: std_gate, os_std): the gate
// that Print, Println, Printf, Scan, Scanln and Scanf pass, and the cold paths behind it. In a file
// of its own, so that the printer's file keeps the size (and the inlining) it had.
#include <gocpp/os.hpp>
#include <gocpp/rt.hpp>

#include <atomic>
#include <chrono>
#include <mutex>
#include <vector>
#include <thread>

namespace gp::fmt::detail {

// full_fence: a sequentially consistent barrier written as a read-modify-write (a thread fence is
// not understood by ThreadSanitizer, and g++ warns about it).
void full_fence() noexcept {
    static std::atomic<int> dummy{0};
    dummy.fetch_add(0, std::memory_order_seq_cst);
}

// empty_write_error: the write call that Go's File.Write makes for an empty slice, and its error
// (null: none).
const char* empty_write_error() {
    switch (os::write_empty(os::stdout_fd)) {
    case os::write_result::ok:
        return nullptr;
    case os::write_result::broken_pipe:
        os::die_of_sigpipe();
    default:
        return os::write_error();
    }
}

std_gate stdout_gate;
std_gate stdin_gate;

namespace {
std::atomic<print_slot*> slots_head{nullptr};
std::mutex slots_mu;
std::vector<print_slot*> free_slots; // slots of threads that ended

// A thread's slot goes back to the free list when the thread ends.
struct slot_owner {
    print_slot* slot = nullptr;
    ~slot_owner() {
        if (slot != nullptr) {
            slot->inflight.store(0, std::memory_order_relaxed);
            std::lock_guard<std::mutex> g(slots_mu);
            free_slots.push_back(slot);
        }
    }
};
thread_local slot_owner my_slot_owner;

// Where the system has no process-wide barrier, every Print takes the counted slow path.
bool barrier_ok = [] {
    if (!os::process_barrier_init()) {
        stdout_gate.word.fetch_or(1, std::memory_order_relaxed);
        return false;
    }
    return true;
}();
} // namespace

print_slot* register_print_slot() {
    print_slot* s = nullptr;
    {
        std::lock_guard<std::mutex> g(slots_mu);
        if (!free_slots.empty()) {
            s = free_slots.back();
            free_slots.pop_back();
        }
    }
    if (s == nullptr) {
        s = new print_slot;
        s->next = slots_head.load(std::memory_order_relaxed);
        while (!slots_head.compare_exchange_weak(s->next, s, std::memory_order_release,
                                                 std::memory_order_relaxed)) {
        }
    }
    my_slot_owner.slot = s;
    tls_print_slot = s;
    return s;
}

// slots_drained: no thread is inside the direct write of a Print.
bool slots_drained() noexcept {
    for (print_slot* s = slots_head.load(std::memory_order_acquire); s != nullptr; s = s->next) {
        if (s->inflight.load(std::memory_order_acquire) != 0) {
            return false;
        }
    }
    return true;
}

namespace {
// What package os registered for os.Stdout (register_os_stdout): never freed, set once by os's init
// before any other goroutine can use it, and read without a lock.
struct stdout_reg {
    os_std d;
    gp::func<std::tuple<int64_t, gp::error>(std::shared_ptr<void>, std::string_view)> write;
};
std::atomic<const stdout_reg*> stdout_registered{nullptr};
} // namespace

void check_os_file_layout(const void* v0, const void* file0) {
    if (v0 == nullptr || *static_cast<const void* const*>(v0) != file0) {
        gp::detail::fatal("fmt: the layout of os.File is not what fmt expects");
    }
}

void register_os_stdout(
    const os_std& d,
    gp::func<std::tuple<int64_t, gp::error>(std::shared_ptr<void>, std::string_view)> write) {
    stdout_registered.store(new stdout_reg{d, std::move(write)}, std::memory_order_release);
}

// print_slow is print_out after the program changed os.Stdout or closed it: the file the variable
// holds now takes the text, unless it is still the original, which is then written to directly (and
// counted, so that Close of the original waits).
std::tuple<int64_t, gp::error> print_slow(const std::string& s) {
    const stdout_reg* r = stdout_registered.load(std::memory_order_acquire);
    stdout_gate.word.fetch_add(2, std::memory_order_seq_cst);
    std::shared_ptr<void> snap;
    if (r != nullptr) {
        snap = r->d.load(r->d.var);
    }
    if (r == nullptr || !r->d.replaced(snap)) {
        const char* failed = write_stdout_checked(s);
        stdout_gate.word.fetch_sub(2, std::memory_order_seq_cst);
        finish_close(true);
        return failed == nullptr ? written(s) : write_failed(failed, s);
    }
    stdout_gate.word.fetch_sub(2, std::memory_order_seq_cst);
    finish_close(true); // even a replaced-file snapshot can be the last user of the gate
    gp::detail::flush_c_stdio();
    return r->write(snap, s);
}

bool close_drained(bool is_stdout) noexcept {
    const std_gate& g = is_stdout ? stdout_gate : stdin_gate;
    return (g.word.load(std::memory_order_seq_cst) >> 1) == 0 && (!is_stdout || slots_drained());
}

pending_close pending_stdout;
pending_close pending_stdin;

void finish_close(bool is_stdout) {
    pending_close& pc = is_stdout ? pending_stdout : pending_stdin;
    if (!pc.flag.load(std::memory_order_acquire)) {
        return;
    }
    gp::func<void()> f;
    {
        // defer_close holds this through the process-wide barrier. In particular a slow Print
        // that drains the counter during that barrier must not close under an unobserved fast
        // Print. Recheck flag under the lock: another finisher or the caller may have claimed it.
        std::lock_guard<std::mutex> lock(pc.mu);
        full_fence();
        if (close_drained(is_stdout) && pc.flag.exchange(false, std::memory_order_seq_cst)) {
            f = std::move(pc.close);
            pc.close = nullptr;
        }
    }
    if (f != nullptr) {
        f();
    }
}

} // namespace gp::fmt::detail

namespace gp::fmt {

void std_changed() noexcept {
    detail::stdout_gate.word.fetch_or(1, std::memory_order_seq_cst);
    detail::stdin_gate.word.fetch_or(1, std::memory_order_seq_cst);
}

void std_closing(bool is_stdout) noexcept {
    (is_stdout ? detail::stdout_gate : detail::stdin_gate)
        .word.fetch_or(1, std::memory_order_seq_cst);
}

// No worker is started: an in-flight Print/Scan's existing thread claims the retained callback
// in finish_close. It may do so as soon as this function releases the lock, before its caller
// resumes, so retaining the callback is not a synchronous-callback guarantee.
bool defer_close(bool is_stdout, gp::func<void()> close) {
    detail::std_gate& g = is_stdout ? detail::stdout_gate : detail::stdin_gate;
    g.word.fetch_or(1, std::memory_order_seq_cst);
    detail::pending_close& pc = is_stdout ? detail::pending_stdout : detail::pending_stdin;
    std::lock_guard<std::mutex> lock(pc.mu);
    pc.close = std::move(close);
    pc.flag.store(true, std::memory_order_seq_cst);
    if (is_stdout) {
        os::process_barrier(); // every thread's announcement of a Print is visible from here on
    } else {
        detail::full_fence();
    }
    if (detail::close_drained(is_stdout) && pc.flag.exchange(false, std::memory_order_seq_cst)) {
        pc.close = nullptr;
        return false; // nobody is inside a direct read or write: the caller closes at once
    }
    return true; // the last one out closes (finish_close)
}

} // namespace gp::fmt
