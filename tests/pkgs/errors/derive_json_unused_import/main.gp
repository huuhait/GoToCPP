// phase: 09

// The import of encoding/json that #Derive(JSON) needs is not reported as unused in the file of the type, and only there: another file of the
// package that imports it and does not use it still gets the error.

package main

import "encoding/json"

#Derive(JSON)
type Cfg struct {
	Name string `json:"name"`
}

func main() {
	var c Cfg
	_ = c
}
