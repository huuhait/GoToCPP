// Defined here, declared by no header: main.gp declares them with //gocpp:extern.
#include <stdatomic.h>
#include <stdint.h>
#include <stdio.h>

static atomic_int shown_calls;

// Prints a line and counts it, for the goroutine that main.gp waits for.
void show(int32_t tag, int32_t n) {
	printf("tag %d: %d\n", (int)tag, (int)n);
	fflush(stdout);
	atomic_fetch_add(&shown_calls, 1);
}

int32_t shown(void) { return atomic_load(&shown_calls); }
