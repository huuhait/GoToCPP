/* words.h: a C helper header with a .c file next to it (step 4.16). A .h file that parses as C
   is wrapped in extern "C" by gocpp (step 4.14). The package is words. */
#ifndef WORDS_H
#define WORDS_H

#include <stddef.h>

/* words_upper returns a malloc'ed upper-case copy of s; the caller frees it (C.free). Under
   -asan a missing free is a leak. */
char *words_upper(const char *s);

/* words_count counts the words of s, which are separated by spaces. */
int words_count(const char *s);

/* words_join returns "a,b" in a malloc'ed buffer. */
char *words_join(const char *a, const char *b);

/* words_alloc returns n bytes of malloc'ed memory as a void*. */
void *words_alloc(size_t n);

#endif
