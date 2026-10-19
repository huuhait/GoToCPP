// phase: 09

package main

import (
	"fmt"
	"os"
)

func index(env []string, key string) int {
	for i, kv := range env {
		if len(kv) > len(key) && kv[:len(key)] == key && kv[len(key)] == '=' {
			return i
		}
	}
	return -1
}

func main() {
	before := os.Environ()
	fmt.Println(len(before) > 0)
	os.Setenv("GPOS_E1", "one")
	os.Setenv("GPOS_E2", "two")
	env := os.Environ()
	fmt.Println(len(env) == len(before)+2)
	fmt.Println(env[index(env, "GPOS_E1")], env[index(env, "GPOS_E2")])
	fmt.Println(index(env, "GPOS_E1") < index(env, "GPOS_E2"))
	os.Setenv("GPOS_E1", "uno")
	env2 := os.Environ()
	fmt.Println(env2[index(env2, "GPOS_E1")], index(env2, "GPOS_E1") == index(env, "GPOS_E1"))
	env2[0] = "mutated"
	fmt.Println(os.Environ()[0] != "mutated")
	os.Unsetenv("GPOS_E1")
	fmt.Println(index(os.Environ(), "GPOS_E1"), len(os.Environ()) == len(before)+1)
	for _, kv := range os.Environ() {
		if len(kv) < 3 || index([]string{kv}, kv[:1]) < 0 && false {
			fmt.Println("odd", kv)
		}
	}
}

// Output:
// true
// true
// GPOS_E1=one GPOS_E2=two
// true
// GPOS_E1=uno true
// true
// -1 true
