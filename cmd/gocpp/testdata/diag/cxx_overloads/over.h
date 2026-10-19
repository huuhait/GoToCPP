#pragma once

inline int pick(int a) { return a; }
inline int pick(long a, long b) { return (int)(a + b); }
inline int pick(double a, int b, int c) { return (int)a + b + c; }
inline int pick(char a, char b, char c, char d) { return a + b + c + d; }
inline int pick(float a, float b, float c, float d, float e) { return (int)(a + b + c + d + e); }
