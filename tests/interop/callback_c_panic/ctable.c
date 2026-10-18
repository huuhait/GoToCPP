#include "ctable.h"

#include <stdlib.h>

int ctab_walk(int n, int (*visit)(void *user, int i), void *user) {
    int *scratch = malloc(sizeof(int) * (size_t)(n > 0 ? n : 1));
    int sum = 0;
    for (int i = 0; i < n; i++) {
        scratch[i] = visit(user, i);
        sum += scratch[i];
    }
    free(scratch);
    return sum;
}
