/* ctable.c: compiled by "gocpp run ." with the C compiler. */
#include "ctable.h"

#include <stdlib.h>
#include <stddef.h>

static int cleanups;
static struct ctab_handlers handlers;

int ctab_walk(int n, int (*visit)(void *user, int i), void *user) {
    int *scratch = malloc(sizeof(int) * (size_t)(n > 0 ? n : 1));
    int sum = 0;
    for (int i = 0; i < n; i++) {
        scratch[i] = visit(user, i);
        sum += scratch[i];
    }
    free(scratch);
    cleanups++;
    return sum;
}

int ctab_cleanups(void) { return cleanups; }

void ctab_set_handlers(const struct ctab_handlers *h) { handlers = *h; }

void ctab_open(const char *name) {
    if (handlers.on_open != NULL) {
        handlers.on_open(name);
    }
}

int ctab_data(const char *data, int len) {
    return handlers.on_data != NULL ? handlers.on_data(handlers.user, data, len) : -1;
}
