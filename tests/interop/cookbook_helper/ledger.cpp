#include "ledger.hpp"

#include <cstdio>

std::string ledger::dollars(int cents) {
    char buf[32];
    std::snprintf(buf, sizeof buf, "$%d.%02d", cents / 100, cents % 100);
    return buf;
}
