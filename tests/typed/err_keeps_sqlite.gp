// phase: 12
// pkg-config: sqlite3
// error: sqlite3_bind_text keeps the pointer that it is given

// SQLITE_STATIC says that the bytes live as long as the statement, which a gocpp string's do not.
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
	var st *sqlite3.sqlite3_stmt
	sqlite3.sqlite3_prepare_v2(db, "SELECT ?1", -1, &st, nil)
	name := "ann" + fmt.Sprint(1)
	sqlite3.sqlite3_bind_text(st, 1, name, -1, sqlite3.SQLITE_STATIC)
}
