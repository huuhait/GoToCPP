// go/types finds the mismatch first and the unused variables last; go
// build sorts them by column.
package samepos

func f() {
	a, b := 1
}
