// phase: 12
// pkg-config: sqlite3
// opaque: reject

// sqlite3 with gocpp strings in variables for every const char* parameter (the SQL text, the
// text to bind, the file name), C numbers into gocpp numbers and the reverse.
//
//gocpp:pkg-config sqlite3
package main

import (
	"fmt"

	"<sqlite3.h>"
)

func main() {
	file := ":memory:"
	var db *sqlite3.sqlite3
	if rc := sqlite3.sqlite3_open(file, &db); rc != sqlite3.SQLITE_OK {
		panic("open failed")
	}
	defer sqlite3.sqlite3_close(db)

	create := "CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT, score REAL)"
	sqlite3.sqlite3_exec(db, create, nil, nil, nil)

	insert := "INSERT INTO t (name, score) VALUES (?1, ?2)"
	var ins *sqlite3.sqlite3_stmt
	sqlite3.sqlite3_prepare_v2(db, insert, -1, &ins, nil)
	names := []string{"ann", "bo", "cy"}
	for i, name := range names {
		label := name + "-" + fmt.Sprint(i)
		sqlite3.sqlite3_bind_text(ins, 1, label, -1, sqlite3.SQLITE_TRANSIENT)
		sqlite3.sqlite3_bind_double(ins, 2, float64(i)+0.5)
		sqlite3.sqlite3_step(ins)
		sqlite3.sqlite3_reset(ins)
	}
	sqlite3.sqlite3_finalize(ins)
	var rows int = sqlite3.sqlite3_last_insert_rowid(db)
	var changes int = sqlite3.sqlite3_changes(db)
	fmt.Println("rows:", rows, "changes:", changes)

	query := "SELECT id, name, score FROM t WHERE score > ?1 ORDER BY id"
	var q *sqlite3.sqlite3_stmt
	sqlite3.sqlite3_prepare_v2(db, query, -1, &q, nil)
	sqlite3.sqlite3_bind_double(q, 1, 0.9)
	for sqlite3.sqlite3_step(q) == sqlite3.SQLITE_ROW {
		var id int = sqlite3.sqlite3_column_int(q, 0)
		var score float64 = sqlite3.sqlite3_column_double(q, 2)
		fmt.Println(id, C.GoString(sqlite3.sqlite3_column_text(q, 1)), score)
	}
	sqlite3.sqlite3_finalize(q)

	bad := "SELEC nonsense"
	rc := sqlite3.sqlite3_exec(db, bad, nil, nil, nil)
	fmt.Println(rc == sqlite3.SQLITE_ERROR, C.GoString(sqlite3.sqlite3_errmsg(db)))
}

// Output:
// rows: 3 changes: 1
// 2 bo-1 1.5
// 3 cy-2 2.5
// true near "SELEC": syntax error
