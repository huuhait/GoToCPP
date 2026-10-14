// std::filesystem, through a renamed namespace and by its full name.
package main

import (
	"fmt"

	"<filesystem>"
	fs "::std::filesystem"
)

type Walker struct {
	root  fs.path
	count int
}

func (w *Walker) visit(p std.filesystem.path) {
	if fs.is_regular_file(p) {
		w.count++
	}
}

func list(dir fs.path) []std.filesystem.path {
	var out []std.filesystem.path
	for _, entry := range fs.directory_iterator(dir) {
		out = append(out, entry.path())
	}
	return out
}

func main() {
	root := fs.path("/tmp")
	w := &Walker{root: root}
	if fs.exists(root) && fs.is_directory(root) {
		for _, p := range list(root) {
			w.visit(p)
		}
	}
	child := root / "x.txt"
	fmt.Println(w.count, child.string(), child.extension().string())
	var opts fs.copy_options = fs.copy_options.recursive
	_ = opts
	size := int64(fs.file_size(child))
	fmt.Println(size > 0, fs.temp_directory_path().string())
}
