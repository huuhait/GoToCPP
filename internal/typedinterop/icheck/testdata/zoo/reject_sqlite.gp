// Wrong calls of sqlite3 (the dev shell has it).
//
//gocpp:pkg-config sqlite3
package main

import "<sqlite3.h>"

func main() {
	var db *sqlite3.sqlite3
	_ = db
	_ = sqlite3.sqlite3_open(5, &db)
	_ = sqlite3.sqlite3_close()
	_ = sqlite3.sqlite3_nosuch(1)
	_ = db.x
}
