// textutil.hpp: C++ helpers, defined in textutil.cpp. Its header package is
// textutil, and so is its namespace: textutil.title is looked up in the
// namespace first (Phase 04 §2.3). A .hpp header is never put inside
// extern "C".
#pragma once

#include <map>
#include <string>
#include <string_view>

namespace textutil {

// title turns the first letter of every word into upper case.
std::string title(std::string_view s);

// word_count counts the words, which are separated by spaces.
int word_count(std::string_view s);

// tally counts words.
class tally {
public:
    void add(std::string_view word);
    int count(std::string_view word) const;
    int size() const { return static_cast<int>(counts_.size()); }
    // top returns the word seen most often; the first in order on a tie.
    std::string top() const;

private:
    std::map<std::string, int, std::less<>> counts_;
};

} // namespace textutil
