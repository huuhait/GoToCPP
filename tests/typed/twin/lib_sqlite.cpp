// The C++ twin of lib_sqlite.gp: the expected output is what this program prints.
#include <sqlite3.h>
#include <cstdio>
#include <string>

int main() {
    std::string file = ":memory:";
    sqlite3* db = nullptr;
    if (sqlite3_open(file.c_str(), &db) != SQLITE_OK) return 1;

    std::string create = "CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT, score REAL)";
    sqlite3_exec(db, create.c_str(), nullptr, nullptr, nullptr);

    std::string insert = "INSERT INTO t (name, score) VALUES (?1, ?2)";
    sqlite3_stmt* ins = nullptr;
    sqlite3_prepare_v2(db, insert.c_str(), -1, &ins, nullptr);
    const char* names[] = {"ann", "bo", "cy"};
    for (int i = 0; i < 3; i++) {
        std::string label = std::string(names[i]) + "-" + std::to_string(i);
        sqlite3_bind_text(ins, 1, label.c_str(), -1, SQLITE_TRANSIENT);
        sqlite3_bind_double(ins, 2, double(i) + 0.5);
        sqlite3_step(ins);
        sqlite3_reset(ins);
    }
    sqlite3_finalize(ins);
    long rows = sqlite3_last_insert_rowid(db);
    long changes = sqlite3_changes(db);
    std::printf("rows: %ld changes: %ld\n", rows, changes);

    std::string query = "SELECT id, name, score FROM t WHERE score > ?1 ORDER BY id";
    sqlite3_stmt* q = nullptr;
    sqlite3_prepare_v2(db, query.c_str(), -1, &q, nullptr);
    sqlite3_bind_double(q, 1, 0.9);
    while (sqlite3_step(q) == SQLITE_ROW) {
        long id = sqlite3_column_int(q, 0);
        double score = sqlite3_column_double(q, 2);
        std::printf("%ld %s %g\n", id, (const char*)sqlite3_column_text(q, 1), score);
    }
    sqlite3_finalize(q);

    std::string bad = "SELEC nonsense";
    int rc = sqlite3_exec(db, bad.c_str(), nullptr, nullptr, nullptr);
    std::printf("%s %s\n", rc == SQLITE_ERROR ? "true" : "false", sqlite3_errmsg(db));
    sqlite3_close(db);
}
