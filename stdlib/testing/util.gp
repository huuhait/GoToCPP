// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testing

import "fmt"

// Small string helpers: package strings is a declaration stub until step 9.4 ports it, and the
// testing package must not wait for it. They work on bytes, like the standard functions do.

func indexOf(s, sub string) int {
	n := len(sub)
	for i := 0; i+n <= len(s); i++ {
		if s[i:i+n] == sub {
			return i
		}
	}
	return -1
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func countByte(s string, c byte) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			n++
		}
	}
	return n
}

// splitByte splits s around each c, like strings.Split(s, string(c)).
func splitByte(s string, c byte) []string {
	var parts []string
	for {
		i := indexByte(s, c)
		if i < 0 {
			break
		}
		parts = append(parts, s[:i])
		s = s[i+1:]
	}
	return append(parts, s)
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
}

func trimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && isSpaceByte(s[i]) {
		i++
	}
	for j > i && isSpaceByte(s[j-1]) {
		j--
	}
	return s[i:j]
}

// atoi parses a decimal integer with an optional sign; ok is false for anything else.
func atoi(s string) (n int, ok bool) {
	if s == "" {
		return 0, false
	}
	neg := false
	if s[0] == '-' || s[0] == '+' {
		neg = s[0] == '-'
		s = s[1:]
		if s == "" {
			return 0, false
		}
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		n = n*10 + int(s[i]-'0')
		if n < 0 {
			return 0, false
		}
	}
	if neg {
		n = -n
	}
	return n, true
}

// parseBool accepts what strconv.ParseBool does.
func parseBool(s string) (v, ok bool) {
	switch s {
	case "1", "t", "T", "true", "TRUE", "True":
		return true, true
	case "0", "f", "F", "false", "FALSE", "False":
		return false, true
	}
	return false, false
}

// parseDuration parses a duration like time.ParseDuration: a sequence of decimal numbers with a
// unit suffix (ns, us, µs, μs, ms, s, m, h), such as "1m30s" or "1.5h". It returns nanoseconds.
func parseDuration(s string) (int64, bool) {
	orig := s
	_ = orig
	neg := false
	if s != "" && (s[0] == '-' || s[0] == '+') {
		neg = s[0] == '-'
		s = s[1:]
	}
	if s == "0" {
		return 0, true
	}
	if s == "" {
		return 0, false
	}
	var total float64
	for s != "" {
		// the number
		i := 0
		var whole, frac float64
		scale := 1.0
		seenDigit := false
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			whole = whole*10 + float64(s[i]-'0')
			i++
			seenDigit = true
		}
		if i < len(s) && s[i] == '.' {
			i++
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				frac = frac*10 + float64(s[i]-'0')
				scale *= 10
				i++
				seenDigit = true
			}
		}
		if !seenDigit {
			return 0, false
		}
		s = s[i:]
		// the unit
		j := 0
		for j < len(s) && s[j] != '.' && (s[j] < '0' || s[j] > '9') {
			j++
		}
		if j == 0 {
			return 0, false
		}
		unit := s[:j]
		s = s[j:]
		var mult float64
		switch unit {
		case "ns":
			mult = 1
		case "us", "µs", "μs":
			mult = 1e3
		case "ms":
			mult = 1e6
		case "s":
			mult = 1e9
		case "m":
			mult = 60e9
		case "h":
			mult = 3600e9
		default:
			return 0, false
		}
		total += (whole + frac/scale) * mult
	}
	d := int64(total)
	if neg {
		d = -d
	}
	return d, true
}

// durationString formats nanoseconds like time.Duration.String: "1.5s", "2m0s", "10m0s", "0s".
func durationString(d int64) string {
	if d == 0 {
		return "0s"
	}
	neg := d < 0
	u := uint64(d)
	if neg {
		u = -u
	}
	var out string
	if u < uint64(1000000000) {
		// Less than a second: ns, µs or ms with a fraction.
		switch {
		case u < 1000:
			out = fmt.Sprintf("%dns", u)
		case u < 1000000:
			out = fracString(u, 1000) + "µs"
		default:
			out = fracString(u, 1000000) + "ms"
		}
	} else {
		secNanos := u % 60000000000
		mins := u / 60000000000
		out = fracString(secNanos, 1000000000) + "s"
		if mins > 0 {
			hours := mins / 60
			mins = mins % 60
			out = fmt.Sprintf("%dm", mins) + out
			if hours > 0 {
				out = fmt.Sprintf("%dh", hours) + out
			}
		}
	}
	if neg {
		out = "-" + out
	}
	return out
}

// fracString formats v/unit with its fraction digits, without trailing zeros.
func fracString(v, unit uint64) string {
	whole := v / unit
	rest := v % unit
	if rest == 0 {
		return fmt.Sprintf("%d", whole)
	}
	digits := fmt.Sprintf("%d", unit)
	width := len(digits) - 1
	f := fmt.Sprintf("%0*d", width, rest)
	for len(f) > 0 && f[len(f)-1] == '0' {
		f = f[:len(f)-1]
	}
	return fmt.Sprintf("%d.%s", whole, f)
}

// fmtDuration returns a string representing d in the form "87.00s".
func fmtDuration(d int64) string {
	return fmt.Sprintf("%.2fs", float64(d)/1e9)
}

// sortStrings sorts s in increasing order (package sort is not a dependency of testing).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
