/* cbtable.h: C callbacks for the callback tests (Phase 04 step 4.11). It is plain C: a callback
   table in a struct, a function that takes a callback and a user-data pointer, a callback that C
   keeps and calls later, and a call from a thread of C's own. */
#ifndef CBTABLE_H
#define CBTABLE_H

#include <pthread.h>
#include <stdarg.h>
#include <stddef.h>

typedef int (*cb_apply_t)(void *user, int x);

struct cb_ops {
    int (*apply)(void *user, int x);
    void (*done)(void *user);
    void *user;
};

static inline int cb_apply(const struct cb_ops *ops, int x) { return ops->apply(ops->user, x); }
static inline void cb_done(const struct cb_ops *ops) {
    if (ops->done != NULL) {
        ops->done(ops->user);
    }
}

/* cb_sum is f(user, 0) + ... + f(user, n-1). */
static inline int cb_sum(int n, int (*f)(void *, int), void *user) {
    int s = 0;
    for (int i = 0; i < n; i++) {
        s += f(user, i);
    }
    return s;
}

/* cb_register keeps f, and cb_fire calls it later. */
static void (*cb_saved)(int);
static inline void cb_register(void (*f)(int)) { cb_saved = f; }
static inline void cb_fire(int x) {
    if (cb_saved != NULL) {
        cb_saved(x);
    }
}

/* cb_name returns what a callback that returns a pointer gave. */
static inline const char *cb_name(const char *(*f)(int), int x) { return f(x); }

/* cb_logger takes a variadic callback (printf-like): not supported. */
static inline int cb_logger(int (*log)(const char *, ...)) { return log == NULL; }

/* cb_vcall reads a function pointer through `...`: a gocpp func can't go there. */
static inline int cb_vcall(int n, ...) {
    va_list ap;
    va_start(ap, n);
    int (*f)(void *, int) = va_arg(ap, int (*)(void *, int));
    va_end(ap);
    return f(NULL, n);
}

/* cb_ud returns what f gives back for ud: a callback may return the pointer it was given. */
static inline void *cb_ud(void *(*f)(void *), void *ud) { return f(ud); }

/* cb_get calls a callback that returns a function pointer, and calls that pointer. */
typedef int (*cb_unary_t)(int);
static inline int cb_get(cb_unary_t (*get)(void), int x) { return get()(x); }

struct cb_job {
    int (*f)(void *, int);
    void *user;
    int x;
    int out;
};
static void *cb_job_main(void *p) {
    struct cb_job *j = (struct cb_job *)p;
    j->out = j->f(j->user, j->x);
    return NULL;
}
/* cb_in_thread calls f(user, x) on a new thread of C's own and returns its result. */
static inline int cb_in_thread(int (*f)(void *, int), void *user, int x) {
    pthread_t t;
    struct cb_job j = {f, user, x, 0};
    pthread_create(&t, NULL, cb_job_main, &j);
    pthread_join(t, NULL);
    return j.out;
}

#endif
