// phase: 03

// A stack as a slice: balanced brackets, and an expression evaluator (shunting-yard to RPN,
// then the RPN evaluated), with byte-by-byte parsing of the input strings.
package main

import "fmt"

func balanced(s string) bool {
	pairs := map[byte]byte{')': '(', ']': '[', '}': '{'}
	var st []byte
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '(', '[', '{':
			st = append(st, c)
		case ')', ']', '}':
			if len(st) == 0 || st[len(st)-1] != pairs[c] {
				return false
			}
			st = st[:len(st)-1]
		}
	}
	return len(st) == 0
}

var prec = map[byte]int{'+': 1, '-': 1, '*': 2, '/': 2}

func rpn(expr string) []string {
	var out []string
	var ops []byte
	for i := 0; i < len(expr); i++ {
		c := expr[i]
		switch {
		case c >= '0' && c <= '9':
			j := i
			for j < len(expr) && expr[j] >= '0' && expr[j] <= '9' {
				j++
			}
			out = append(out, expr[i:j])
			i = j - 1
		case c == '(':
			ops = append(ops, c)
		case c == ')':
			for ops[len(ops)-1] != '(' {
				out = append(out, string(ops[len(ops)-1]))
				ops = ops[:len(ops)-1]
			}
			ops = ops[:len(ops)-1]
		case prec[c] > 0:
			for len(ops) > 0 && prec[ops[len(ops)-1]] >= prec[c] {
				out = append(out, string(ops[len(ops)-1]))
				ops = ops[:len(ops)-1]
			}
			ops = append(ops, c)
		}
	}
	for len(ops) > 0 {
		out = append(out, string(ops[len(ops)-1]))
		ops = ops[:len(ops)-1]
	}
	return out
}

func eval(toks []string) int {
	var st []int
	for _, t := range toks {
		if p := prec[t[0]]; p > 0 && len(t) == 1 {
			b, a := st[len(st)-1], st[len(st)-2]
			st = st[:len(st)-2]
			var r int
			switch t {
			case "+":
				r = a + b
			case "-":
				r = a - b
			case "*":
				r = a * b
			case "/":
				r = a / b
			}
			st = append(st, r)
			continue
		}
		n := 0
		for i := 0; i < len(t); i++ {
			n = n*10 + int(t[i]-'0')
		}
		st = append(st, n)
	}
	return st[0]
}

func main() {
	var bs []bool
	for _, s := range []string{"([]{})", "([)]", "((", "", "{[()()]}"} {
		bs = append(bs, balanced(s))
	}
	fmt.Println(bs)
	for _, e := range []string{"1+2*3", "(1+2)*3", "100/(4-2*1)-7", "2*(3+4)*(5-1)"} {
		r := rpn(e)
		fmt.Println(e, r, eval(r))
	}
}

// Output:
// [true false false true true]
// 1+2*3 [1 2 3 * +] 7
// (1+2)*3 [1 2 + 3 *] 9
// 100/(4-2*1)-7 [100 4 2 1 * - / 7 -] 43
// 2*(3+4)*(5-1) [2 3 4 + * 5 1 - *] 56
