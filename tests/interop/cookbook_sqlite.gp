// phase: 04
// pkg-config: sqlite3

//gocpp:pkg-config sqlite3
package main

import (
	"fmt"

	"<sqlite3.h>"
)

// check stops at the first error, with the library's own message.
func check(db *sqlite3.sqlite3, rc C.int) {
	if rc != sqlite3.SQLITE_OK && rc != sqlite3.SQLITE_ROW && rc != sqlite3.SQLITE_DONE {
		panic(C.GoString(sqlite3.sqlite3_errmsg(db)))
	}
}

func main() {
	// sqlite3_open writes the new handle through a **sqlite3: &db is that out parameter.
	var db *sqlite3.sqlite3
	check(db, sqlite3.sqlite3_open(":memory:", &db))
	defer sqlite3.sqlite3_close(db)

	check(db, sqlite3.sqlite3_exec(db, "CREATE TABLE fruit (name TEXT, qty INTEGER)", nil, nil, nil))

	// A prepared statement: bind Go values, step, reset.
	var ins *sqlite3.sqlite3_stmt
	check(db, sqlite3.sqlite3_prepare_v2(db, "INSERT INTO fruit VALUES (?1, ?2)", -1, &ins, nil))
	for i, name := range []string{"apple", "banana", "cherry"} {
		// SQLITE_TRANSIENT makes sqlite copy the text, so C.cstr's pointer need not outlive the call.
		sqlite3.sqlite3_bind_text(ins, 1, C.cstr(name), -1, sqlite3.SQLITE_TRANSIENT)
		sqlite3.sqlite3_bind_int(ins, 2, C.int((i+1)*10))
		check(db, sqlite3.sqlite3_step(ins))
		sqlite3.sqlite3_reset(ins)
	}
	sqlite3.sqlite3_finalize(ins)

	// A query: sqlite3_column_text returns an unsigned char*, which C.GoString copies into a string.
	var q *sqlite3.sqlite3_stmt
	check(db, sqlite3.sqlite3_prepare_v2(db, "SELECT name, qty FROM fruit WHERE qty > ?1 ORDER BY qty", -1, &q, nil))
	sqlite3.sqlite3_bind_int(q, 1, 10)
	for sqlite3.sqlite3_step(q) == sqlite3.SQLITE_ROW {
		fmt.Println(C.GoString(sqlite3.sqlite3_column_text(q, 0)), int(sqlite3.sqlite3_column_int(q, 1)))
	}
	sqlite3.sqlite3_finalize(q)
}

// Output:
// banana 20
// cherry 30
