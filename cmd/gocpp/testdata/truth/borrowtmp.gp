package main

type T struct{ n int }

func bump(p *T) { p.n++ }

func main() {
	bump(&T{})
}
