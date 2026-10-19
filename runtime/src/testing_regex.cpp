// The regular expressions of -test.run, -test.skip, -test.list and -test.bench (Phase 09 step 9.9),
// until package regexp is ported (step 9.4).
//
// Go's regexp package (RE2) is not there yet, and std::regex has the ECMAScript syntax. This file
// translates the pattern from RE2's syntax to ECMAScript's, for what test names need, and reports
// the errors that Go reports with Go's words:
//
//   - the syntax of RE2 that ECMAScript has not, and the other way round: named groups
//     (?P<name>...) and (?<name>...) are plain groups, \A and \z are ^ and $, \Q...\E quotes,
//     \x{10FFFF} is a code point, (?i), (?s) and (?m) at the start of the pattern are flags;
//   - what ECMAScript has and RE2 has not is an error, as in Go: lookahead (?=...), backreferences
//     \1, possessive and nested repetition a++, a**, a{2}{3};
//   - the dot and a negated class match a whole UTF-8 character, as in Go (std::regex<char> would
//     match one byte).
//
// What it cannot do is an error that says so: \pL and \p{Greek}, the flags in the middle of a
// pattern
// ((?i:...), a?(?i)b), (?U). A class that has a non-ASCII character in it is matched byte by byte:
// [é] matches the bytes of é, and [^é] does not match the first byte of any character that has the
// same first byte. docs/guides/testing.md lists these differences.
#include <gocpp/testing.hpp>

#include <cstdint>
#include <string>
#include <vector>

namespace gp::testing::detail {

namespace {

// One UTF-8 character, as bytes: ASCII (all, or without the newline), or a lead byte and its
// continuation bytes. The ranges are in the signed order of char, which keeps [\x80-\xBF] a range.
constexpr const char* utf8_tail = "[\\x80-\\xBF]";
std::string utf8_char(bool newline) {
    std::string t = utf8_tail;
    return std::string("(?:") + (newline ? "[\\x00-\\x7F]" : "[\\x00-\\x09\\x0B-\\x7F]") +
           "|[\\xC2-\\xDF]" + t + "|[\\xE0-\\xEF]" + t + t + "|[\\xF0-\\xF4]" + t + t + t + ")";
}

bool is_digit(char c) {
    return c >= '0' && c <= '9';
}
bool is_alnum(char c) {
    return is_digit(c) || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z');
}
int hex_value(char c) {
    if (is_digit(c)) {
        return c - '0';
    }
    if (c >= 'a' && c <= 'f') {
        return c - 'a' + 10;
    }
    if (c >= 'A' && c <= 'F') {
        return c - 'A' + 10;
    }
    return -1;
}

// utf8_length is the length of the UTF-8 character that starts at s[i] (1 for a byte that does not
// start one).
std::size_t utf8_length(const std::string& s, std::size_t i) {
    auto b = static_cast<unsigned char>(s[i]);
    std::size_t n = b < 0x80 ? 1 : b >= 0xF0 ? 4 : b >= 0xE0 ? 3 : b >= 0xC0 ? 2 : 1;
    return i + n <= s.size() ? n : 1;
}

void append_utf8(std::string& out, uint32_t r) {
    if (r < 0x80) {
        out += static_cast<char>(r);
    } else if (r < 0x800) {
        out += static_cast<char>(0xC0 | (r >> 6));
        out += static_cast<char>(0x80 | (r & 0x3F));
    } else if (r < 0x10000) {
        out += static_cast<char>(0xE0 | (r >> 12));
        out += static_cast<char>(0x80 | ((r >> 6) & 0x3F));
        out += static_cast<char>(0x80 | (r & 0x3F));
    } else {
        out += static_cast<char>(0xF0 | (r >> 18));
        out += static_cast<char>(0x80 | ((r >> 12) & 0x3F));
        out += static_cast<char>(0x80 | ((r >> 6) & 0x3F));
        out += static_cast<char>(0x80 | (r & 0x3F));
    }
}

// escape_literal returns ECMAScript text that matches the ASCII character c.
std::string escape_literal(char c) {
    switch (c) {
    case '\\':
    case '^':
    case '$':
    case '.':
    case '|':
    case '?':
    case '*':
    case '+':
    case '(':
    case ')':
    case '[':
    case ']':
    case '{':
    case '}':
    case '/':
    case '-':
        return std::string("\\") + c;
    case '\n':
        return "\\n";
    case '\r':
        return "\\r";
    case '\t':
        return "\\t";
    case '\f':
        return "\\f";
    case '\v':
        return "\\v";
    default:
        break;
    }
    if (static_cast<unsigned char>(c) < 0x20 || c == 0x7F) {
        static const char* digits = "0123456789ABCDEF";
        std::string s = "\\x";
        s += digits[(c >> 4) & 15];
        s += digits[c & 15];
        return s;
    }
    return std::string(1, c);
}

// saw_wide is set when a pattern has a character that is not ASCII (translate_regexp).
thread_local bool saw_wide = false;

// literal_rune returns ECMAScript text that matches the character r: an ASCII character is escaped
// where it needs to be, and the bytes of any other are a group (a repetition applies to all of
// them).
std::string literal_rune(uint32_t r) {
    if (r >= 0x80) {
        saw_wide = true;
    }
    if (r < 0x80) {
        return escape_literal(static_cast<char>(r));
    }
    std::string bytes;
    append_utf8(bytes, r);
    std::string out = "(?:";
    for (char b : bytes) {
        static const char* digits = "0123456789ABCDEF";
        out += "\\x";
        out += digits[(static_cast<unsigned char>(b) >> 4) & 15];
        out += digits[static_cast<unsigned char>(b) & 15];
    }
    return out + ")";
}

std::string re_error(const std::string& what, const std::string& text) {
    return "error parsing regexp: " + what + ": `" + text + "`";
}

enum class last_kind { none, atom, repeat, lazy };

struct translator {
    const std::string& pat;
    std::string out;
    std::string err;
    bool icase = false;
    bool multiline = false;
    bool dotall = false;

    explicit translator(const std::string& p) : pat(p) {}

    bool fail(const std::string& what, const std::string& text) {
        err = re_error(what, text);
        return false;
    }
    bool unsupported(const std::string& text) {
        err =
            "error parsing regexp: not supported by gocpp test until package regexp is ported: `" +
            text + "`";
        return false;
    }

    // assertion appends the zero-width assertion text (^, $, \\b, ...) that ends at pat[i]; Go lets
    // a repetition operator follow it, ECMAScript does not, so a group is made then.
    void assertion(std::string& to, const std::string& text, std::size_t i) {
        bool rep = i < pat.size() && (pat[i] == '*' || pat[i] == '+' || pat[i] == '?');
        if (!rep && i < pat.size() && pat[i] == '{') {
            bool bad = false;
            rep = repeat_at(i, bad) != 0;
        }
        to += rep ? "(?:" + text + ")" : text;
    }

    // The escape at pat[i] (a backslash), outside or inside a class. It appends the ECMAScript text
    // to `to` and moves i after the escape. In a class, the escapes that stand for a set are
    // passed on, and a character is not wrapped in a group.
    bool escape(std::size_t& i, std::string& to, bool in_class,
                std::vector<std::string>* multi = nullptr) {
        if (i + 1 >= pat.size()) {
            return fail("trailing backslash at end of expression", "");
        }
        char c = pat[i + 1];
        std::size_t start = i;
        i += 2;
        auto put = [&](uint32_t r) {
            if (in_class && r >= 0x80) {
                multi->push_back(literal_rune(
                    r)); // a class with such a character is an alternation (char_class)
            } else {
                to += literal_rune(r);
            }
        };
        switch (c) {
        case '1':
        case '2':
        case '3':
        case '4':
        case '5':
        case '6':
        case '7': {
            // Go: a single non-zero digit is a backreference (not supported); \12 and \123 are
            // octal.
            if (i >= pat.size() || pat[i] < '0' || pat[i] > '7') {
                return fail("invalid escape sequence", pat.substr(start, 2));
            }
            [[fallthrough]];
        }
        case '0': {
            uint32_t v = static_cast<uint32_t>(c - '0');
            for (int k = 0; k < 2 && i < pat.size() && pat[i] >= '0' && pat[i] <= '7'; k++, i++) {
                v = v * 8 + static_cast<uint32_t>(pat[i] - '0');
            }
            put(v);
            return true;
        }
        case 'x': {
            if (i < pat.size() && pat[i] == '{') {
                std::size_t close = pat.find('}', i);
                if (close == std::string::npos) {
                    return fail("invalid escape sequence", pat.substr(start));
                }
                uint64_t v = 0;
                bool ok = close > i + 1 && close - i - 1 <= 8;
                for (std::size_t k = i + 1; ok && k < close; k++) {
                    int h = hex_value(pat[k]);
                    ok = h >= 0;
                    v = v * 16 + static_cast<uint64_t>(h < 0 ? 0 : h);
                }
                if (!ok || v > 0x10FFFF || (v >= 0xD800 && v < 0xE000)) {
                    return fail("invalid escape sequence", pat.substr(start, close - start));
                }
                i = close + 1;
                put(static_cast<uint32_t>(v));
                return true;
            }
            if (i + 1 < pat.size() && hex_value(pat[i]) >= 0 && hex_value(pat[i + 1]) >= 0) {
                uint32_t v = static_cast<uint32_t>(hex_value(pat[i]) * 16 + hex_value(pat[i + 1]));
                i += 2;
                put(v);
                return true;
            }
            return fail("invalid escape sequence", pat.substr(start, 4));
        }
        case 'a':
            put(7);
            return true;
        case 'f':
            put(12);
            return true;
        case 'n':
            put(10);
            return true;
        case 'r':
            put(13);
            return true;
        case 't':
            put(9);
            return true;
        case 'v':
            put(11);
            return true;
        case 'd':
        case 's':
        case 'w':
            to += '\\';
            to += c;
            return true;
        case 'D':
        case 'S':
        case 'W': {
            // A character that is not a digit, space or word character: a whole UTF-8 character.
            std::string set = std::string("\\") + static_cast<char>(c + 'a' - 'A');
            std::string text = "(?:(?!" + set + ")" + utf8_char(true) + ")";
            if (in_class) {
                multi->push_back(text);
            } else {
                to += text;
            }
            return true;
        }
        case 'p':
        case 'P': {
            std::size_t end = start + 3; // \pL
            if (i < pat.size() && pat[i] == '{') {
                std::size_t close = pat.find('}', i);
                end = close == std::string::npos ? pat.size() : close + 1; // \p{Greek}
            }
            return unsupported(pat.substr(start, end - start));
        }
        case 'b':
        case 'B':
            if (in_class) {
                return fail("invalid escape sequence", pat.substr(start, 2));
            }
            assertion(to, std::string("\\") + c, i);
            return true;
        case 'A':
        case 'z':
            if (in_class) {
                return fail("invalid escape sequence", pat.substr(start, 2));
            }
            assertion(to, c == 'A' ? "^" : "$", i);
            return true;
        case 'Q': {
            // Quoted text up to \E (or the end).
            std::size_t e = pat.find("\\E", i);
            std::string text = pat.substr(i, e == std::string::npos ? std::string::npos : e - i);
            i = e == std::string::npos ? pat.size() : e + 2;
            for (std::size_t k = 0; k < text.size();) {
                std::size_t n = utf8_length(text, k);
                if (n == 1) {
                    to += escape_literal(text[k]);
                } else {
                    std::string lit = literal_rune(decode(text, k, n));
                    if (in_class) {
                        multi->push_back(lit);
                    } else {
                        to += lit;
                    }
                }
                k += n;
            }
            return true;
        }
        case 'E':
            return true; // a stray \E is ignored
        default:
            // Escaped punctuation is the character itself.
            if (static_cast<unsigned char>(c) < 0x80 && !is_alnum(c)) {
                put(static_cast<unsigned char>(c));
                return true;
            }
            return fail("invalid escape sequence", pat.substr(start, 2));
        }
    }

    // The class at pat[i] ('['). It appends the ECMAScript text and moves i after the class. A
    // class is a set of ASCII characters ([...]) and the alternatives for the characters that are
    // not ASCII, each a group of their bytes: [aÜ] is (?:[a]|(?:\xC3\x9C)).
    bool char_class(std::size_t& i) {
        std::size_t start = i;
        i++;
        bool negated = false;
        if (i < pat.size() && pat[i] == '^') {
            negated = true;
            i++;
        }
        std::string items;
        std::vector<std::string> multi;
        bool first = true;
        for (;;) {
            if (i >= pat.size()) {
                return fail("missing closing ]", pat.substr(start));
            }
            char c = pat[i];
            if (c == ']' && !first) {
                i++;
                break;
            }
            first = false;
            std::size_t item_start = i;
            // [:alpha:] and [:^alpha:]
            if (c == '[' && i + 1 < pat.size() && pat[i + 1] == ':') {
                std::size_t close = pat.find(":]", i + 2);
                if (close != std::string::npos) {
                    std::string name = pat.substr(i + 2, close - i - 2);
                    bool neg = !name.empty() && name[0] == '^';
                    std::string base = neg ? name.substr(1) : name;
                    static const char* known[] = {"alnum", "alpha", "ascii", "blank", "cntrl",
                                                  "digit", "graph", "lower", "print", "punct",
                                                  "space", "upper", "word",  "xdigit"};
                    bool ok = false;
                    for (const char* k : known) {
                        ok = ok || base == k;
                    }
                    if (!ok) {
                        return fail("invalid character class range", pat.substr(i, close + 2 - i));
                    }
                    std::string set = base == "word" ? "\\w" : "[[:" + base + ":]]";
                    if (neg) {
                        // ECMAScript has no negated POSIX class: a whole character that the class
                        // has not.
                        multi.push_back("(?:(?!" + set + ")" + utf8_char(true) + ")");
                    } else {
                        items += base == "word" ? "\\w" : "[:" + base + ":]";
                    }
                    i = close + 2;
                    continue;
                }
            }
            // One character (an escape or a literal), and maybe a range from it.
            std::string lo;
            uint32_t lo_value = 0;
            bool lo_single = true; // a character, not a set (\d) or a quoted text (\Q...\E)
            if (c == '\\') {
                std::size_t at = i;
                if (!escape(i, lo, true, &multi)) {
                    return false;
                }
                const char e = pat[at + 1];
                lo_single = !(e == 'd' || e == 'D' || e == 's' || e == 'S' || e == 'w' ||
                              e == 'W' || e == 'Q');
                if (lo_single) {
                    lo_value = char_value(at);
                }
            } else {
                std::size_t n = utf8_length(pat, i);
                lo_value = decode(pat, i, n);
                if (n == 1) {
                    lo = escape_literal(c);
                    if (c == '[' || c == ':') {
                        lo = std::string("\\") + c;
                    }
                } else {
                    multi.push_back(literal_rune(lo_value));
                }
                i += n;
            }
            if (lo_single && i + 1 < pat.size() && pat[i] == '-' && pat[i + 1] != ']') {
                // A range lo-hi.
                i++;
                std::string hi;
                uint32_t hi_value = 0;
                if (pat[i] == '\\') {
                    std::size_t at = i;
                    std::vector<std::string> ignored;
                    if (!escape(i, hi, true, &ignored)) {
                        return false;
                    }
                    const char e = pat[at + 1];
                    if (e == 'd' || e == 'D' || e == 's' || e == 'S' || e == 'w' || e == 'W' ||
                        e == 'Q') {
                        return fail("invalid character class range",
                                    pat.substr(item_start, i - item_start));
                    }
                    hi_value = char_value(at);
                } else {
                    std::size_t n = utf8_length(pat, i);
                    hi = n == 1 ? escape_literal(pat[i]) : "";
                    hi_value = decode(pat, i, n);
                    i += n;
                }
                if (lo_value > hi_value) {
                    return fail("invalid character class range",
                                pat.substr(item_start, i - item_start));
                }
                if (hi_value >= 0x80) {
                    return unsupported(pat.substr(item_start, i - item_start));
                }
                items += lo + "-" + hi;
                continue;
            }
            items += lo;
        }
        std::string set; // the alternatives, joined
        if (!items.empty()) {
            set = "[" + items + "]";
        }
        for (const std::string& m : multi) {
            set += (set.empty() ? "" : "|") + m;
        }
        if (negated) {
            out += "(?:(?!" + set + ")" + utf8_char(true) + ")";
        } else if (multi.empty()) {
            out += set;
        } else {
            out += "(?:" + set + ")";
        }
        return true;
    }

    // The value of the character that the escape at pat[at] stands for (a character, not a set).
    uint32_t char_value(std::size_t at) {
        char e = pat[at + 1];
        switch (e) {
        case 'a':
            return 7;
        case 'f':
            return 12;
        case 'n':
            return 10;
        case 'r':
            return 13;
        case 't':
            return 9;
        case 'v':
            return 11;
        case 'x': {
            std::size_t k = at + 2;
            if (k < pat.size() && pat[k] == '{') {
                uint64_t v = 0;
                for (k++; k < pat.size() && pat[k] != '}'; k++) {
                    v = v * 16 + static_cast<uint64_t>(hex_value(pat[k]));
                }
                return static_cast<uint32_t>(v);
            }
            return static_cast<uint32_t>(hex_value(pat[k]) * 16 + hex_value(pat[k + 1]));
        }
        default:
            if (e >= '0' && e <= '7') {
                uint32_t v = 0;
                for (std::size_t k = at + 1;
                     k < pat.size() && k < at + 4 && pat[k] >= '0' && pat[k] <= '7'; k++) {
                    v = v * 8 + static_cast<uint32_t>(pat[k] - '0');
                }
                return v;
            }
            return static_cast<unsigned char>(e);
        }
    }

    // The code point of the UTF-8 character of n bytes at s[i].
    static uint32_t decode(const std::string& s, std::size_t i, std::size_t n) {
        auto b = [&](std::size_t k) {
            return static_cast<uint32_t>(static_cast<unsigned char>(s[i + k]));
        };
        switch (n) {
        case 2:
            return ((b(0) & 0x1F) << 6) | (b(1) & 0x3F);
        case 3:
            return ((b(0) & 0x0F) << 12) | ((b(1) & 0x3F) << 6) | (b(2) & 0x3F);
        case 4:
            return ((b(0) & 0x07) << 18) | ((b(1) & 0x3F) << 12) | ((b(2) & 0x3F) << 6) |
                   (b(3) & 0x3F);
        default:
            return b(0);
        }
    }

    // A repetition {n}, {n,} or {n,m} at pat[i], or none (Go takes a { that is not one as itself).
    // It returns its length, 0 if it is not one, and sets bad for a count that is too big or out of
    // order.
    std::size_t repeat_at(std::size_t i, bool& bad) {
        std::size_t k = i + 1;
        auto number = [&](int& n) {
            std::size_t s = k;
            n = 0;
            while (k < pat.size() && is_digit(pat[k])) {
                n = n > 100000 ? n : n * 10 + (pat[k] - '0');
                k++;
            }
            return k > s;
        };
        int lo = 0, hi = 0;
        if (!number(lo)) {
            return 0;
        }
        hi = lo;
        if (k < pat.size() && pat[k] == ',') {
            k++;
            if (k < pat.size() && pat[k] == '}') {
                hi = -1;
            } else if (!number(hi)) {
                return 0;
            }
        }
        if (k >= pat.size() || pat[k] != '}') {
            return 0;
        }
        bad = lo > 1000 || hi > 1000 || (hi >= 0 && lo > hi);
        return k + 1 - i;
    }

    bool run() {
        std::size_t i = 0;
        // Flags at the start: (?i) (?m) (?s), alone or together.
        while (pat.compare(i, 2, "(?") == 0) {
            std::size_t k = i + 2;
            bool ok = false;
            while (k < pat.size() && (pat[k] == 'i' || pat[k] == 'm' || pat[k] == 's')) {
                (pat[k] == 'i' ? icase : pat[k] == 'm' ? multiline : dotall) = true;
                k++;
                ok = true;
            }
            if (!ok || k >= pat.size() || pat[k] != ')') {
                icase = multiline = dotall = false;
                break;
            }
            i = k + 1;
        }
        int depth = 0;
        last_kind last = last_kind::none;
        std::size_t rep_start = 0; // where the last repetition operator starts
        // The group starts: after one the next thing is the first of an alternative.
        for (; i < pat.size();) {
            char c = pat[i];
            switch (c) {
            case '(': {
                if (pat.compare(i, 2, "(?") == 0) {
                    if (pat.compare(i, 3, "(?:") == 0) {
                        out += "(?:";
                        i += 3;
                    } else if (pat.compare(i, 4, "(?P<") == 0 ||
                               (pat.compare(i, 3, "(?<") == 0 && i + 3 < pat.size() &&
                                pat[i + 3] != '=' && pat[i + 3] != '!')) {
                        std::size_t name_at = pat[i + 2] == 'P' ? i + 4 : i + 3;
                        std::size_t close = pat.find('>', name_at);
                        if (close == std::string::npos) {
                            return fail("invalid named capture", pat.substr(i));
                        }
                        std::string name = pat.substr(name_at, close - name_at);
                        bool ok = !name.empty();
                        for (char ch : name) {
                            ok = ok && (is_alnum(ch) || ch == '_');
                        }
                        if (!ok) {
                            return fail("invalid named capture", pat.substr(i, close + 1 - i));
                        }
                        out += "(";
                        i = close + 1;
                    } else if (pat.compare(i, 3, "(?<") == 0) {
                        return fail("invalid named capture", pat.substr(i));
                    } else if (pat.compare(i, 3, "(?)") == 0) {
                        i += 3; // no flags: Go accepts it and it does nothing
                        break;
                    } else {
                        // Flags inside the pattern: (?i) in the middle, (?i:...), (?-i), (?U); the
                        // rest is an error of Go's: (?=, (?!, (?P=name) and the like.
                        std::size_t k = i + 2;
                        while (k < pat.size() && (pat[k] == 'i' || pat[k] == 'm' || pat[k] == 's' ||
                                                  pat[k] == 'U' || pat[k] == '-')) {
                            k++;
                        }
                        if (k < pat.size() && (pat[k] == ')' || pat[k] == ':') && k > i + 2) {
                            std::string fl = pat.substr(i + 2, k - i - 2);
                            std::size_t dash = fl.find('-');
                            if (dash != std::string::npos &&
                                (dash + 1 == fl.size() ||
                                 fl.find('-', dash + 1) != std::string::npos)) {
                                // Go: a second - or none of the flags after it.
                                return fail("invalid or unsupported Perl syntax",
                                            pat.substr(i, k + 1 - i));
                            }
                            return unsupported(pat.substr(i, k + 1 - i));
                        }
                        return fail("invalid or unsupported Perl syntax", pat.substr(i, k + 1 - i));
                    }
                } else {
                    out += "(";
                    i++;
                }
                depth++;
                last = last_kind::none;
                break;
            }
            case ')':
                if (depth == 0) {
                    return fail("unexpected )", pat);
                }
                depth--;
                out += ")";
                i++;
                last = last_kind::atom;
                break;
            case '|':
                out += "|";
                i++;
                last = last_kind::none;
                break;
            case '*':
            case '+':
            case '?': {
                if (last == last_kind::none) {
                    return fail("missing argument to repetition operator", std::string(1, c));
                }
                if (last == last_kind::lazy || (last == last_kind::repeat && c != '?')) {
                    return fail("invalid nested repetition operator",
                                pat.substr(rep_start, i + 1 - rep_start));
                }
                if (last == last_kind::repeat) {
                    last = last_kind::lazy; // a*? is the lazy a*
                } else {
                    last = last_kind::repeat;
                    rep_start = i;
                }
                out += c;
                i++;
                break;
            }
            case '{': {
                bool bad = false;
                std::size_t n = repeat_at(i, bad);
                if (n == 0) {
                    out += "\\{";
                    i++;
                    last = last_kind::atom;
                    break;
                }
                if (last == last_kind::none) {
                    return fail("missing argument to repetition operator", pat.substr(i, n));
                }
                if (last == last_kind::repeat || last == last_kind::lazy) {
                    return fail("invalid nested repetition operator",
                                pat.substr(rep_start, i + n - rep_start));
                }
                if (bad) {
                    return fail("invalid repeat count", pat.substr(i, n));
                }
                out += pat.substr(i, n);
                rep_start = i;
                i += n;
                last = last_kind::repeat;
                break;
            }
            case '.':
                out += utf8_char(dotall);
                i++;
                last = last_kind::atom;
                break;
            case '^':
            case '$':
                assertion(out, std::string(1, c), i + 1);
                i++;
                last = last_kind::atom;
                break;
            case '[':
                if (!char_class(i)) {
                    return false;
                }
                last = last_kind::atom;
                break;
            case '\\': {
                char next = i + 1 < pat.size() ? pat[i + 1] : '\0';
                if (!escape(i, out, false)) {
                    return false;
                }
                last = next == 'E' ? last : last_kind::atom;
                break;
            }
            default: {
                std::size_t n = utf8_length(pat, i);
                out += n == 1 ? escape_literal(c) : literal_rune(decode(pat, i, n));
                i += n;
                last = last_kind::atom;
                break;
            }
            }
        }
        if (depth > 0) {
            return fail("missing closing )", pat);
        }
        return true;
    }
};

} // namespace

bool translate_regexp(const std::string& pat, std::string& out, bool& icase, bool& multiline,
                      std::string& err) {
    translator t(pat);
    saw_wide = false;
    if (!t.run()) {
        err = t.err;
        return false;
    }
    if (t.icase && saw_wide) {
        // Go folds the case of every Unicode letter; the matcher here only knows ASCII.
        err = "error parsing regexp: not supported by gocpp test until package regexp is ported: "
              "`(?i)` with a character that is not ASCII: `" +
              pat + "`";
        return false;
    }
    out = std::move(t.out);
    icase = t.icase;
    multiline = t.multiline;
    return true;
}

} // namespace gp::testing::detail
