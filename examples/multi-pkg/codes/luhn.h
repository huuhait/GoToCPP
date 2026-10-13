/* luhn.h: the Luhn check digit (used for card and IMEI numbers), in C. */
#ifndef LUHN_H
#define LUHN_H

/* luhn_check_digit returns the digit (0 to 9) to append to digits so that
 * the number is valid, or -1 when digits is empty or has a character that is
 * not a digit. */
int luhn_check_digit(const char* digits);

/* luhn_valid returns 1 when the last digit of number is its check digit,
 * else 0. */
int luhn_valid(const char* number);

#endif
