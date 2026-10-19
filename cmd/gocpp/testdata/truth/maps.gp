package main

func key() string { return "k" }

func main() {
	ok := map[string]int{}
	ok[key()] = 5
	ok["a"] = 1
	ok["a"]++
	ok["a"] += 2
	ok["a"] /= 2
	d := len(ok)
	ok["a"] /= d
	ok["b"] %= d
	v := ok["a"]
	w, found := ok["b"]
	println(v, w, found)
}
