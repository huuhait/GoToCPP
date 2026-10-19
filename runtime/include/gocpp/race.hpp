// gocpp/race.hpp: the hooks of the race detector (gocpp build -race, step 8.12, ADR-062).
//
// A program built with -race is compiled with ThreadSanitizer (the runtime too), which sees every
// access of every thread and the synchronization between them. TSan finds the synchronization
// of the C++ library by itself (std::mutex, std::atomic, thread creation); gocpp's runtime
// builds everything on those (gp::waiter is a mutex and a condition variable, a channel a
// mutex and waiters, sync.Mutex atomics), so no annotation is needed for channels, select,
// WaitGroup or Mutex: the happens-before edges are the ones of the Go memory model.
//
// What this header adds is the Go-shaped part: the OS layer (src/os/linux/race.cpp) turns TSan's
// reports into Go's text ("WARNING: DATA RACE", "Write at 0x... by goroutine 8:", goroutine
// names, `.gp` lines from the #line directives, "Found N data race(s)" at the end), maps GORACE
// to TSan's options, and the thread of each goroutine gets its name.
//
// Two levels: `enabled` says that the compiler has ThreadSanitizer (the runtime's own TSan unit
// tests are built so: TSan's text, as before), and `mode` that the program was built with -race:
// gocpp passes -DGOCPP_RACE_MODE=1 (internal/build/userflags.go), so that every file of the program
// and of the runtime agrees. Without TSan every function here is an empty
// inline function; without the mode the functions that change the runtime's behavior do nothing.
#pragma once

#include <cstddef>
#include <cstdint>

#if defined(__linux__) // the OS layer of the reporter is linux's (src/os/linux/race.cpp)
#if defined(__SANITIZE_THREAD__) || defined(GOCPP_RACE_MODE)
#define GOCPP_RACE 1
#elif defined(__has_feature)
#if __has_feature(thread_sanitizer)
#define GOCPP_RACE 1
#endif
#endif
#endif
#ifndef GOCPP_RACE
#define GOCPP_RACE 0
#endif

#if GOCPP_RACE
extern "C" {
void AnnotateIgnoreSyncBegin(const char* file, int line);
void AnnotateIgnoreSyncEnd(const char* file, int line);
void __tsan_acquire(void* addr);
void __tsan_release(void* addr);
}
#endif

namespace gp::race {

#if GOCPP_RACE

// enabled says that this build has ThreadSanitizer (or is a -race build, whose few files that the
// reporter uses are compiled without it: internal/build/plan.go rtNoTSan).
inline constexpr bool enabled = true;

// mode says that this is a -race build on linux (-DGOCPP_RACE_MODE=1).
#ifdef GOCPP_RACE_MODE
inline constexpr bool mode = true;
#else
inline constexpr bool mode = false;
#endif

// init is called by os::start on the main thread: in the mode it takes the report file of TSan
// away from stderr (the reports are printed in Go's form), starts the reporter, names the main
// thread "goroutine 1", and sets the callback for the end of the process. GOCPP_RACE_RAW=1 keeps
// TSan's own text.
void init() noexcept;

// finish is called by os::exit and os::exit_now before the process ends with code. As Go's os.Exit
// does, a process that ends with 0 after a race ends with 66 (TSan makes it so) and prints "Found N
// data race(s)"; one that ends with another code (a panic, os.Exit(3)) keeps its code, and prints
// no count. Either way the reports that are still to be written are written first.
void finish(int code) noexcept;

// goroutine_started names the calling thread "goroutine <id>": a report names the goroutines.
void goroutine_started(uint64_t id) noexcept;

// finished_goroutine is called by the thread of a goroutine that has ended: in the race mode it
// parks the thread for a while (the last 64 finished goroutines are kept), so that
// ThreadSanitizer keeps the trace of what the goroutine did. release_finished lets them all go.
void finished_goroutine() noexcept;
void release_finished() noexcept;

// reporter_process is the pid of the process that writes the reports (init forks it), or 0.
long reporter_process() noexcept;

// no_sync is a scope in which TSan ignores the synchronization of the atomic operations (and of
// the mutexes) of the calling thread: they still work, but order nothing for the detector. The
// runtime's own bookkeeping that is not Go synchronization (the count of live goroutines, of
// blocked ones) must not make one goroutine's accesses happen before another's: a race between
// them would not be found, and whether it is found would depend on the timing. The data that such
// an operation protects must not be shared with anything TSan checks. It does nothing outside
// the mode.
// acquire and release are Go's race.Acquire and race.Release: the happens-before edge from every
// release of addr to every later acquire of it, for the objects of package sync whose own atomics
// are hidden (no_sync). They do nothing outside the mode.
inline void acquire(const void* addr) noexcept {
    if constexpr (mode) {
        __tsan_acquire(const_cast<void*>(addr));
    }
}
inline void release(const void* addr) noexcept {
    if constexpr (mode) {
        __tsan_release(const_cast<void*>(addr));
    }
}

class no_sync {
public:
    no_sync() noexcept {
        if constexpr (mode) {
            AnnotateIgnoreSyncBegin(__FILE__, __LINE__);
        }
    }
    ~no_sync() {
        if constexpr (mode) {
            AnnotateIgnoreSyncEnd(__FILE__, __LINE__);
        }
    }
    no_sync(const no_sync&) = delete;
    no_sync& operator=(const no_sync&) = delete;
};

#else

inline constexpr bool enabled = false;
inline constexpr bool mode = false;
inline void init() noexcept {}
inline void finish(int) noexcept {}
inline void goroutine_started(uint64_t) noexcept {}
inline long reporter_process() noexcept {
    return 0;
}
inline void finished_goroutine() noexcept {}
inline void release_finished() noexcept {}
inline void acquire(const void*) noexcept {}
inline void release(const void*) noexcept {}

class no_sync {
public:
    no_sync() noexcept {}
    ~no_sync() {}
    no_sync(const no_sync&) = delete;
    no_sync& operator=(const no_sync&) = delete;
};

#endif

} // namespace gp::race
