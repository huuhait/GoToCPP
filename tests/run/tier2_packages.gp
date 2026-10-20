// phase: 09
// The small tier-2 packages of step 9.11: encoding/hex, encoding/binary, container/heap, hash/fnv, hash/crc32 and text/tabwriter (container/list and container/ring are in containers_cycles.gp).

package main

import (
	"bytes"
	"container/heap"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"hash/crc32"
	"hash/fnv"
	"os"
	"strings"
	"text/tabwriter"
)

type Item struct {
	Name     string
	Priority int
	index    int
}

type PQ []*Item

func (pq PQ) Len() int            { return len(pq) }
func (pq PQ) Less(i, j int) bool  { return pq[i].Priority > pq[j].Priority }
func (pq PQ) Swap(i, j int)       { pq[i], pq[j] = pq[j], pq[i]; pq[i].index = i; pq[j].index = j }
func (pq *PQ) Push(x any)         { it := x.(*Item); it.index = len(*pq); *pq = append(*pq, it) }
func (pq *PQ) Pop() any {
	old := *pq
	n := len(old)
	it := old[n-1]
	old[n-1] = nil
	*pq = old[:n-1]
	return it
}

type Header struct {
	Magic   [4]byte
	Version uint16
	Flags   int16
	Size    uint32
	Ratio   float32
	Offset  int64
	OK      bool
}

func main() {
	// encoding/hex
	src := []byte("Hello, Gopher! \x00\xff")
	e := hex.EncodeToString(src)
	d, err := hex.DecodeString(e)
	fmt.Println(e, string(d) == string(src), err)
	_, err = hex.DecodeString("abc")
	fmt.Println(err)
	_, err = hex.DecodeString("zz")
	fmt.Println(err)
	fmt.Print(hex.Dump([]byte("The quick brown fox jumps over the lazy dog.")))
	dec := hex.NewDecoder(strings.NewReader("48656c6c6f2c20776f726c6421"))
	all, err := io.ReadAll(dec)
	fmt.Printf("%q %v\n", all, err)
	dec = hex.NewDecoder(strings.NewReader("4865zz"))
	all, err = io.ReadAll(dec)
	fmt.Printf("%q %v\n", all, err)
	fmt.Println(hex.EncodedLen(5), hex.DecodedLen(10), string(hex.AppendEncode([]byte("x:"), []byte{1, 171})))

	// encoding/binary
	var buf bytes.Buffer
	h := Header{[4]byte{'G', 'P', 'C', 'P'}, 3, -2, 1 << 20, 1.5, -1 << 40, true}
	fmt.Println(binary.Write(&buf, binary.LittleEndian, h), buf.Len(), binary.Size(h))
	fmt.Println(hex.EncodeToString(buf.Bytes()))
	var h2 Header
	fmt.Println(binary.Read(bytes.NewReader(buf.Bytes()), binary.LittleEndian, &h2), h2 == h)
	var be bytes.Buffer
	binary.Write(&be, binary.BigEndian, []uint16{1, 2, 0xabcd})
	fmt.Println(be.Bytes())
	b := binary.BigEndian.AppendUint32(nil, 0xdeadbeef)
	b = binary.LittleEndian.AppendUint16(b, 0x1234)
	fmt.Println(b, binary.BigEndian.Uint32(b), binary.LittleEndian.Uint64(append(b, 0, 0)))
	vb := binary.AppendUvarint(nil, 300)
	vb = binary.AppendVarint(vb, -12345)
	u, n1 := binary.Uvarint(vb)
	s, n2 := binary.Varint(vb[n1:])
	fmt.Println(vb, u, n1, s, n2)
	fmt.Println(binary.Read(bytes.NewReader([]byte{1}), binary.BigEndian, &h2))
	fmt.Println(binary.Write(&buf, binary.BigEndian, map[string]int{}))

	// container/heap
	items := map[string]int{"banana": 3, "apple": 2, "pear": 4}
	pq := make(PQ, 0)
	heap.Init(&pq)
	for _, name := range []string{"banana", "apple", "pear"} {
		heap.Push(&pq, &Item{Name: name, Priority: items[name]})
	}
	it := &Item{Name: "orange", Priority: 1}
	heap.Push(&pq, it)
	it.Priority = 5
	heap.Fix(&pq, it.index)
	var vals []string
	for pq.Len() > 0 {
		x := heap.Pop(&pq).(*Item)
		vals = append(vals, fmt.Sprint(x.Name, ":", x.Priority))
	}
	fmt.Println(strings.Join(vals, " "))

	// hash/fnv, hash/crc32
	for _, in := range []string{"", "a", "hello world", strings.Repeat("xyz", 1000)} {
		h32 := fnv.New32()
		h32a := fnv.New32a()
		h64 := fnv.New64()
		h64a := fnv.New64a()
		h128 := fnv.New128a()
		for _, h := range []interface{ Write([]byte) (int, error) }{h32, h32a, h64, h64a, h128} {
			h.Write([]byte(in))
		}
		fmt.Printf("%d %d %d %d %x | crc ieee=%08x castagnoli=%08x koopman=%08x\n", h32.Sum32(), h32a.Sum32(), h64.Sum64(), h64a.Sum64(), h128.Sum(nil),
			crc32.ChecksumIEEE([]byte(in)), crc32.Checksum([]byte(in), crc32.MakeTable(crc32.Castagnoli)), crc32.Checksum([]byte(in), crc32.MakeTable(crc32.Koopman)))
	}
	c := crc32.NewIEEE()
	c.Write([]byte("hello "))
	c.Write([]byte("world"))
	fmt.Printf("%08x %08x\n", c.Sum32(), crc32.Update(crc32.ChecksumIEEE([]byte("hello ")), crc32.IEEETable, []byte("world")))

	// text/tabwriter
	w := tabwriter.NewWriter(os.Stdout, 5, 0, 1, ' ', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintln(w, "a\tb\tc\t")
	fmt.Fprintln(w, "aa\tbb\tcc\t")
	fmt.Fprintln(w, "aaa\t")
	fmt.Fprintln(w, "aaaa\tdddd\teeee\t")
	w.Flush()
	w = tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', 0)
	fmt.Fprintln(w, "Name\tQty\tPrice")
	fmt.Fprintln(w, "Widget\t12\t3.50")
	fmt.Fprintln(w, "Gadget with a long name\t3\t120.00")
	w.Flush()
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, '.', tabwriter.TabIndent|tabwriter.FilterHTML)
	fmt.Fprintln(w, "<b>héllo</b>\t世界\tx&amp;y")
	fmt.Fprintln(w, "z\t\tq")
	w.Flush()
}

// Output:
// 48656c6c6f2c20476f70686572212000ff true <nil>
// encoding/hex: odd length hex string
// encoding/hex: invalid byte: U+007A 'z'
// 00000000  54 68 65 20 71 75 69 63  6b 20 62 72 6f 77 6e 20  |The quick brown |
// 00000010  66 6f 78 20 6a 75 6d 70  73 20 6f 76 65 72 20 74  |fox jumps over t|
// 00000020  68 65 20 6c 61 7a 79 20  64 6f 67 2e              |he lazy dog.|
// "Hello, world!" <nil>
// "He" encoding/hex: invalid byte: U+007A 'z'
// 10 5 x:01ab
// <nil> 25 25
// 475043500300feff000010000000c03f0000000000ffffff01
// <nil> true
// [0 1 0 2 171 205]
// [222 173 190 239 52 18] 3735928559 20018569850334
// [172 2 241 192 1] 300 2 -12345 3
// unexpected EOF
// binary.Write: some values are not fixed-sized in type map[string]int
// orange:5 pear:4 banana:3 apple:2
// 2166136261 2166136261 14695981039346656037 14695981039346656037 6c62272e07bb014262b821756295c58d | crc ieee=00000000 castagnoli=00000000 koopman=00000000
// 84696446 3826002220 12638153115695167422 12638187200555641996 d228cb696f1a8caf78912b704e4a8964 | crc ieee=e8b7be43 castagnoli=c1d04330 koopman=0da2aa8a
// 1418570095 3582672807 9065573210506989167 8618312879776256743 6c155799fdc8eec4b91523808e7726b7 | crc ieee=0d4a1185 castagnoli=c99465aa koopman=df373d3c
// 1412783741 2250301229 11876344552830686557 14816048344047622925 e63a177d626c0bb9446017cba0c89455 | crc ieee=d88998a9 castagnoli=975a421a koopman=05d5a583
// 0d4a1185 0d4a1185
//     a|    b|    c|
//    aa|   bb|   cc|
//   aaa|
//  aaaa| dddd| eeee|
// Name			Qty	Price
// Widget			12	3.50
// Gadget with a long name	3	120.00
// <b>héllo</b>..世界..x&amp;y
// z..........q
