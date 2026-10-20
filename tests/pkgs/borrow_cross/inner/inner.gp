package inner

// Counter is embedded by lib.Wrapper and main's types; its pointer methods borrow their receiver
// (they only read and write through it); Keep stores it and does not.
type Counter struct {
	n    int
}

func (c *Counter) Inc() int       { c.n++; return c.n }
func (c *Counter) Add(k int)      { c.n += k }
func (c Counter) Get() int        { return c.n }
func (c *Counter) Keep() *Counter { Last = c; return c }
func (c *Counter) reset()         { c.n = 0 }
func (c *Counter) Reset()         { c.reset() }

var Last *Counter

// Remember stores its argument: an owning parameter. Peek only reads it: a borrowed one.
func Remember(c *Counter) { Last = c }
func Peek(c *Counter) int  { return c.n }

// Drop clears the package variable that may hold its argument, then uses the argument.
func Drop(c *Counter) int {
	Last = nil
	c.n += 100
	return c.n
}
