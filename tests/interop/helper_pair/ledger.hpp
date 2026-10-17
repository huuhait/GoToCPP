// ledger.hpp: a helper header with a .cpp next to it (step 4.16). The header package is ledger,
// and so is its namespace, which is looked up first (README §2.3); ledger_total is a C-level name.
// A .hpp header is never put inside extern "C".
#pragma once

#include <string>
#include <string_view>
#include <vector>

#include "inc/limits.h"

namespace ledger {

class book {
public:
    // add records an entry; false when the book is full.
    bool add(std::string_view name, int cents);
    int total() const;
    int size() const { return static_cast<int>(entries_.size()); }
    std::string report() const;

private:
    struct entry {
        std::string name;
        int cents;
    };
    std::vector<entry> entries_;
};

// dollars formats cents as "12.34".
std::string dollars(int cents);

// calls counts the calls of dollars (state that lives in ledger.cpp).
int calls();

} // namespace ledger

// A function of the C++ file that is not in the namespace.
int ledger_total(const ledger::book& b);
