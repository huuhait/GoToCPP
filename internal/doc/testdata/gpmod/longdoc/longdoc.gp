// Package longdoc has a comment whose lines are longer than the 80 columns
// that the text output wraps at: this sentence has to be broken across several lines by the printer, whatever the width of the source line is.
package longdoc

// Wide does a thing. This paragraph of the comment of a function is also one long source line that the output has to wrap at eighty columns, so that a wrong width changes the golden file.
func Wide() {}
