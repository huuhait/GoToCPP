// phase: 09
// encoding/json against io: a Decoder fed one byte at a time or in odd chunk sizes, readers that
// fail or return data with an error, documents larger than the decoder's buffer, a stream of many
// values, an Encoder writing to a failing or short writer, and large values in both directions.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type chunkReader struct {
	data  []byte
	chunk int
}

func (r *chunkReader) Read(buffer []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	count := min(r.chunk, len(buffer), len(r.data))
	copy(buffer, r.data[:count])
	r.data = r.data[count:]
	return count, nil
}

type dataWithError struct {
	data []byte
	err  error
}

func (r *dataWithError) Read(buffer []byte) (int, error) {
	count := copy(buffer, r.data)
	r.data = r.data[count:]
	if len(r.data) == 0 {
		return count, r.err
	}
	return count, nil
}

type failingWriter struct {
	budget int
	err    error
}

func (w *failingWriter) Write(data []byte) (int, error) {
	if len(data) > w.budget {
		written := w.budget
		w.budget = 0
		return written, w.err
	}
	w.budget -= len(data)
	return len(data), nil
}

type shortWriter struct{}

func (shortWriter) Write(data []byte) (int, error) { return len(data) / 2, nil }

type Record struct {
	ID    int
	Name  string
	Tags  []string
	Attrs map[string]any
}

func makeRecords(count int) []Record {
	records := make([]Record, count)
	for index := range records {
		records[index] = Record{
			ID:    index,
			Name:  strings.Repeat("n", index%37) + fmt.Sprint(index),
			Tags:  []string{"a", strings.Repeat("t", index%11)},
			Attrs: map[string]any{"k": index, "s": strings.Repeat("é", index%5), "n": nil, "l": []any{1.5, true}},
		}
	}
	return records
}

func main() {
	records := makeRecords(200)
	stream := &bytes.Buffer{}
	encoder := json.NewEncoder(stream)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			fmt.Println("encode error", err)
		}
	}
	data := append([]byte(nil), stream.Bytes()...)
	fmt.Println("stream bytes:", len(data))

	fmt.Println("== chunk sizes")
	for _, chunk := range []int{1, 2, 3, 7, 64, 511, 512, 513, 4096, 1 << 20} {
		decoder := json.NewDecoder(&chunkReader{data: data, chunk: chunk})
		count, lastID := 0, -1
		var failure error
		for {
			var record Record
			err := decoder.Decode(&record)
			if err == io.EOF {
				break
			}
			if err != nil {
				failure = err
				break
			}
			count++
			lastID = record.ID
		}
		fmt.Printf("chunk %7d: %d records, last %d, offset %d, err %v\n", chunk, count, lastID, decoder.InputOffset(), failure)
	}

	fmt.Println("== one big document")
	big := makeRecords(3000)
	encoded, err := json.Marshal(big)
	fmt.Println(len(encoded), err)
	for _, chunk := range []int{1, 100, 4096} {
		var back []Record
		decoder := json.NewDecoder(&chunkReader{data: encoded, chunk: chunk})
		err = decoder.Decode(&back)
		again, _ := json.Marshal(back)
		fmt.Printf("chunk %5d: %d records, same=%v, offset %d, err %v\n", chunk, len(back), bytes.Equal(again, encoded), decoder.InputOffset(), err)
	}
	var viaUnmarshal []Record
	err = json.Unmarshal(encoded, &viaUnmarshal)
	fmt.Println(len(viaUnmarshal), err)
	indented, err := json.MarshalIndent(big[:50], ">", "  ")
	fmt.Println(len(indented), err)
	hugeString := strings.Repeat("0123456789abcdef", 100000)
	encoded, err = json.Marshal(hugeString)
	fmt.Println(len(encoded), err)
	var hugeBack string
	err = json.Unmarshal(encoded, &hugeBack)
	fmt.Println(hugeBack == hugeString, err)
	hugeList := make([]int, 200000)
	for index := range hugeList {
		hugeList[index] = index * 7
	}
	encoded, err = json.Marshal(hugeList)
	var hugeListBack []int
	err2 := json.Unmarshal(encoded, &hugeListBack)
	fmt.Println(len(encoded), err, len(hugeListBack), err2, hugeListBack[199999])
	manyKeys := make(map[string]int, 20000)
	for index := 0; index < 20000; index++ {
		manyKeys[fmt.Sprintf("key-%05d", (index*7919)%20000)] = index
	}
	encoded, err = json.Marshal(manyKeys)
	var manyBack map[string]int
	err2 = json.Unmarshal(encoded, &manyBack)
	fmt.Println(len(encoded), err, len(manyBack), err2, bytes.HasPrefix(encoded, []byte(`{"key-00000":`)))

	fmt.Println("== readers that fail")
	readFailure := errors.New("disk on fire")
	for _, input := range []struct {
		label string
		data  string
		err   error
	}{
		{"complete then error", `{"ID":1}`, readFailure},
		{"complete then EOF", `{"ID":2}`, io.EOF},
		{"truncated then EOF", `{"ID":2`, io.EOF},
		{"truncated then error", `{"ID":2`, readFailure},
		{"empty then error", ``, readFailure},
		{"empty then EOF", ``, io.EOF},
		{"two then error", `{"ID":1} {"ID":2}`, readFailure},
		{"token then error", `[1,2`, readFailure},
		{"unexpected EOF", `{"ID"`, io.ErrUnexpectedEOF},
	} {
		decoder := json.NewDecoder(&dataWithError{data: []byte(input.data), err: input.err})
		var results []string
		for round := 0; round < 3; round++ {
			var record Record
			err := decoder.Decode(&record)
			results = append(results, fmt.Sprintf("%d/%v", record.ID, err))
			if err != nil {
				break
			}
		}
		fmt.Printf("%-22s %q\n", input.label, results)
	}
	decoder := json.NewDecoder(&dataWithError{data: []byte(`[1,2`), err: readFailure})
	for round := 0; round < 5; round++ {
		token, err := decoder.Token()
		fmt.Printf("token %v err %v more %v\n", token, err, decoder.More())
	}
	decoder = json.NewDecoder(strings.NewReader(`{"a":1`))
	var partial map[string]int
	err = decoder.Decode(&partial)
	err2 = decoder.Decode(&partial)
	fmt.Println(partial, err, err2, err == err2)

	fmt.Println("== failing writers")
	for _, budget := range []int{0, 1, 10, 100000} {
		writer := &failingWriter{budget: budget, err: errors.New("pipe closed")}
		encoder := json.NewEncoder(writer)
		first := encoder.Encode(records[:3])
		second := encoder.Encode("again")
		fmt.Printf("budget %6d: %v | %v | left %d\n", budget, first, second, writer.budget)
	}
	err = json.NewEncoder(shortWriter{}).Encode(map[string]int{"a": 1})
	fmt.Println("short writer:", err)
	err = json.NewEncoder(io.Discard).Encode(map[string]any{"bad": make(chan int)})
	fmt.Println("discard:", err)
	var sink strings.Builder
	encoder = json.NewEncoder(&sink)
	for index := 0; index < 5; index++ {
		_ = encoder.Encode(index)
	}
	fmt.Printf("%q\n", sink.String())
}

// Output:
// stream bytes: 22696
// == chunk sizes
// chunk       1: 200 records, last 199, offset 22695, err <nil>
// chunk       2: 200 records, last 199, offset 22695, err <nil>
// chunk       3: 200 records, last 199, offset 22695, err <nil>
// chunk       7: 200 records, last 199, offset 22695, err <nil>
// chunk      64: 200 records, last 199, offset 22695, err <nil>
// chunk     511: 200 records, last 199, offset 22695, err <nil>
// chunk     512: 200 records, last 199, offset 22695, err <nil>
// chunk     513: 200 records, last 199, offset 22695, err <nil>
// chunk    4096: 200 records, last 199, offset 22695, err <nil>
// chunk 1048576: 200 records, last 199, offset 22695, err <nil>
// == one big document
// 353608 <nil>
// chunk     1: 3000 records, same=true, offset 353608, err <nil>
// chunk   100: 3000 records, same=true, offset 353608, err <nil>
// chunk  4096: 3000 records, same=true, offset 353608, err <nil>
// 3000 <nil>
// 11852 <nil>
// 1600002 <nil>
// true <nil>
// 1441268 <nil> 200000 <nil> 1399993
// 348891 <nil> 20000 <nil> true
// == readers that fail
// complete then error    ["1/<nil>" "0/disk on fire"]
// complete then EOF      ["2/<nil>" "0/EOF"]
// truncated then EOF     ["0/unexpected EOF"]
// truncated then error   ["0/disk on fire"]
// empty then error       ["0/disk on fire"]
// empty then EOF         ["0/EOF"]
// two then error         ["1/<nil>" "2/<nil>" "0/disk on fire"]
// token then error       ["0/disk on fire"]
// unexpected EOF         ["0/unexpected EOF"]
// token [ err <nil> more true
// token 1 err <nil> more true
// token <nil> err disk on fire more true
// token <nil> err disk on fire more true
// token <nil> err disk on fire more true
// map[] unexpected EOF unexpected EOF true
// == failing writers
// budget      0: pipe closed | pipe closed | left 0
// budget      1: pipe closed | pipe closed | left 0
// budget     10: pipe closed | pipe closed | left 0
// budget 100000: <nil> | <nil> | left 99729
// short writer: <nil>
// discard: json: unsupported type: chan int
// "0\n1\n2\n3\n4\n"
