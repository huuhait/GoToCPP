// phase: 05

// Runes, bytes and strings in one place: %c %q %U %x of runes, string(rune) values, Print of
// runes (numbers), range over strings, Sprintf with unicode padding, invalid UTF-8 in %q and %x.
package main

import "fmt"

func main() {
	r := 'é'
	b := byte('a')
	fmt.Println(r, b, string(r), string(rune(b)))
	fmt.Printf("%c %q %U %#U %x %d %v\n", r, r, r, r, r, r, r)
	fmt.Printf("%c%c%c %q\n", 0x65e5, 0x672c, 0x8a9e, 0x65e5)
	fmt.Printf("%5c|%-5c|%05d|%+q\n", r, r, r, r)
	fmt.Printf("%q %+q %#q %x % x %X\n", "héllo", "héllo", "héllo", "héllo", "héllo", "héllo")
	fmt.Printf("%q %x %q\n", "a\xffb", "a\xffb", "\xff")
	fmt.Printf("%s|%10s|%-10s|%.2s|%10.2s|\n", "日本語", "日本語", "日本語", "日本語", "日本語")
	for i, c := range "aé日😀" {
		fmt.Printf("%d:%c(%U,%d);", i, c, c, c)
	}
	fmt.Println()
	fmt.Println([]rune("日本"), []byte("日本"), string([]rune{26085, 26412}))
	fmt.Printf("%v %s %q %x\n", []rune("ab"), []rune("ab"), []rune("ab"), []rune("ab"))
	fmt.Printf("%c %c %c\n", []rune("abc"), []byte("abc"), [2]int{65, 66})
	fmt.Printf("%q %q\n", 'x', []int32{'x', 'y'})
	fmt.Printf("%08.3f|%+.2e|%x|%o|%b\n", 3.14159, 12345.678, 255, 8, 5)
	fmt.Printf("%3d|%-3d|%03d|% d|%+d\n", 7, 7, 7, 7, 7)
	fmt.Printf("%t %v %5t|%-7t|\n", true, false, true, false)
}

// Output:
// 233 97 é a
// é 'é' U+00E9 U+00E9 'é' e9 233 233
// 日本語 '日'
//     é|é    |00233|'\u00e9'
// "héllo" "h\u00e9llo" `héllo` 68c3a96c6c6f 68 c3 a9 6c 6c 6f 68C3A96C6C6F
// "a\xffb" 61ff62 "\xff"
// 日本語|       日本語|日本語       |日本|        日本|
// 0:a(U+0061,97);1:é(U+00E9,233);3:日(U+65E5,26085);6:😀(U+1F600,128512);
// [26085 26412] [230 151 165 230 156 172] 日本
// [97 98] [%!s(int32=97) %!s(int32=98)] ['a' 'b'] [61 62]
// [a b c] [a b c] [A B]
// 'x' ['x' 'y']
// 0003.142|+1.23e+04|ff|10|101
//   7|7  |007| 7|+7
// true false  true|false  |
