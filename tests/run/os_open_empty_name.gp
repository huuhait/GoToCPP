// phase: 09

package main

import (
	"fmt"
	"os"
)

func main() {
	_, err := os.Open("")
	fmt.Println(err)
	_, err = os.Create("")
	fmt.Println(err)
	_, err = os.Stat("")
	fmt.Println(err)
	_, err = os.ReadFile("")
	fmt.Println(err)
	fmt.Println(os.Remove(""))
	fmt.Println(os.Mkdir("", 0o755))
	fmt.Println(os.Chdir(""))
	_, err = os.ReadDir("")
	fmt.Println(err)
}

// Output:
// open : no such file or directory
// open : no such file or directory
// stat : no such file or directory
// open : no such file or directory
// remove : no such file or directory
// mkdir : no such file or directory
// chdir : no such file or directory
// open : no such file or directory
