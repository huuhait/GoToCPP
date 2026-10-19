package deep

import "fmt"

func Where() string { return fmt.Sprint(#file(), " ", #func()) }
