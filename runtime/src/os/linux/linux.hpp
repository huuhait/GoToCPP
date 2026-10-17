// Shared by the files of the Linux OS layer (os.cpp, thread.cpp); not a header of the runtime.
#pragma once

#include <gocpp/os.hpp>

// GOCPP_LSAN: a build with AddressSanitizer (it has LeakSanitizer's interface).
#if defined(__SANITIZE_ADDRESS__)
#define GOCPP_LSAN 1
#elif defined(__has_feature)
#if __has_feature(address_sanitizer)
#define GOCPP_LSAN 1
#endif
#endif

// GOCPP_OWN_SEGV: the runtime installs its SIGSEGV handler (stack overflow, thread.cpp). Not
// with a sanitizer, which has its own handler and its own report of a stack overflow.
#if !defined(GOCPP_LSAN) && !defined(__SANITIZE_THREAD__)
#define GOCPP_OWN_SEGV 1
#if defined(__has_feature)
#if __has_feature(thread_sanitizer) || __has_feature(memory_sanitizer)
#undef GOCPP_OWN_SEGV
#endif
#endif
#endif

namespace gp::os {

// start_threads is what start does for the thread layer (thread.cpp): the main thread's stack
// bounds and the SIGSEGV handler.
void start_threads() noexcept;

} // namespace gp::os
