// phase: 08
package main

import (
	"./arguments.hpp"
	"example.com/borrowargs/lib"
	"fmt"
)

type T struct {
	n   int
	pad [8]int
}

type Foreign struct{ value arguments.Copy }

var pointer *T
var text string
var foreign Foreign
var conversion arguments.Convert

func read(p *T, s string, n int) int            { return p.n + len(s) + n }
func (p *T) read(s string, n int) int           { return p.n + len(s) + n }
func foreignRead(p *T, s string, _ Foreign) int { return p.n + len(s) }

func setup() {
	pointer = &T{n: 7}
	text = "before"
	arguments.set(func() {
		pointer = nil
		text = "changed"
	})
}

func caseSlice() {
	setup()
	result := read(pointer, text, len([]Foreign{foreign}))
	fmt.Println("slice", result, pointer == nil, text)
}

func caseConversion() {
	setup()
	result := read(pointer, text, int(conversion))
	fmt.Println("conversion", result, pointer == nil, text)
}

func caseAggregateConversion() {
	setup()
	result := read(pointer, text, len([]Foreign{Foreign(foreign)}))
	fmt.Println("aggregate", result, pointer == nil, text)
}

func caseArgument() {
	setup()
	result := foreignRead(pointer, text, foreign)
	fmt.Println("argument", result, pointer == nil, text)
}

func caseMethod() {
	setup()
	result := pointer.read(text, len([]Foreign{foreign}))
	fmt.Println("method", result, pointer == nil, text)
}

func caseFuncValue() {
	setup()
	call := read
	result := call(pointer, text, len([]Foreign{foreign}))
	fmt.Println("func", result, pointer == nil, text)
}

func change() int {
	pointer.n = 9
	text = "after"
	return 0
}

func caseExplicitOrder() {
	setup()
	result := read(pointer, text, len([]Foreign{foreign})+change())
	fmt.Println("explicit", result, pointer == nil, text)
}

func show(s string, _ Foreign) {
	fmt.Println("defer", s)
}

func caseDefer() {
	setup()
	defer show(text, foreign)
	fmt.Println("defer source", text)
}

func showCount(s string, n int) {
	fmt.Println("defer count", s, n)
}

func caseDeferCount() {
	setup()
	defer showCount(text, len([]Foreign{foreign}))
	fmt.Println("defer count source", text)
}

func send(s string, _ Foreign, done chan string) {
	done <- s
}

func caseGo() {
	setup()
	done := make(chan string, 1)
	go send(text, foreign, done)
	got := <-done
	fmt.Println("go", got, text)
}

func sendCount(s string, n int, done chan string) {
	if n != 1 {
		panic("wrong collected count")
	}
	done <- s
}

func caseGoCount() {
	setup()
	done := make(chan string, 1)
	go sendCount(text, len([]Foreign{foreign}), done)
	got := <-done
	fmt.Println("go count", got, text)
}

func caseMapLiteral() {
	setup()
	result := read(pointer, text, len(map[int]Foreign{0: foreign}))
	fmt.Println("map literal", result, pointer == nil, text)
}

func caseSliceView() {
	setup()
	result := read(pointer, text, len([]Foreign{foreign}[:][:]))
	fmt.Println("slice view", result, pointer == nil, text)
}

func caseAppend() {
	setup()
	result := read(pointer, text, len(append([]Foreign{}, foreign)))
	fmt.Println("append", result, pointer == nil, text)
}

func caseMapKey() {
	setup()
	result := read(pointer, text, len(map[int]Foreign{change(): foreign}))
	fmt.Println("map key", result, pointer == nil, text)
}

func caseSliceBound() {
	setup()
	result := read(pointer, text, len([]Foreign{foreign}[:change()+1]))
	fmt.Println("slice bound", result, pointer == nil, text)
}

func caseMapSiblingCall() {
	setup()
	result := read(pointer, text, len(map[int]Foreign{0: foreign})+change())
	fmt.Println("map sibling", result, pointer == nil, text)
}

var qualifiedPointer *lib.Node

func setupQualified() {
	qualifiedPointer = &lib.Node{Value: 7}
	text = "before"
	arguments.set(func() {
		qualifiedPointer = nil
		text = "changed"
	})
}

func caseQualified() {
	setupQualified()
	result := lib.Read(qualifiedPointer, text, len([]Foreign{foreign}))
	fmt.Println("qualified", result, qualifiedPointer == nil, text)
}

func caseQualifiedDefer() {
	setupQualified()
	defer lib.Show(text, len([]Foreign{foreign}))
	fmt.Println("qualified defer source", text)
}

func caseQualifiedGo() {
	setupQualified()
	done := make(chan string, 1)
	go lib.Send(text, len([]Foreign{foreign}), done)
	got := <-done
	fmt.Println("qualified go", got, text)
}

var checkValues = []int{1}
var checkKeys = [1]int{0}
var checkMap = map[int]int{0: 2}
var checkCalls int
var checkReceivers []*T

func gather(value int, count int) int { return value + count }

func checkedArgument(i, j int) int {
	return gather(checkValues[i]+checkMap[checkKeys[j]], len([]Foreign{foreign}))
}

func checkedNil(index *int, i int) int {
	return gather(checkValues[i]+checkMap[*index], len([]Foreign{foreign}))
}

func nextCheck() int {
	checkCalls++
	return 0
}

func checkedCall(j int) int {
	return gather(checkValues[nextCheck()]+checkMap[checkKeys[j]], len([]Foreign{foreign}))
}

func checkedReceiver(i, j int) int {
	return checkReceivers[checkValues[i]+checkMap[checkKeys[j]]].read(
		text,
		len([]Foreign{foreign}),
	)
}

func caseArgumentChecks() {
	setup()
	result := checkedArgument(0, 0)
	fmt.Println("argument checks", result, text)
}

func caseNilChecks() {
	setup()
	key := 0
	result := checkedNil(&key, 0)
	fmt.Println("nil checks", result, text)
}

func caseCheckedCall() {
	setup()
	checkCalls = 0
	result := checkedCall(0)
	fmt.Println("checked call", result, checkCalls, text)
}

func caseReceiverChecks() {
	setup()
	checkReceivers = []*T{nil, nil, nil, pointer}
	result := checkedReceiver(0, 0)
	fmt.Println("receiver checks", result, text)
}

func main() {
	caseSlice()
	caseConversion()
	caseAggregateConversion()
	caseArgument()
	caseMethod()
	caseFuncValue()
	caseExplicitOrder()
	caseDefer()
	caseDeferCount()
	caseGo()
	caseGoCount()
	caseQualified()
	caseQualifiedDefer()
	caseQualifiedGo()
	caseMapLiteral()
	caseSliceView()
	caseAppend()
	caseMapKey()
	caseSliceBound()
	caseMapSiblingCall()
	caseArgumentChecks()
	caseNilChecks()
	caseCheckedCall()
	caseReceiverChecks()
}

// Output:
// slice 14 true changed
// conversion 14 true changed
// aggregate 14 true changed
// argument 13 true changed
// method 14 true changed
// func 14 true changed
// explicit 15 true changed
// defer source changed
// defer before
// defer count source changed
// defer count before 1
// go before changed
// go count before changed
// qualified 14 true changed
// qualified defer source changed
// qualified defer before 1
// qualified go before changed
// map literal 14 true changed
// slice view 14 true changed
// append 14 true changed
// map key 15 true changed
// slice bound 15 true changed
// map sibling 15 true changed
// argument checks 4 changed
// nil checks 4 changed
// checked call 4 1 changed
// receiver checks 14 changed
