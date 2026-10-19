// A helper header for cpp_error_macro.gp: a function-like macro that only works on a pointer.
#pragma once

struct errmacro_point { int x, y; };
#define POINT_X(p) ((p)->x)
