// phase: 09
// encoding/json maps: sorted keys, integer and string-kind keys, unsupported key types, nil values, nested, maps of interfaces.

package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Name string
type ID int
type U8 uint8

func show(label string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		fmt.Printf("%s: error: %v\n", label, err)
		return
	}
	fmt.Printf("%s: %s\n", label, b)
}

type Rec struct {
	M map[string]any `json:"m"`
	N map[int]string `json:"n,omitempty"`
}

func main() {
	show("sorted", map[string]int{"zebra": 1, "apple": 2, "mango": 3, "Banana": 4, "": 5, "é": 6, "z": 7, "aa": 8, "a": 9})
	show("int keys", map[int]string{10: "ten", 2: "two", -1: "neg", 100: "hundred", 0: "zero"})
	show("uint keys", map[uint8]bool{200: true, 3: false})
	show("int64 keys", map[int64]int{-9223372036854775808: 1, 9223372036854775807: 2})
	show("string kind keys", map[Name]int{"b": 1, "a": 2})
	show("int kind keys", map[ID]int{3: 1, 1: 2, 20: 3})
	show("U8 keys", map[U8]int{9: 1, 10: 2})
	show("bool keys", map[bool]int{true: 1})
	show("float keys", map[float64]int{1.5: 1})
	show("array keys", map[[2]int]int{{1, 2}: 1})
	show("struct keys", map[struct{ A int }]int{{1}: 1})
	show("pointer keys", map[*int]int{})
	show("interface keys", map[any]int{"a": 1})
	show("any values", map[string]any{"i": 1, "s": "s", "f": 1.5, "n": nil, "b": true, "l": []any{1, "x"}, "m": map[string]any{"deep": map[string]any{"deeper": []int{1}}}})
	show("nil slice values", map[string][]int{"a": nil, "b": {}})
	show("struct values", map[string]struct{ X int }{"k": {1}})
	show("pointer values", map[string]*int{"a": nil})
	show("map in struct", Rec{M: map[string]any{"x": 1}, N: map[int]string{2: "b", 1: "a"}})
	show("empty struct map", Rec{})
	show("big map", func() map[string]int {
		m := map[string]int{}
		for i := 0; i < 200; i++ {
			m[strings.Repeat("k", i%7)+fmt.Sprint(i*37%101)] = i
		}
		return m
	}())
	m := map[string]any{}
	m["self"] = 1
	show("map key with special chars", map[string]int{"a\"b": 1, "c\\d": 2, "\t": 3, "<>": 4})
	show("map[string]map", map[string]map[int]int{"a": {2: 1, 1: 2}, "b": nil})
	show("slice of maps", []map[string]int{{"a": 1}, nil, {}})
}

// Output:
// sorted: {"":5,"Banana":4,"a":9,"aa":8,"apple":2,"mango":3,"z":7,"zebra":1,"é":6}
// int keys: {"-1":"neg","0":"zero","10":"ten","100":"hundred","2":"two"}
// uint keys: {"200":true,"3":false}
// int64 keys: {"-9223372036854775808":1,"9223372036854775807":2}
// string kind keys: {"a":2,"b":1}
// int kind keys: {"1":2,"20":3,"3":1}
// U8 keys: {"10":2,"9":1}
// bool keys: error: json: unsupported type: map[bool]int
// float keys: error: json: unsupported type: map[float64]int
// array keys: error: json: unsupported type: map[[2]int]int
// struct keys: error: json: unsupported type: map[struct { A int }]int
// pointer keys: error: json: unsupported type: map[*int]int
// interface keys: error: json: unsupported type: map[interface {}]int
// any values: {"b":true,"f":1.5,"i":1,"l":[1,"x"],"m":{"deep":{"deeper":[1]}},"n":null,"s":"s"}
// nil slice values: {"a":null,"b":[]}
// struct values: {"k":{"X":1}}
// pointer values: {"a":null}
// map in struct: {"m":{"x":1},"n":{"1":"a","2":"b"}}
// empty struct map: {"m":null}
// big map: {"0":0,"11":175,"13":14,"16":126,"21":77,"24":189,"26":28,"29":140,"3":112,"34":91,"39":42,"42":154,"47":105,"52":56,"55":168,"57":7,"60":119,"65":70,"68":182,"70":21,"73":133,"78":84,"8":63,"81":196,"83":35,"86":147,"91":98,"96":49,"99":161,"k1":71,"k14":85,"k17":197,"k19":36,"k22":148,"k27":99,"k32":50,"k35":162,"k37":1,"k4":183,"k40":113,"k45":64,"k48":176,"k50":15,"k53":127,"k58":78,"k6":22,"k61":190,"k63":29,"k66":141,"k71":92,"k76":43,"k79":155,"k84":106,"k89":57,"k9":134,"k92":169,"k94":8,"k97":120,"kk100":30,"kk12":44,"kk15":156,"kk2":142,"kk20":107,"kk25":58,"kk28":170,"kk30":9,"kk33":121,"kk38":72,"kk41":184,"kk43":23,"kk46":135,"kk51":86,"kk54":198,"kk56":37,"kk59":149,"kk64":100,"kk69":51,"kk7":93,"kk72":163,"kk74":2,"kk77":114,"kk82":65,"kk85":177,"kk87":16,"kk90":128,"kk95":79,"kk98":191,"kkk0":101,"kkk10":3,"kkk13":115,"kkk18":66,"kkk21":178,"kkk23":17,"kkk26":129,"kkk31":80,"kkk34":192,"kkk36":31,"kkk39":143,"kkk44":94,"kkk49":45,"kkk5":52,"kkk52":157,"kkk57":108,"kkk62":59,"kkk65":171,"kkk67":10,"kkk70":122,"kkk75":73,"kkk78":185,"kkk8":164,"kkk80":24,"kkk83":136,"kkk88":87,"kkk91":199,"kkk93":38,"kkk96":150,"kkkk1":172,"kkkk11":74,"kkkk14":186,"kkkk16":25,"kkkk19":137,"kkkk24":88,"kkkk29":39,"kkkk3":11,"kkkk32":151,"kkkk37":102,"kkkk42":53,"kkkk45":165,"kkkk47":4,"kkkk50":116,"kkkk55":67,"kkkk58":179,"kkkk6":123,"kkkk60":18,"kkkk63":130,"kkkk68":81,"kkkk71":193,"kkkk73":32,"kkkk76":144,"kkkk81":95,"kkkk86":46,"kkkk89":158,"kkkk94":109,"kkkk99":60,"kkkkk100":131,"kkkkk12":145,"kkkkk17":96,"kkkkk22":47,"kkkkk25":159,"kkkkk30":110,"kkkkk35":61,"kkkkk38":173,"kkkkk4":82,"kkkkk40":12,"kkkkk43":124,"kkkkk48":75,"kkkkk51":187,"kkkkk53":26,"kkkkk56":138,"kkkkk61":89,"kkkkk66":40,"kkkkk69":152,"kkkkk7":194,"kkkkk74":103,"kkkkk79":54,"kkkkk82":166,"kkkkk84":5,"kkkkk87":117,"kkkkk9":33,"kkkkk92":68,"kkkkk95":180,"kkkkk97":19,"kkkkkk10":104,"kkkkkk15":55,"kkkkkk18":167,"kkkkkk2":41,"kkkkkk20":6,"kkkkkk23":118,"kkkkkk28":69,"kkkkkk31":181,"kkkkkk33":20,"kkkkkk36":132,"kkkkkk41":83,"kkkkkk44":195,"kkkkkk46":34,"kkkkkk49":146,"kkkkkk5":153,"kkkkkk54":97,"kkkkkk59":48,"kkkkkk62":160,"kkkkkk67":111,"kkkkkk72":62,"kkkkkk75":174,"kkkkkk77":13,"kkkkkk80":125,"kkkkkk85":76,"kkkkkk88":188,"kkkkkk90":27,"kkkkkk93":139,"kkkkkk98":90}
// map key with special chars: {"\t":3,"\u003c\u003e":4,"a\"b":1,"c\\d":2}
// map[string]map: {"a":{"1":2,"2":1},"b":null}
// slice of maps: [{"a":1},null,{}]
