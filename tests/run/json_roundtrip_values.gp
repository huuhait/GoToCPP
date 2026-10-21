// phase: 09
// encoding/json round trips: seeded pseudo-random values of a large struct type (every scalar
// kind, slices, arrays, maps with several key types, pointers, interfaces, embedded structs, byte
// slices, custom marshalers, time) go through Marshal, Unmarshal into a fresh value, DeepEqual and
// Marshal again; MarshalIndent and an Encoder/Decoder pair must agree with Marshal.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"
)

type generator struct{ state uint64 }

func (g *generator) next() uint64 {
	g.state = g.state*6364136223846793005 + 1442695040888963407
	return g.state >> 20
}

func (g *generator) below(limit int) int { return int(g.next() % uint64(limit)) }

func (g *generator) chance() bool { return g.next()&1 == 0 }

var words = []string{"", "a", "word", "é", "日本語", "😀", "quote\"d", "back\\slash", "new\nline", "tab\t", "<html>&amp;", " ", "\x00", "mixed é\"\\"}

func (g *generator) word() string { return words[g.below(len(words))] }

func (g *generator) float() float64 {
	switch g.below(8) {
	case 0:
		return 0
	case 1:
		return math.Float64frombits(g.next() << 20 >> 1 &^ (0x7ff << 52))
	case 2:
		return float64(int64(g.next())) / 1000
	case 3:
		return math.Ldexp(1, g.below(2000)-1000)
	case 4:
		return -float64(g.below(1000000))
	case 5:
		return 1 / float64(g.below(1000)+1)
	}
	return float64(g.below(1000)) / 8
}

type Level uint8

type Label string

func (l Label) MarshalText() ([]byte, error) { return []byte("L:" + string(l)), nil }

func (l *Label) UnmarshalText(text []byte) error {
	*l = Label(strings.TrimPrefix(string(text), "L:"))
	return nil
}

type Version struct{ Major, Minor int }

func (v Version) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%d.%d"`, v.Major, v.Minor)), nil
}

func (v *Version) UnmarshalJSON(data []byte) error {
	_, err := fmt.Sscanf(string(data), `"%d.%d"`, &v.Major, &v.Minor)
	return err
}

type Common struct {
	ID   int64
	Tags []string `json:"tags,omitempty"`
}

type Mega struct {
	Common
	Flag     bool
	Int8     int8
	Int16    int16
	Int32    int32
	Int64    int64
	Uint8    uint8
	Uint16   uint16
	Uint32   uint32
	Uint64   uint64
	Float32  float32
	Float64  float64
	Text     string
	Level    Level
	Label    Label
	Version  Version
	Bytes    []byte
	Ints     []int
	Strings  []string
	Matrix   [][]float64
	Array    [3]int16
	ByString map[string]int
	ByInt    map[int]string
	ByUint8  map[uint8]bool
	ByLabel  map[Label]int
	Nested   map[string][]Common
	Pointer  *int
	PointerP **string
	Struct   *Common
	Any      any
	Anys     []any
	When     time.Time
	Raw      json.RawMessage
	Number   json.Number
	Optional *Version `json:",omitempty"`
	Quoted   int64    `json:",string"`
	QuotedF  float64  `json:",string"`
	QuotedB  bool     `json:",string"`
	Skipped  int      `json:"-"`
}

func (g *generator) anyValue(depth int) any {
	kind := g.below(7)
	if depth <= 0 && kind >= 5 {
		kind = g.below(5)
	}
	switch kind {
	case 0:
		return nil
	case 1:
		return g.chance()
	case 2:
		return g.float()
	case 3:
		return g.word()
	case 4:
		return float64(g.below(100))
	case 5:
		list := make([]any, g.below(4))
		for index := range list {
			list[index] = g.anyValue(depth - 1)
		}
		return list
	}
	object := map[string]any{}
	for count := g.below(4); count > 0; count-- {
		object[g.word()] = g.anyValue(depth - 1)
	}
	return object
}

func (g *generator) mega() Mega {
	var mega Mega
	mega.ID = int64(g.next()) >> g.below(60)
	for count := g.below(3); count > 0; count-- {
		mega.Tags = append(mega.Tags, g.word())
	}
	mega.Flag = g.chance()
	mega.Int8 = int8(g.next())
	mega.Int16 = int16(g.next())
	mega.Int32 = int32(g.next())
	mega.Int64 = int64(g.next()) << g.below(4)
	mega.Uint8 = uint8(g.next())
	mega.Uint16 = uint16(g.next())
	mega.Uint32 = uint32(g.next())
	mega.Uint64 = g.next()<<20 | g.next()
	narrow := g.float()
	if math.Abs(narrow) > 1e30 {
		narrow /= 1e300
	}
	mega.Float32 = float32(narrow)
	mega.Float64 = g.float()
	mega.Text = g.word() + g.word()
	mega.Level = Level(g.below(256))
	mega.Label = Label(g.word())
	mega.Version = Version{g.below(10), g.below(100)}
	if g.chance() {
		mega.Bytes = []byte(g.word())
	}
	for count := g.below(4); count > 0; count-- {
		mega.Ints = append(mega.Ints, int(int32(g.next())))
		mega.Strings = append(mega.Strings, g.word())
	}
	for row := g.below(3); row > 0; row-- {
		var line []float64
		for count := g.below(3); count > 0; count-- {
			line = append(line, g.float())
		}
		mega.Matrix = append(mega.Matrix, line)
	}
	for index := range mega.Array {
		mega.Array[index] = int16(g.next())
	}
	if g.chance() {
		mega.ByString = map[string]int{}
		mega.ByInt = map[int]string{}
		mega.ByUint8 = map[uint8]bool{}
		mega.ByLabel = map[Label]int{}
		for count := g.below(4); count > 0; count-- {
			mega.ByString[g.word()] = int(int16(g.next()))
			mega.ByInt[int(int16(g.next()))] = g.word()
			mega.ByUint8[uint8(g.next())] = g.chance()
			mega.ByLabel[Label(g.word())] = g.below(10)
		}
	}
	if g.chance() {
		mega.Nested = map[string][]Common{g.word(): {{ID: 1, Tags: []string{g.word()}}, {}}}
	}
	if g.chance() {
		number := int(int32(g.next()))
		mega.Pointer = &number
	}
	if g.chance() {
		text := g.word()
		pointer := &text
		mega.PointerP = &pointer
	}
	if g.chance() {
		mega.Struct = &Common{ID: int64(g.below(100)), Tags: []string{g.word()}}
	}
	mega.Any = g.anyValue(3)
	for count := g.below(3); count > 0; count-- {
		mega.Anys = append(mega.Anys, g.anyValue(2))
	}
	mega.When = time.Unix(int64(g.below(4000000000)), int64(g.below(1000000000))).UTC()
	mega.Raw = json.RawMessage(fmt.Sprintf(`{"raw":[%d, "x"]}`, g.below(100)))
	mega.Number = json.Number(fmt.Sprint(g.below(100000)))
	if g.chance() {
		mega.Optional = &Version{1, g.below(9)}
	}
	mega.Quoted = int64(g.next()) >> 10
	mega.QuotedF = g.float()
	mega.QuotedB = g.chance()
	mega.Skipped = 5
	return mega
}

func checksum(data []byte) uint32 {
	hash := uint32(2166136261)
	for _, value := range data {
		hash ^= uint32(value)
		hash *= 16777619
	}
	return hash
}

func main() {
	random := &generator{state: 777}
	equal, differing := 0, 0
	for seed := 0; seed < 120; seed++ {
		original := random.mega()
		encoded, err := json.Marshal(original)
		if err != nil {
			fmt.Printf("seed %03d marshal error %v\n", seed, err)
			continue
		}

		var back Mega
		err = json.Unmarshal(encoded, &back)
		reencoded, reErr := json.Marshal(back)

		// The skipped field and the raw message's spacing are expected differences.
		original.Skipped = 0
		original.Raw = nil
		back.Raw = nil
		same := reflect.DeepEqual(original, back)
		if same {
			equal++
		} else {
			differing++
		}

		var indented bytes.Buffer
		indentErr := json.Indent(&indented, encoded, "", "\t")
		var compacted bytes.Buffer
		_ = json.Compact(&compacted, indented.Bytes())
		var stream bytes.Buffer
		encoderErr := json.NewEncoder(&stream).Encode(back)
		var fromStream Mega
		decodeErr := json.NewDecoder(&stream).Decode(&fromStream)
		fmt.Printf("seed %03d len=%d json=%08x unmarshal=%v remarshal-same=%v(%v) deepEqual=%v indent=%v/%v streamed=%v/%v\n",
			seed, len(encoded), checksum(encoded), err, bytes.Equal(encoded, reencoded), reErr, same,
			indentErr, bytes.Equal(compacted.Bytes(), encoded), encoderErr, decodeErr)
		if !same {
			fmt.Printf("  differs: %s\n", firstDifference(original, back))
		}
		if seed < 3 {
			fmt.Printf("%s\n", encoded)
		}
	}
	fmt.Println("deep equal:", equal, "different:", differing)
}

// firstDifference names the first top-level field of Mega that differs.
func firstDifference(left, right Mega) string {
	leftValue := reflect.ValueOf(left)
	rightValue := reflect.ValueOf(right)
	for index := 0; index < leftValue.NumField(); index++ {
		if !reflect.DeepEqual(leftValue.Field(index).Interface(), rightValue.Field(index).Interface()) {
			return fmt.Sprintf("%s: %#v vs %#v", leftValue.Type().Field(index).Name, leftValue.Field(index).Interface(), rightValue.Field(index).Interface())
		}
	}
	return "none"
}

// Output:
// seed 000 len=766 json=bc005bf0 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// {"ID":19274951,"tags":["word"],"Flag":true,"Int8":-71,"Int16":-25248,"Int32":-735510616,"Int64":57659335025732,"Uint8":66,"Uint16":56340,"Uint32":207865893,"Uint64":15453028995279671252,"Float32":26,"Float64":18,"Text":"quote\"dquote\"d","Level":216,"Label":"L:\u003chtml\u003e\u0026amp;","Version":"1.71","Bytes":"8J+YgA==","Ints":[-2002819791,1949188092,-311531190],"Strings":["","word","日本語"],"Matrix":[[9820359962.357]],"Array":[3407,306,13798],"ByString":null,"ByInt":null,"ByUint8":null,"ByLabel":null,"Nested":null,"Pointer":null,"PointerP":"mixed é\"\\","Struct":null,"Any":"new\nline","Anys":null,"When":"2013-08-30T04:59:35.060167125Z","Raw":{"raw":[96,"x"]},"Number":45376,"Quoted":"2920944396","QuotedF":"1.0959046745042015e-193","QuotedB":"true"}
// seed 001 len=705 json=2c9417c5 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// {"ID":0,"Flag":true,"Int8":94,"Int16":-21494,"Int32":-989613523,"Int64":4334483570458,"Uint8":170,"Uint16":55312,"Uint32":1487448285,"Uint64":13189688652998896422,"Float32":0,"Float64":8.875,"Text":"\u0000tab\t","Level":205,"Label":"L:\u003chtml\u003e\u0026amp;","Version":"3.68","Bytes":"w6k=","Ints":null,"Strings":null,"Matrix":null,"Array":[10166,-8068,32542],"ByString":null,"ByInt":null,"ByUint8":null,"ByLabel":null,"Nested":null,"Pointer":null,"PointerP":"\u003chtml\u003e\u0026amp;","Struct":{"ID":21,"tags":["quote\"d"]},"Any":{},"Anys":[67],"When":"2090-04-25T03:37:45.209344996Z","Raw":{"raw":[58,"x"]},"Number":44368,"Quoted":"8652488929","QuotedF":"1.3888460079128273e-308","QuotedB":"true"}
// seed 002 len=863 json=d63e3550 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// {"ID":482053822,"tags":["back\\slash"],"Flag":false,"Int8":3,"Int16":19419,"Int32":-2024915666,"Int64":3651527454115,"Uint8":89,"Uint16":52424,"Uint32":909904194,"Uint64":7751729598825224073,"Float32":-766701,"Float64":1.844884615252251e-308,"Text":"a\u0000","Level":5,"Label":"L:\u2028","Version":"8.17","Bytes":"YQ==","Ints":[-912277615,-1491968131],"Strings":["\u003chtml\u003e\u0026amp;","😀"],"Matrix":[[16330242470.456,3.111507638930571e-61],[0]],"Array":[12411,11443,10783],"ByString":{"quote\"d":23500,"word":14682},"ByInt":{"-20144":"é","-31964":"a"},"ByUint8":{"51":false,"90":true},"ByLabel":{"back\\slash":4,"é":2},"Nested":null,"Pointer":-1048749257,"PointerP":"mixed é\"\\","Struct":null,"Any":0,"Anys":null,"When":"2084-05-17T07:25:26.545406552Z","Raw":{"raw":[52,"x"]},"Number":36146,"Quoted":"1107442717","QuotedF":"95.875","QuotedB":"true"}
// seed 003 len=951 json=aad25ab8 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 004 len=764 json=7071a83d unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 005 len=783 json=7587839f unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 006 len=910 json=b4096d12 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 007 len=791 json=ff4bbdf9 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 008 len=932 json=c6328248 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 009 len=919 json=20a64acc unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 010 len=804 json=40d7e82f unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 011 len=869 json=b163447a unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 012 len=828 json=4344e522 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 013 len=640 json=dad888b1 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 014 len=746 json=feb08406 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 015 len=901 json=1075fa91 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 016 len=816 json=98af6687 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 017 len=926 json=824dda21 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 018 len=734 json=bf1f9f4c unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 019 len=754 json=22a882a8 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 020 len=733 json=a68589b1 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 021 len=937 json=f7501f56 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 022 len=861 json=69f82aee unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 023 len=825 json=368d0cb2 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 024 len=901 json=7ca1dd6d unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 025 len=921 json=d3eb200c unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 026 len=753 json=561edd2b unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 027 len=1018 json=b02446d2 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 028 len=750 json=ecfe7a1b unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 029 len=776 json=24d61b4a unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 030 len=730 json=17c23371 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 031 len=773 json=69c79ed5 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 032 len=670 json=57d6cb86 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 033 len=824 json=a848fa22 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 034 len=856 json=9998aafa unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 035 len=806 json=ccdc6b29 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 036 len=942 json=755da525 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 037 len=897 json=8fcd68ff unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 038 len=688 json=d7b40847 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 039 len=703 json=654795ef unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 040 len=726 json=08c42051 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 041 len=706 json=617a2fbd unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 042 len=801 json=296ebbd2 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 043 len=838 json=742db002 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 044 len=769 json=20d1cf9a unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 045 len=798 json=7216b3e3 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 046 len=750 json=45b21509 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 047 len=752 json=1ed63477 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 048 len=897 json=d670482c unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 049 len=794 json=78e4994a unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 050 len=882 json=8182eca5 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 051 len=813 json=dc9bddb7 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 052 len=776 json=8809fe9e unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 053 len=894 json=55646841 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 054 len=845 json=d4b882c0 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 055 len=853 json=2242176a unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 056 len=699 json=93b40816 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 057 len=775 json=49ee8ba8 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 058 len=896 json=575840ca unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 059 len=958 json=15911c16 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 060 len=825 json=aaa5cbcb unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 061 len=858 json=68702220 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 062 len=943 json=41a500d2 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 063 len=895 json=8967ab5a unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 064 len=663 json=7f4b921d unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 065 len=776 json=6de7416e unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 066 len=786 json=81e1110f unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 067 len=717 json=b2b038dd unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 068 len=755 json=334288b4 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 069 len=742 json=f0fc9439 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 070 len=769 json=b040ee0a unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 071 len=812 json=6b8414cf unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 072 len=865 json=a8a05dbb unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 073 len=884 json=0e7ec173 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 074 len=722 json=6e486a20 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 075 len=783 json=8d522475 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 076 len=854 json=de927097 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 077 len=754 json=9d6c8b25 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 078 len=838 json=1265f0e4 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 079 len=714 json=dd067389 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 080 len=879 json=701929de unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 081 len=914 json=5220d27e unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 082 len=914 json=1fbd5ae4 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 083 len=732 json=47652a84 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 084 len=777 json=d4644a76 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 085 len=763 json=1e95ea23 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 086 len=860 json=19c0f7a4 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 087 len=832 json=d4f787a9 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 088 len=731 json=783d0eda unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 089 len=881 json=4bbe8b56 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 090 len=740 json=b8195b89 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 091 len=784 json=816c69e1 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 092 len=783 json=637f7007 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 093 len=768 json=cb04e52e unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 094 len=790 json=d3a664ef unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 095 len=698 json=a5612d43 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 096 len=885 json=83162d80 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 097 len=938 json=32f05185 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 098 len=873 json=d02d6da2 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 099 len=864 json=4251e882 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 100 len=895 json=12fc1896 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 101 len=732 json=071b5b0b unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 102 len=943 json=d5fcc1cb unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 103 len=889 json=c68d653e unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 104 len=820 json=26ec7fff unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 105 len=777 json=5d631621 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 106 len=825 json=374deb59 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 107 len=790 json=3c953ddd unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 108 len=860 json=f5a77410 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 109 len=859 json=1a496755 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 110 len=880 json=8cb55839 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 111 len=865 json=9a2304f3 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 112 len=820 json=e57ba354 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 113 len=678 json=3f6ac5de unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 114 len=774 json=b3c14bee unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 115 len=915 json=40c22e20 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 116 len=873 json=77620c1d unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 117 len=881 json=d8fb0d65 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 118 len=1072 json=d3517ba5 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// seed 119 len=1003 json=28a134e3 unmarshal=<nil> remarshal-same=true(<nil>) deepEqual=true indent=<nil>/true streamed=<nil>/<nil>
// deep equal: 120 different: 0
