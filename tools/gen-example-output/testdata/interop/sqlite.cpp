// C++ equivalent of examples/sqlite/main.gp, to check its output.txt.
#include <sqlite3.h>
#include <cstdint>
#include <cstdlib>
#include <cstring>
#include <iostream>
#include <string>
static std::string GoString(const void* p) {
    return p ? std::string(static_cast<const char*>(p)) : std::string();
}
static int printRow(void*, int n, char** values, char** names) {
    std::string line;
    for (int i = 0; i < n; i++) {
        if (i > 0)
            line += " ";
        line += GoString(names[i]) + "=" + GoString(values[i]);
    }
    std::cout << line << "\n";
    return 0;
}
static int countRow(void* arg, int, char**, char**) {
    (*static_cast<int64_t*>(arg))++;
    return 0;
}
static bool exec(sqlite3* db, const std::string& sql) {
    char* csql = strdup(sql.c_str());
    char* errmsg = nullptr;
    bool ok = true;
    if (sqlite3_exec(db, csql, nullptr, nullptr, &errmsg) != SQLITE_OK) {
        std::cout << "error: " << GoString(errmsg) << "\n";
        sqlite3_free(errmsg);
        ok = false;
    }
    free(csql);
    return ok;
}
int main() {
    std::cout << std::boolalpha;
    sqlite3* db = nullptr;
    if (int rc = sqlite3_open(":memory:", &db); rc != SQLITE_OK) {
        std::cout << "open failed\n";
        return 0;
    }
    exec(db, "CREATE TABLE fruit (name TEXT, qty INTEGER)");
    sqlite3_stmt* stmt = nullptr;
    sqlite3_prepare_v2(db, "INSERT INTO fruit VALUES (?, ?)", -1, &stmt, nullptr);
    const char* names[] = {"apple", "banana", "cherry", "date"};
    for (int i = 0; i < 4; i++) {
        sqlite3_bind_text(stmt, 1, names[i], -1, SQLITE_TRANSIENT);
        sqlite3_bind_int(stmt, 2, (i + 1) * 3);
        if (sqlite3_step(stmt) != SQLITE_DONE)
            std::cout << "insert failed: " << sqlite3_errmsg(db) << "\n";
        sqlite3_reset(stmt);
    }
    sqlite3_finalize(stmt);
    exec(db, "UPDATE fruit SET qty = qty * 10 WHERE name < 'c'");
    std::cout << "updated: " << sqlite3_changes(db) << "\n";
    sqlite3_exec(db, "SELECT name, qty FROM fruit ORDER BY qty DESC", printRow, nullptr, nullptr);
    int64_t count = 0;
    sqlite3_exec(db, "SELECT * FROM fruit WHERE qty > 10", countRow, &count, nullptr);
    std::cout << "rows with qty > 10: " << count << "\n";
    sqlite3_stmt* q = nullptr;
    sqlite3_prepare_v2(db, "SELECT sum(qty), max(name) FROM fruit", -1, &q, nullptr);
    if (sqlite3_step(q) == SQLITE_ROW) {
        int64_t total = sqlite3_column_int64(q, 0);
        std::string last = GoString(sqlite3_column_text(q, 1));
        std::cout << "total: " << total << " last: " << last << "\n";
    }
    sqlite3_finalize(q);
    bool ok = exec(db, "INSERT INTO vegetables VALUES ('kale', 1)");
    std::cout << "insert into vegetables: " << ok << "\n";
    sqlite3_close(db);
}
