// A helper header for cpp_error_link.gp: a C function that no file of the program defines.
#pragma once

#ifdef __cplusplus
extern "C" {
#endif
int gocpp_test_undefined_fn(int);
#ifdef __cplusplus
}
#endif
