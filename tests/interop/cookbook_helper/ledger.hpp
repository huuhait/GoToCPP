#pragma once

#include <string>

namespace ledger {

// dollars formats an amount in cents: 1050 -> "$10.50".
std::string dollars(int cents);

// Account is a C++ class that Go code can use like any other C++ type.
class Account {
public:
    explicit Account(int cents = 0) : cents_(cents) {}
    void deposit(int cents) { cents_ += cents; }
    int cents() const { return cents_; }
    std::string text() const { return dollars(cents_); }

private:
    int cents_;
};

}  // namespace ledger
