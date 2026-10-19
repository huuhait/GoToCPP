// C code that leaks a block, for canary_lsan.gp: the only pointer to it is overwritten.
// LeakSanitizer must report the leak at the end of the run.
#pragma once

#include <stdlib.h>

static inline int leak_block(void) {
    static void* volatile keep;
    keep = malloc(77);
    *(volatile char*)keep = 1;
    keep = NULL; // now nothing points to the block
    return 1;
}
