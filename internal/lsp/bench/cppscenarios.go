package bench

// cppScenarios are the rows of a file that imports C++ headers (step 12.13d): typing into it, as
// the keystroke row does into a Go file, and asking for hover of its C++ names (the completion
// of a C++ name is empty until the background index is ready, which a row cannot call a failure).
// With a typed-mode server every edit starts the background clang work again for the new text,
// so these are the rows where that work could slow the editor down. They are gocpp only (Go
// cannot read the file) and the package of the file is the project's cpp00.
func cppScenarios(rel string) []scenario {
	keystroke := keystrokeNewAt(ScKeystrokeCpp, rel, "\tbox.w = ", "w", 40, 3)
	hover := position(ScHoverCpp, "textDocument/hover", rel, []mark{
		{"std.map[", 5}, {"counts.size(", 9}, {"std.sort(", 6}, {"box.area(", 6},
		{"shape.perimeter(", 10},
	}, 60)
	scs := []scenario{keystroke, hover}
	for i := range scs {
		scs[i].gocppOnly = true
	}
	return scs
}
