// phase: 07

package main

import "fmt"

func Cnt(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

func classify(n int) string {
	out := ""
	for x := range Cnt(n) {
		switch {
		case x == 0:
			out += "zero,"
			fallthrough
		case x == 1:
			out += "small,"
		case x == 3:
			out += "three,"
			continue
		case x > 5:
			out += "big,"
			return out + "stop"
		default:
			out += "other,"
			if x == 4 {
				break
			}
			out += "(not four),"
		}
		out += ";"
	}
	return out
}

func typeSwitch(vals []any) (res string) {
	seq := func(yield func(any) bool) {
		for _, v := range vals {
			if !yield(v) {
				return
			}
		}
	}
	for v := range seq {
		switch x := v.(type) {
		case int:
			res += fmt.Sprint("int", x, ",")
		case string:
			if x == "stop" {
				return
			}
			res += "string " + x + ","
		default:
			continue
		}
	}
	return res + "end"
}

func main() {
	fmt.Println(classify(3))
	fmt.Println(classify(5))
	fmt.Println(classify(9))
	fmt.Println(typeSwitch([]any{1, "a", 2.5, "stop", 3}))
	fmt.Println(typeSwitch([]any{1, nil}))
}

// Output:
// zero,small,;small,;other,(not four),;
// zero,small,;small,;other,(not four),;three,other,;
// zero,small,;small,;other,(not four),;three,other,;other,(not four),;big,stop
// int1,string a,
// int1,end
