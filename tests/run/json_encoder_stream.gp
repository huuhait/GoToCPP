// phase: 09
// encoding/json Encoder: Encode appends a newline, SetIndent, SetEscapeHTML, writers that fail, many values to one writer, errors do not write.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
)

type Row struct {
	ID   int      `json:"id"`
	Name string   `json:"name"`
	Tags []string `json:"tags,omitempty"`
}

type failWriter struct{ n int }

func (f *failWriter) Write(p []byte) (int, error) {
	if f.n <= 0 {
		return 0, errors.New("write failed")
	}
	f.n--
	return len(p), nil
}

func main() {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for i, name := range []string{"a<b", "c&d", "e\"f"} {
		if err := enc.Encode(Row{ID: i, Name: name, Tags: []string{"x"}[:i%2]}); err != nil {
			fmt.Println(err)
		}
	}
	fmt.Print(buf.String())

	buf.Reset()
	enc = json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.Encode(Row{ID: 1, Name: "x", Tags: []string{"p", "q"}})
	enc.Encode([]int{})
	enc.Encode(map[string]any{"k": []any{1, map[string]any{}}})
	fmt.Print(buf.String())

	buf.Reset()
	enc = json.NewEncoder(&buf)
	enc.SetIndent("> ", "\t")
	enc.Encode(map[string]int{"a": 1, "b": 2})
	enc.SetIndent("", "")
	enc.Encode(map[string]int{"a": 1, "b": 2})
	fmt.Printf("%q\n", buf.String())

	buf.Reset()
	enc = json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.Encode("<tag>&")
	enc.SetEscapeHTML(true)
	enc.Encode("<tag>&")
	fmt.Print(buf.String())

	// An error writes nothing.
	buf.Reset()
	enc = json.NewEncoder(&buf)
	err := enc.Encode(math.NaN())
	fmt.Println(err, buf.Len())
	err = enc.Encode(make(chan int))
	fmt.Println(err, buf.Len())
	err = enc.Encode([]any{1, math.Inf(1)})
	fmt.Println(err, buf.Len())
	enc.Encode(1)
	fmt.Printf("%q\n", buf.String())

	// A failing writer.
	fw := &failWriter{n: 1}
	enc = json.NewEncoder(fw)
	fmt.Println(enc.Encode(1))
	fmt.Println(enc.Encode(2))
	fmt.Println(enc.Encode(3))

	// Scalars and top-level values.
	buf.Reset()
	enc = json.NewEncoder(&buf)
	for _, v := range []any{nil, true, 1, 2.5, "s", []byte("hi"), [2]int{1, 2}, struct{}{}, (*int)(nil), json.RawMessage(`{ "a" : 1 }`), json.Number("7")} {
		enc.Encode(v)
	}
	fmt.Print(buf.String())

	// Encoding to stdout.
	enc = json.NewEncoder(os.Stdout)
	enc.Encode(map[string][]string{"stdout": {"works"}})

	// Round trip through Encoder and Decoder.
	buf.Reset()
	enc = json.NewEncoder(&buf)
	for i := 0; i < 3; i++ {
		enc.Encode(Row{ID: i, Name: strings.Repeat("n", i)})
	}
	dec := json.NewDecoder(&buf)
	for dec.More() {
		var r Row
		if err := dec.Decode(&r); err != nil {
			fmt.Println(err)
			break
		}
		fmt.Printf("%+v\n", r)
	}
}

// Output:
// {"id":0,"name":"a\u003cb"}
// {"id":1,"name":"c\u0026d","tags":["x"]}
// {"id":2,"name":"e\"f"}
// {
//   "id": 1,
//   "name": "x",
//   "tags": [
//     "p",
//     "q"
//   ]
// }
// []
// {
//   "k": [
//     1,
//     {}
//   ]
// }
// "{\n> \t\"a\": 1,\n> \t\"b\": 2\n> }\n{\"a\":1,\"b\":2}\n"
// "<tag>&"
// "\u003ctag\u003e\u0026"
// json: unsupported value: NaN 0
// json: unsupported type: chan int 0
// json: unsupported value: +Inf 0
// "1\n"
// <nil>
// write failed
// write failed
// null
// true
// 1
// 2.5
// "s"
// "aGk="
// [1,2]
// {}
// null
// {"a":1}
// 7
// {"stdout":["works"]}
// {ID:0 Name: Tags:[]}
// {ID:1 Name:n Tags:[]}
// {ID:2 Name:nn Tags:[]}
