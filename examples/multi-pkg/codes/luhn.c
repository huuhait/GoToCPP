/* luhn.c: compiled with the C compiler and linked into package codes. */
#include "luhn.h"

#include <string.h>

/* luhn_sum adds up the digits of s[0..n) from the right, doubling every
 * second one, starting with the last digit when double_last is set. It
 * returns -1 for a character that is not a digit. */
static int luhn_sum(const char* s, size_t n, int double_last) {
    int sum = 0;
    int twice = double_last;
    for (size_t i = n; i > 0; i--) {
        char c = s[i - 1];
        if (c < '0' || c > '9') {
            return -1;
        }
        int d = c - '0';
        if (twice) {
            d *= 2;
            if (d > 9) {
                d -= 9;
            }
        }
        sum += d;
        twice = !twice;
    }
    return sum;
}

int luhn_check_digit(const char* digits) {
    size_t n = strlen(digits);
    int sum = luhn_sum(digits, n, 1);
    if (n == 0 || sum < 0) {
        return -1;
    }
    return (10 - sum % 10) % 10;
}

int luhn_valid(const char* number) {
    size_t n = strlen(number);
    if (n < 2) {
        return 0;
    }
    int sum = luhn_sum(number, n, 0);
    return sum >= 0 && sum % 10 == 0;
}
