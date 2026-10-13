// What the user writes (gocpp syntax):
//
//	#util.Trace()
//	func loadConfig(path string) (string, error) {
//		#util.Assert(path != "", "path is empty")
//		return "port=8080", nil
//	}
//
//	#util.Locked(&s.mu)
//	func (s *Store) Put(k string, v int) { s.m[k] = v }
//
// What the compiler generates (plain Go, checked by stock go/types):
package main

import (
	"fmt"
	"sync"

	"spike/macro"
	"spike/util"
)

var (
	gp_site_1 = macro.Site{File: "main.gp", Line: 3, Func: "main.loadConfig"}
	gp_site_2 = macro.Site{File: "main.gp", Line: 5, Func: "main.loadConfig", Args: []string{`path != ""`, `"path is empty"`}}
	gp_site_3 = macro.Site{File: "main.gp", Line: 9, Func: "main.(*Store).Put", Args: []string{"&s.mu"}}
)

func loadConfig(path string) (string, error) {
	defer util.Trace(&gp_site_1)() // macro on top of the func: inserted as the first statement
	util.Assert(&gp_site_2, path != "", func() string { return "path is empty" })
	return "port=8080", nil
}

type Store struct {
	mu sync.Mutex
	m  map[string]int
}

func (s *Store) Put(k string, v int) {
	defer util.Locked(&gp_site_3, &s.mu)() // lock on entry, unlock on exit
	s.m[k] = v
}

func main() {
	cfg, _ := loadConfig("app.json")
	fmt.Println("config:", cfg)

	s := &Store{m: map[string]int{}}
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func() { defer wg.Done(); s.Put(fmt.Sprint(i%10), i) }()
	}
	wg.Wait()
	fmt.Println("store size:", len(s.m))

	defer func() { fmt.Println("recovered:", recover()) }()
	loadConfig("")
}
