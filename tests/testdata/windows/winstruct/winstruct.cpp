#include "winstruct.h"

#include <algorithm>

// With the macros min and max of <windows.h> this would not compile (step 14.4: NOMINMAX).
extern "C" int clamp3(int lo, int v, int hi) {
    return std::min(std::max(v, lo), hi);
}
