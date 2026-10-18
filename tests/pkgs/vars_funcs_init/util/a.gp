package util

import (
	"fmt"

	"example.com/vars/cfg"
)

var A = B + cfg.Size

func init() { fmt.Println("util.init a", A, B, CC) }

func Apply(h cfg.Handler, s string) string { return h(s) }

type Counter struct{ n int }

func (c *Counter) Inc() int { c.n++; return c.n }
func (c Counter) Get() int  { return c.n }

func NewCounter() *Counter { return &Counter{} }

