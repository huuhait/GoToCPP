/* the names of the C library that the view writes as Go types, declared here by the header itself */
#pragma once
typedef unsigned char uint8_t;
typedef int int32_t;
typedef unsigned long size_t;
typedef __builtin_va_list va_list;
uint8_t low_byte(int32_t value);
size_t measure(const uint8_t *data, size_t n);
int vmeasure(const char *fmt, va_list ap);
