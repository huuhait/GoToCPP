// phase: 04

// <filesystem>: path operations (`/` joins), stem/extension/filename, create_directories and
// exists in a temporary directory, directory_iterator, remove_all. The namespace is renamed
// like a Go import: import fs "::std::filesystem" is not needed, std.filesystem works too.
package main

import (
	"fmt"
	"unsafe"

	"<algorithm>"
	"<filesystem>"
	"<fstream>"
	"<stdlib.h>"
	"<string>"
	"<vector>"
)

func main() {
	p := std.filesystem.path("/a/b") / "archive.tar.gz"
	fmt.Println(string(p.string()), string(p.filename().string()), string(p.stem().string()), string(p.extension().string()))
	fmt.Println(string(p.parent_path().string()), bool(p.has_extension()))

	// A directory of its own (mkdtemp), so that two runs at the same time don't meet.
	tmpl := C.CString(string(std.filesystem.temp_directory_path().string()) + "/gocpp_zoo_XXXXXX")
	root := std.filesystem.path(std.string(C.GoString(stdlib.mkdtemp(tmpl))))
	C.free(unsafe.Pointer(tmpl))
	std.filesystem.create_directories(root / "sub")
	f := std.ofstream(root / "one.txt")
	f.close()
	fmt.Println(std.filesystem.exists(root/"sub"), std.filesystem.is_directory(root/"sub"), std.filesystem.exists(root/"nope"))

	var names std.vector[std.string]
	for _, e := range std.filesystem.directory_iterator(root) {
		names.push_back(e.path().filename().string())
	}
	std.sort(names.begin(), names.end())
	fmt.Println(string(names[0]), string(names[1]))
	fmt.Println(int(std.filesystem.remove_all(root)))
}

// Output:
// /a/b/archive.tar.gz archive.tar.gz archive.tar .gz
// /a/b true
// true true false
// one.txt sub
// 3
