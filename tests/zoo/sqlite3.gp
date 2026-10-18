// phase: 04
// pkg-config: sqlite3

// sqlite3 in opaque mode: open an in-memory database, prepare/bind/step/column with typed C
// pointers (*sqlite3.sqlite3, *sqlite3.sqlite3_stmt), the SQLITE_* macros, C strings both ways
// (C.cstr, C.GoString; sqlite3_column_text returns unsigned char*), and an error code.
//
//gocpp:pkg-config sqlite3
package main

import (
	"fmt"

	"<sqlite3.h>"
)

func main() {
	var db *sqlite3.sqlite3
	if rc := sqlite3.sqlite3_open(":memory:", &db); rc != sqlite3.SQLITE_OK {
		panic("open failed")
	}
	defer sqlite3.sqlite3_close(db)

	sqlite3.sqlite3_exec(db, "CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT, score REAL)", nil, nil, nil)
	var ins *sqlite3.sqlite3_stmt
	sqlite3.sqlite3_prepare_v2(db, "INSERT INTO t (name, score) VALUES (?1, ?2)", -1, &ins, nil)
	for i, name := range []string{"ann", "bo", "cy"} {
		sqlite3.sqlite3_bind_text(ins, 1, C.cstr(name), -1, sqlite3.SQLITE_TRANSIENT)
		sqlite3.sqlite3_bind_double(ins, 2, C.double(i)+0.5)
		sqlite3.sqlite3_step(ins)
		sqlite3.sqlite3_reset(ins)
	}
	sqlite3.sqlite3_finalize(ins)
	fmt.Println("rows:", int(sqlite3.sqlite3_last_insert_rowid(db)), "changes:", int(sqlite3.sqlite3_changes(db)))

	var q *sqlite3.sqlite3_stmt
	sqlite3.sqlite3_prepare_v2(db, "SELECT id, name, score FROM t WHERE score > ?1 ORDER BY id", -1, &q, nil)
	sqlite3.sqlite3_bind_double(q, 1, 0.9)
	for sqlite3.sqlite3_step(q) == sqlite3.SQLITE_ROW {
		fmt.Println(int(sqlite3.sqlite3_column_int(q, 0)), C.GoString(sqlite3.sqlite3_column_text(q, 1)), float64(sqlite3.sqlite3_column_double(q, 2)))
	}
	sqlite3.sqlite3_finalize(q)

	rc := sqlite3.sqlite3_exec(db, "SELEC nonsense", nil, nil, nil)
	fmt.Println(rc == sqlite3.SQLITE_ERROR, C.GoString(sqlite3.sqlite3_errmsg(db)))
	fmt.Println(sqlite3.sqlite3_libversion_number() >= 3000000)
}

// Output:
// rows: 3 changes: 1
// 2 bo 1.5
// 3 cy 2.5
// true near "SELEC": syntax error
// true
