// A header in a sub-folder: "./sub/deep.hh" is package deep. Only the package folder's own C and
// C++ files are compiled, so everything in it is inline.
#pragma once
inline int deep_value() { return 1; }
