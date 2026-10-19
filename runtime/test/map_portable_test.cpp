// map_swiss_test.cpp with the portable control-byte group (64-bit arithmetic), which the machines
// without SSE2 (arm64 among them) use: the same tests, so that every group implementation is run
// through the whole map.
#define GOCPP_SWISS_PORTABLE 1
#include "map_swiss_test.cpp"
