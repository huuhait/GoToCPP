/* ctable.h: a C library that calls back (step 4.11). It is compiled as C by "gocpp run .", with
   no -fexceptions: a panic must not unwind through its frames. */
#ifndef CTABLE_H
#define CTABLE_H

/* ctab_walk calls visit(user, i) for i = 0..n-1 with a malloc'ed scratch buffer alive, adds the
   results up and frees the buffer. ctab_cleanups counts the walks that reached their cleanup. */
int ctab_walk(int n, int (*visit)(void *user, int i), void *user);
int ctab_cleanups(void);

/* The callbacks of a table, called later by name. */
struct ctab_handlers {
    void (*on_open)(const char *name);
    int (*on_data)(void *user, const char *data, int len);
    void *user;
};
void ctab_set_handlers(const struct ctab_handlers *h);
void ctab_open(const char *name);
int ctab_data(const char *data, int len);

#endif
