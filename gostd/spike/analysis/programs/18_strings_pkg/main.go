// strings (Go's own package): searching, case mapping, trimming, replacement, splitting,
// joining, Builder, Replacer, a range over strings.Lines (iter.Seq, range-over-func), Map with a
// rune func value, and Clone.
package main

import "strings"

func main() {
	println(strings.Contains("gocpp", "cpp"))
	println(strings.Index("banana", "na"), strings.LastIndex("banana", "na"))
	println(strings.Count("banana", "a"))
	println(strings.ToUpper("go"), strings.ToLower("CPP"))
	println(strings.Repeat("ab", 3))
	println(strings.TrimSpace("  x  "), strings.TrimPrefix("prefix-x", "prefix-"), strings.TrimSuffix("x-suffix", "-suffix"))
	println(strings.Replace("aaa", "a", "b", 2), strings.ReplaceAll("aaa", "a", "b"))
	println(strings.Join([]string{"a", "b", "c"}, "-"))
	parts := strings.Split("a,b,c", ",")
	println(len(parts), parts[0], parts[2])
	fields := strings.Fields("  a  b  ")
	println(len(fields), fields[1])
	println(strings.HasPrefix("gocpp", "go"), strings.HasSuffix("gocpp", "cpp"))
	println(strings.EqualFold("Go", "GO"))
	println(strings.Compare("a", "b"), strings.Compare("b", "a"), strings.Compare("a", "a"))
	var b strings.Builder
	b.WriteString("go")
	b.WriteByte('!')
	b.WriteRune('é')
	println(b.String(), b.Len())
	r := strings.NewReplacer("a", "1", "b", "2")
	println(r.Replace("abba"))
	r2 := strings.NewReplacer("xy", "Z")
	println(r2.Replace("axby"))
	n := 0
	for line := range strings.Lines("one\ntwo\n") {
		n++
		println("line", line)
	}
	println(n)
	println(strings.Map(func(r rune) rune {
		if r == 'a' {
			return 'A'
		}
		return r
	}, "banana"))
	println(strings.Clone("abc"))
}
