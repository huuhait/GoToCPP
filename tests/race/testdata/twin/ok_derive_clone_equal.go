package main

import (
	"fmt"
	"sync"
)

type Config struct {
	Name  string
	Ports []int
	Env   map[string]string
	Next  *Config
}

func main() {
	shared := Config{Name: "base", Ports: []int{1, 2, 3}, Env: map[string]string{"k": "v"}, Next: &Config{Name: "next", Ports: []int{9}}}
	var wg sync.WaitGroup
	var mu sync.Mutex
	same, total := 0, 0
	for i := range 8 {
		wg.Go(func() {
			for range 50 {
				c := shared.Clone()
				c.Ports[0] = i
				c.Env["own"] = "x"
				c.Next.Ports[0]++
				ok := shared.Equal(shared) && !shared.Equal(c)
				mu.Lock()
				total++
				if ok {
					same++
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	fmt.Println(same, total, shared.Ports, shared.Env, shared.Next.Ports)
}

func (c Config) Equal(o Config) bool {
	if c.Name != o.Name || len(c.Ports) != len(o.Ports) || len(c.Env) != len(o.Env) {
		return false
	}
	for i := range c.Ports {
		if c.Ports[i] != o.Ports[i] {
			return false
		}
	}
	for k, v := range c.Env {
		if ov, ok := o.Env[k]; !ok || ov != v {
			return false
		}
	}
	if c.Next != o.Next {
		if c.Next == nil || o.Next == nil || !c.Next.Equal(*o.Next) {
			return false
		}
	}
	return true
}

func (c Config) Clone() Config {
	out := c
	if c.Ports != nil {
		out.Ports = make([]int, len(c.Ports))
		copy(out.Ports, c.Ports)
	}
	if c.Env != nil {
		out.Env = make(map[string]string, len(c.Env))
		for k, v := range c.Env {
			out.Env[k] = v
		}
	}
	if c.Next != nil {
		n := c.Next.Clone()
		out.Next = &n
	}
	return out
}
