#include "cppsrc.hpp"
#include <vector>
int cppsrc_triple(int x) { std::vector<int> v(3, x); int s = 0; for (int e : v) s += e; return s; }
