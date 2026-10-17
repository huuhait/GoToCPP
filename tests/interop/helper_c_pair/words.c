/* words.c: compiled by "gocpp run ." with the C compiler. */
#include "words.h"

#include <ctype.h>
#include <stdlib.h>
#include <string.h>

char *words_upper(const char *s) {
    size_t n = strlen(s);
    char *p = malloc(n + 1);
    for (size_t i = 0; i <= n; i++) {
        p[i] = (char)toupper((unsigned char)s[i]);
    }
    return p;
}

int words_count(const char *s) {
    int n = 0, in = 0;
    for (; *s != '\0'; s++) {
        if (*s == ' ') {
            in = 0;
        } else if (!in) {
            in = 1;
            n++;
        }
    }
    return n;
}

char *words_join(const char *a, const char *b) {
    size_t la = strlen(a), lb = strlen(b);
    char *p = malloc(la + lb + 2);
    memcpy(p, a, la);
    p[la] = ',';
    memcpy(p + la + 1, b, lb + 1);
    return p;
}

void *words_alloc(size_t n) { return malloc(n); }
