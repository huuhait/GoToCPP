// phase: 04
//go:build linux

// Helper headers of every kind, and which C++ files of the package folder are built (step 4.16):
// headers with the extensions .hh, .hxx, .h++ and .inl, one in a sub-folder, one whose file name
// is not a Go name (my-lib.h is package my_lib), two with the same file name in different
// folders (renamed), one with no .cpp of its name. plat_linux.cpp is built, and plat_windows.cpp,
// ignored.cpp (//go:build ignore) and _skip.cpp are left out, because each would define a
// function a second time.
package main

import (
	"fmt"

	"./api.hpp"
	"./sub/deep.hh"
	ax "./a/x.hpp"
	bx "./b/x.hpp"
	"./mix.hxx"
	"./my-lib.h"
	"./plus.h++"
	"./table.inl"
)

func main() {
	fmt.Println(deep.deep_value(), mix.mix_value(), plus.plus_value(), table.table_value(), my_lib.lib_value())
	fmt.Println(ax.ax_value(), bx.bx_value(), api.api_value(), C.GoString(api.plat_name()))
}

// Output:
// 1 2 3 4 5
// 6 7 8 linux
