// phase: 04
// pkg-config: sqlite3

// Sqlite keeps a small table in an in-memory database through the sqlite3 C
// library. It uses the library's header package: raw C pointers
// (*sqlite3.sqlite3), its macros (SQLITE_OK), C strings (C.CString, C.cstr,
// C.GoString) and gocpp functions that C calls back.
//
//gocpp:pkg-config sqlite3
package main

import (
	"fmt"
	"unsafe"

	"<sqlite3.h>"
)

// printRow is called by sqlite3_exec for every row of a result: C calls a
// gocpp function through a plain C function pointer.
func printRow(_ unsafe.Pointer, n C.int, values **C.char, names **C.char) C.int {
	count := int(n)
	cols := unsafe.Slice(names, count)
	vals := unsafe.Slice(values, count)
	line := ""
	for i := range count {
		if i > 0 {
			line += " "
		}
		line += C.GoString(cols[i]) + "=" + C.GoString(vals[i])
	}
	fmt.Println(line)
	return 0
}

// countRow counts rows. sqlite3_exec passes its void* argument through:
// here a pointer to a gocpp int.
func countRow(arg unsafe.Pointer, _ C.int, _ **C.char, _ **C.char) C.int {
	*(*int)(arg)++
	return 0
}

// exec runs SQL that returns no rows. C.CString makes a C copy of the gocpp
// string with malloc, and C.free releases it.
func exec(db *sqlite3.sqlite3, sql string) bool {
	csql := C.CString(sql)
	defer C.free(unsafe.Pointer(csql))
	var errmsg *C.char
	if sqlite3.sqlite3_exec(db, csql, nil, nil, &errmsg) != sqlite3.SQLITE_OK {
		fmt.Println("error:", C.GoString(errmsg))
		sqlite3.sqlite3_free(unsafe.Pointer(errmsg))
		return false
	}
	return true
}

func main() {
	var db *sqlite3.sqlite3
	if rc := sqlite3.sqlite3_open(":memory:", &db); rc != sqlite3.SQLITE_OK {
		fmt.Println("open failed:", C.GoString(sqlite3.sqlite3_errstr(rc)))
		return
	}
	defer sqlite3.sqlite3_close(db)
	exec(db, "CREATE TABLE fruit (name TEXT, qty INTEGER)")

	// A prepared statement, with gocpp values bound to its parameters.
	// C.cstr gives a C string that lives until the end of the statement.
	var stmt *sqlite3.sqlite3_stmt
	sqlite3.sqlite3_prepare_v2(db, "INSERT INTO fruit VALUES (?, ?)", -1, &stmt, nil)
	for i, name := range []string{"apple", "banana", "cherry", "date"} {
		sqlite3.sqlite3_bind_text(stmt, 1, C.cstr(name), -1, sqlite3.SQLITE_TRANSIENT)
		sqlite3.sqlite3_bind_int(stmt, 2, C.int((i+1)*3))
		if sqlite3.sqlite3_step(stmt) != sqlite3.SQLITE_DONE {
			fmt.Println("insert failed:", C.GoString(sqlite3.sqlite3_errmsg(db)))
		}
		sqlite3.sqlite3_reset(stmt)
	}
	sqlite3.sqlite3_finalize(stmt)

	exec(db, "UPDATE fruit SET qty = qty * 10 WHERE name < 'c'")
	fmt.Println("updated:", int(sqlite3.sqlite3_changes(db)))

	// sqlite3_exec calls printRow for every row, and countRow with &count.
	sqlite3.sqlite3_exec(db, "SELECT name, qty FROM fruit ORDER BY qty DESC", printRow, nil, nil)
	count := 0
	sqlite3.sqlite3_exec(db, "SELECT * FROM fruit WHERE qty > 10", countRow, unsafe.Pointer(&count), nil)
	fmt.Println("rows with qty > 10:", count)

	// Read the columns of a result through a prepared statement.
	var q *sqlite3.sqlite3_stmt
	sqlite3.sqlite3_prepare_v2(db, "SELECT sum(qty), max(name) FROM fruit", -1, &q, nil)
	if sqlite3.sqlite3_step(q) == sqlite3.SQLITE_ROW {
		total := int(sqlite3.sqlite3_column_int64(q, 0))
		last := C.GoString(sqlite3.sqlite3_column_text(q, 1))
		fmt.Println("total:", total, "last:", last)
	}
	sqlite3.sqlite3_finalize(q)

	// An error comes back as a C string from sqlite.
	ok := exec(db, "INSERT INTO vegetables VALUES ('kale', 1)")
	fmt.Println("insert into vegetables:", ok)
}
