// phase: 12
// pkg-config: sqlite3
// error: sqlite3_prepare_v2 is given the address of a char pointer

// The tail that sqlite3_prepare_v2 gives back points into the SQL text.
//
//gocpp:pkg-config sqlite3
package main

import (
	"fmt"

	"<sqlite3.h>"
)

func main() {
	var db *sqlite3.sqlite3
	sqlite3.sqlite3_open(":memory:", &db)
	sql := "SELECT " + fmt.Sprint(1)
	var st *sqlite3.sqlite3_stmt
	var tail *C.char
	sqlite3.sqlite3_prepare_v2(db, sql, -1, &st, &tail)
}
