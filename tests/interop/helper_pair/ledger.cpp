// ledger.cpp: compiled with the package by "gocpp run ." and linked with the generated code.
#include "ledger.hpp"

#include <cstdio>

namespace ledger {

static int dollars_calls = 0;

bool book::add(std::string_view name, int cents) {
    if (size() >= LEDGER_MAX_ENTRIES) {
        return false;
    }
    entries_.push_back({std::string(name), cents});
    return true;
}

int book::total() const {
    int sum = 0;
    for (const entry &e : entries_) {
        sum += e.cents;
    }
    return sum;
}

std::string book::report() const {
    std::string out;
    for (const entry &e : entries_) {
        out += e.name + "=" + dollars(e.cents) + ";";
    }
    return out;
}

std::string dollars(int cents) {
    dollars_calls++;
    char buf[32];
    std::snprintf(buf, sizeof buf, "%d.%02d", cents / 100, cents % 100);
    return buf;
}

int calls() { return dollars_calls; }

} // namespace ledger

int ledger_total(const ledger::book &b) { return b.total(); }
