// textutil.cpp: compiled with the C++ compiler ($CXX -std=c++20) and linked in.
#include "textutil.hpp"

#include <cctype>

namespace textutil {

std::string title(std::string_view s) {
    std::string out(s);
    bool start = true;
    for (char& c : out) {
        if (c == ' ') {
            start = true;
        } else if (start) {
            c = static_cast<char>(std::toupper(static_cast<unsigned char>(c)));
            start = false;
        }
    }
    return out;
}

int word_count(std::string_view s) {
    int n = 0;
    bool in_word = false;
    for (char c : s) {
        if (c == ' ') {
            in_word = false;
        } else if (!in_word) {
            in_word = true;
            n++;
        }
    }
    return n;
}

void tally::add(std::string_view word) {
    auto it = counts_.find(word);
    if (it == counts_.end()) {
        counts_.emplace(std::string(word), 1);
    } else {
        it->second++;
    }
}

int tally::count(std::string_view word) const {
    auto it = counts_.find(word);
    return it == counts_.end() ? 0 : it->second;
}

std::string tally::top() const {
    std::string best;
    int most = 0;
    for (const auto& [word, n] : counts_) {
        if (n > most) {
            best = word;
            most = n;
        }
    }
    return best;
}

} // namespace textutil
