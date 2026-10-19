// gocpp/rangefunc.hpp: the state of a range-over-func loop (Phase 07, step 7.8), a copy of the
// check that cmd/compile's rangefunc rewrite makes (src/cmd/compile/internal/rangefunc/rewrite.go,
// Go 1.23+). `for x := range seq { body }` calls seq with the loop body as its yield function;
// each loop has one state object, boxed (gp::ptr) so that a yield function that the iterator
// keeps and calls after the loop is over finds it alive:
//
//     {
//         auto gp_rs0 = gp::new_<gp::rangefunc_state>();
//         seq(gp::func<bool(int64_t)>([gp_rs0](int64_t x) mutable -> bool {
//             gp_rs0->gp_enter();                   // misuse panics here
//             ...
//             return gp_rs0->gp_continue();         // the end of the body, `continue`
//             return gp_rs0->gp_exit();             // `break`: DONE, false
//         }));
//         gp_rs0->gp_finish();                      // the loop is over
//     }
//
// The five states and four messages are Go's:
//
//     DONE           the body returned false (break, return, goto, a labeled jump)
//     READY          the body is not running; the iterator may call it
//     PANIC          the body is running, or it panicked
//     EXHAUSTED      the iterator function returned
//     MISSING_PANIC  (a message only) the iterator returned after the body panicked
//
//     gp_enter     (3) at the start of the body: READY -> PANIC, any other state panics with its
//                  message (DONE: "range function continued iteration after function for loop
//                  body returned false"; EXHAUSTED: "... after whole loop exit"; PANIC: "... after
//                  loop body panic")
//     gp_continue  (4) the body ends or continues: -> READY, true
//     gp_exit      (5) the body leaves the loop: -> DONE, false
//     gp_finish    (2) the iterator returned: PANIC means that the body panicked and the iterator
//                  swallowed it ("range function recovered a loop body panic and did not resume
//                  panicking"); then -> EXHAUSTED
//
// gp_next is the loop nest's "what to do after the iterator returns" code of Go's rewrite (only
// the outermost loop's is used): -1 is a return, a positive number a break or continue of an
// outer range-over-func loop, a number below -1 a goto or labeled jump to a place that is not a
// loop of the nest (docs/spec/cpp-mapping.md, "Range over functions").
//
// The checks stay on with -nochecks: they are what keeps a late call of the yield function
// from running the body of a loop that is over.
#pragma once

#include <cstdint>

namespace gp {

// panic_range_state throws the runtime error of a misuse: the state is DONE (0), EXHAUSTED (3),
// PANIC (2) or MISSING_PANIC (4) of the loop (Go's runtime.panicrangestate).
[[noreturn]] void panic_range_state(int state);

struct rangefunc_state {
    static constexpr int gp_done = 0;
    static constexpr int gp_ready = 1;
    static constexpr int gp_panic = 2;
    static constexpr int gp_exhausted = 3;
    static constexpr int gp_missing_panic = 4;

    int gp_state = gp_ready;
    int64_t gp_next = 0;

    void gp_enter() {
        int was = gp_state;
        gp_state = gp_panic;
        if (was != gp_ready) [[unlikely]] {
            panic_range_state(was);
        }
    }
    bool gp_continue() noexcept {
        gp_state = gp_ready;
        return true;
    }
    bool gp_exit() noexcept {
        gp_state = gp_done;
        return false;
    }
    void gp_finish() {
        if (gp_state == gp_panic) [[unlikely]] {
            panic_range_state(gp_missing_panic);
        }
        gp_state = gp_exhausted;
    }
};

} // namespace gp
