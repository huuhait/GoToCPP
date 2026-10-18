//gocpp:cflags -DCSRC_BASE=1
//gocpp:link m
package csrc

import "./csrc.h"

func Twice(x int) int { return int(csrc.csrc_twice(C.int(x))) }

func Root(x float64) float64 { return float64(csrc.csrc_root(C.double(x))) }
