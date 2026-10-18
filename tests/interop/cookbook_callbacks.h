// A C library that calls back: each() calls fn(user, i) for i in [0, n), and keeps nothing.
#pragma once

#ifdef __cplusplus
extern "C" {
#endif

typedef void (*each_fn)(void* user, int i);

static inline void each(int n, each_fn fn, void* user) {
    for (int i = 0; i < n; i++) {
        fn(user, i);
    }
}

#ifdef __cplusplus
}
#endif
