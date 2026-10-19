// bench: bytes_buffer
// category: data
// size: 40000000
// smoke: 2000
// twin: no
// about: bytes.Buffer writes and reads, bytes.Equal and bytes.IndexByte
package main

import (
	"bytes"
	"fmt"
)

func main() {
	var buf bytes.Buffer
	total := 0
	chunk := []byte("0123456789abcdef")
	for i := 0; i < benchN; i++ {
		buf.Write(chunk)
		buf.WriteByte(byte('a' + i%26))
		if buf.Len() > 4096 {
			total += bytes.IndexByte(buf.Bytes(), 'z') + buf.Len()
			if bytes.Equal(buf.Bytes()[:16], chunk) {
				total++
			}
			buf.Reset()
		}
	}
	fmt.Println(total)
}
