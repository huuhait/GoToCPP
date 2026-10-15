// A helper header for macro_runtime_clash.gp: a macro named like a name of gocpp's runtime.
#pragma once
#include <stdio.h>

#define Println(x) printf("%d\n", (x))
