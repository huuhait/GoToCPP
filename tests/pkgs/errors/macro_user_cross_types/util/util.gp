// phase: 10

package util

macro Plain(n int, msg lazy string) int { return n + len(msg()) }

func Real(n int) int { return n }
