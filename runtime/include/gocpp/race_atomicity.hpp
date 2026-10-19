// gocpp/race_atomicity.hpp: how -race (ThreadSanitizer) sees the reference counts. Only used in a
// program built with -race on linux (-DGOCPP_RACE_MODE=1): gocpp/nofortify.h, the first header of
// every compile of the program and of the runtime, includes it, and nothing else does.
//
// Every gp::ptr is a std::shared_ptr, whose count is changed by an atomic read-modify-write with
// acquire and release order, and TSan takes each of them for synchronization: a goroutine that
// copies or drops a pointer would happen after every goroutine that did before it, and a race
// between two goroutines that both touch the same object would be hidden (or found, by chance of
// the timing). Go's memory model has no such edge: a pointer copy orders nothing. So the library's
// atomic helpers (ext/atomicity.h: it is only a handful of inline functions that
// shared_ptr uses for its counts) are replaced by the same operations inside TSan's
// ignore-synchronization scope (gp::race::no_sync), and the only edges left are the ones that
// the library itself explains to TSan with _GLIBCXX_SYNCHRONIZATION_HAPPENS_BEFORE/AFTER (every
// drop before the last one, which frees the object): they are defined here as
// __tsan_release and __tsan_acquire. The operations are the library's own (acquire and release
// order, so the program's real behavior is unchanged).
//
// This relies on the name of the library's include guard (_GLIBCXX_ATOMICITY_H), so that its
// header is skipped; the race tests (tests/race) fail if a library change stops that from working.
#pragma once

// Every file of a -race build agrees on this (gocpp/race.hpp: gp::race::mode).
#ifndef GOCPP_RACE_MODE
#define GOCPP_RACE_MODE 1
#endif

#if !__has_include(<bits/c++config.h>)
#error "-race needs libstdc++ (the race detector changes how its shared_ptr counts)"
#endif
#include <bits/c++config.h>

#if !defined(__GLIBCXX__)
#error "-race needs libstdc++ (the race detector changes how its shared_ptr counts)"
#endif

extern "C" {
void __tsan_acquire(void* addr);
void __tsan_release(void* addr);
void AnnotateIgnoreSyncBegin(const char* file, int line);
void AnnotateIgnoreSyncEnd(const char* file, int line);
}

// c++config.h defines both as nothing; theirs are replaced.
#undef _GLIBCXX_SYNCHRONIZATION_HAPPENS_BEFORE
#undef _GLIBCXX_SYNCHRONIZATION_HAPPENS_AFTER
#define _GLIBCXX_SYNCHRONIZATION_HAPPENS_BEFORE(A)                                                 \
    __tsan_release(const_cast<void*>(static_cast<const volatile void*>(A)))
#define _GLIBCXX_SYNCHRONIZATION_HAPPENS_AFTER(A)                                                  \
    __tsan_acquire(const_cast<void*>(static_cast<const volatile void*>(A)))

#define _GLIBCXX_ATOMICITY_H 1

#include <bits/gthr.h>
#include <bits/atomic_word.h>
// glibc's <sys/single_threaded.h> declares this flag; the declaration is repeated here because OS
// headers may only be included from runtime/src/os/. Without glibc the answer is always "no".
#if defined(__GLIBC__)
extern "C" char __libc_single_threaded;
#endif

namespace __gnu_cxx {

__attribute__((__always_inline__)) inline bool __is_single_threaded() noexcept {
#if defined(__GLIBC__)
    return ::__libc_single_threaded;
#else
    return false;
#endif
}

__attribute__((__always_inline__)) inline _Atomic_word
__exchange_and_add(volatile _Atomic_word* mem, int val) {
    AnnotateIgnoreSyncBegin(__FILE__, __LINE__);
    _Atomic_word old = __atomic_fetch_add(mem, val, __ATOMIC_ACQ_REL);
    AnnotateIgnoreSyncEnd(__FILE__, __LINE__);
    return old;
}

__attribute__((__always_inline__)) inline void __atomic_add(volatile _Atomic_word* mem, int val) {
    __exchange_and_add(mem, val);
}

__attribute__((__always_inline__)) inline _Atomic_word __exchange_and_add_single(_Atomic_word* mem,
                                                                                 int val) {
    _Atomic_word result = *mem;
    *mem += val;
    return result;
}

__attribute__((__always_inline__)) inline void __atomic_add_single(_Atomic_word* mem, int val) {
    *mem += val;
}

__attribute__((__always_inline__)) inline _Atomic_word
__exchange_and_add_dispatch(_Atomic_word* mem, int val) {
    if (__is_single_threaded()) {
        return __exchange_and_add_single(mem, val);
    }
    return __exchange_and_add(mem, val);
}

__attribute__((__always_inline__)) inline void __atomic_add_dispatch(_Atomic_word* mem, int val) {
    if (__is_single_threaded()) {
        __atomic_add_single(mem, val);
    } else {
        __atomic_add(mem, val);
    }
}

} // namespace __gnu_cxx

#ifndef _GLIBCXX_READ_MEM_BARRIER
#define _GLIBCXX_READ_MEM_BARRIER __atomic_thread_fence(__ATOMIC_ACQUIRE)
#endif
#ifndef _GLIBCXX_WRITE_MEM_BARRIER
#define _GLIBCXX_WRITE_MEM_BARRIER __atomic_thread_fence(__ATOMIC_RELEASE)
#endif
