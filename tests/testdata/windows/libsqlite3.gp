// phase: 14

// sqlite3 built for MinGW-w64 (step 14.4, ADR-082): the zoo program of tests/zoo/sqlite3.gp on Windows. The library is
// a static libsqlite3.a (GOCPP_WINDOWS_LIBS, tests/README.md), named with //gocpp:link.

//gocpp:link sqlite3

package main

import (
	"fmt"

	"<sqlite3.h>"
	"<windows.h>"
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
	for i, name := range []string{"ann", "bö", "世界"} {
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

	// A database file in the current folder (the path is a UTF-8 C string; sqlite3 converts it to UTF-16).
	var file *sqlite3.sqlite3
	rc := sqlite3.sqlite3_open("gocpp-sqlite-\u00e9.db", &file)
	fmt.Println("open file:", rc == sqlite3.SQLITE_OK)
	sqlite3.sqlite3_exec(file, "CREATE TABLE a (x)", nil, nil, nil)
	sqlite3.sqlite3_close(file)
	// The C runtime's remove() reads its name in the ANSI code page, so the file is deleted by a W function.
	fmt.Println("file removed:", windows.DeleteFileW(C.wstr("gocpp-sqlite-\u00e9.db")) != 0)

	rc = sqlite3.sqlite3_exec(db, "SELEC nonsense", nil, nil, nil)
	fmt.Println(rc == sqlite3.SQLITE_ERROR, C.GoString(sqlite3.sqlite3_errmsg(db)))
	fmt.Println(sqlite3.sqlite3_libversion_number() >= 3000000)
}
