// What -D and -std change.
#pragma once
#ifdef FEATURE_X
int feature_x(void);
#define FEATURE_NAME "x"
#else
int no_feature(void);
#define FEATURE_NAME "none"
#endif
#ifdef LEVEL
#define LEVEL_VALUE LEVEL
#endif
#if __cplusplus > 202002L
#define STANDARD 23
#else
#define STANDARD 20
#endif
#include <local_dep.h>
