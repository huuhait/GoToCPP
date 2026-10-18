/* A C function that calls back with its own state alive (step 4.11). */
#ifndef CTABLE_H
#define CTABLE_H
int ctab_walk(int n, int (*visit)(void *user, int i), void *user);
#endif
