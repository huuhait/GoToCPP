//gocpp:pkg-config sqlite3

// sqlite3 with //gocpp:pkg-config (probed with the flags that pkg-config
// gives): a C pointer filled by an out
// parameter, error codes, callbacks through C strings.
package main

import (
	"errors"
	"fmt"

	"<sqlite3.h>"
)

type DB struct {
	conn *sqlite3.sqlite3
	path string
}

func open(path string) (*DB, error) {
	var conn *sqlite3.sqlite3
	if rc := sqlite3.sqlite3_open(C.cstr(path), &conn); rc != sqlite3.SQLITE_OK {
		return nil, errors.New(C.GoString(sqlite3.sqlite3_errstr(rc)))
	}
	return &DB{conn: conn, path: path}, nil
}

func (db *DB) exec(sql string) error {
	var msg *C.char
	rc := sqlite3.sqlite3_exec(db.conn, C.cstr(sql), nil, nil, &msg)
	if int(rc) != int(sqlite3.SQLITE_OK) {
		defer sqlite3.sqlite3_free(msg)
		return errors.New(C.GoString(msg))
	}
	return nil
}

func (db *DB) count() int {
	var stmt *sqlite3.sqlite3_stmt
	sqlite3.sqlite3_prepare_v2(db.conn, C.cstr("SELECT count(*) FROM t"), -1, &stmt, nil)
	defer sqlite3.sqlite3_finalize(stmt)
	n := 0
	for sqlite3.sqlite3_step(stmt) == sqlite3.SQLITE_ROW {
		n = int(sqlite3.sqlite3_column_int(stmt, 0))
	}
	return n
}

func main() {
	db, err := open(":memory:")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer sqlite3.sqlite3_close(db.conn)
	if err := db.exec("CREATE TABLE t(x INT); INSERT INTO t VALUES (1), (2)"); err != nil {
		fmt.Println(err)
	}
	fmt.Println("sqlite", C.GoString(sqlite3.sqlite3_libversion()), db.count(), db.path)
}
