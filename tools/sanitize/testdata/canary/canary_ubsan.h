// C code with undefined behavior, for canary_ubsan.gp: a shift by more than the width of int.
// UndefinedBehaviorSanitizer must report it (and nothing else goes wrong: the program goes on).
#pragma once

static inline int shift_too_far(int x, int n) {
    return x << n;
}
