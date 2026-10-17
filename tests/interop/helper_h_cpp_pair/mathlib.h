// mathlib.h: it parses as C, so gocpp wraps it in extern "C" unless //gocpp:no-extern-c says
// that it is C++ (the directive of main.gp).
#pragma once

int add(int a, int b);
int twice(int a);
