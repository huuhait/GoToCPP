// phase: 09

package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	r, w, _ := os.Pipe()
	const total = 5 << 20
	go func() {
		chunk := make([]byte, 100000)
		for i := range chunk {
			chunk[i] = byte(i)
		}
		sent := 0
		for sent < total {
			n := len(chunk)
			if total-sent < n {
				n = total - sent
			}
			m, err := w.Write(chunk[:n])
			if err != nil || m != n {
				panic(fmt.Sprint(m, err))
			}
			sent += n
		}
		w.Close()
	}()
	n, err := io.Copy(io.Discard, r)
	fmt.Println(n, err)
}

// Output:
// 5242880 <nil>
