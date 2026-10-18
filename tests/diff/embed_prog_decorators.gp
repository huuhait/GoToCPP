// phase: 05

// Writer decorators: embedded interfaces, overriding one method, a nil embedded interface, interface assertions on the decorators.
package main

import "fmt"

type Writer interface{ Write(p []byte) (int, error) }
type Closer interface{ Close() error }
type WriteCloser interface {
	Writer
	Closer
}

type buf struct {
	data   []byte
	closed bool
}

func (b *buf) Write(p []byte) (int, error) {
	if b.closed {
		return 0, fmt.Errorf("write to closed buffer")
	}
	b.data = append(b.data, p...)
	return len(p), nil
}
func (b *buf) Close() error {
	if b.closed {
		return fmt.Errorf("already closed")
	}
	b.closed = true
	return nil
}

type upper struct{ Writer }

func (u upper) Write(p []byte) (int, error) {
	q := make([]byte, len(p))
	for i, c := range p {
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		q[i] = c
	}
	n, err := u.Writer.Write(q)
	return n, err
}

type counting struct {
	WriteCloser
	n int
}

func (c *counting) Write(p []byte) (int, error) {
	n, err := c.WriteCloser.Write(p)
	c.n += n
	return n, err
}

type limited struct {
	Writer
	left int
}

func (l *limited) Write(p []byte) (int, error) {
	if len(p) > l.left {
		p = p[:l.left]
	}
	n, err := l.Writer.Write(p)
	l.left -= n
	if err == nil && n < len(p) {
		err = fmt.Errorf("short write")
	}
	return n, err
}

type multi struct{ ws []Writer }

func (m multi) Write(p []byte) (n int, err error) {
	for _, w := range m.ws {
		if n, err = w.Write(p); err != nil {
			return
		}
	}
	return len(p), nil
}

func fprint(w Writer, a ...any) {
	n, err := w.Write([]byte(fmt.Sprint(a...)))
	if err != nil {
		fmt.Println("error:", err, n)
	}
}

func main() {
	b := &buf{}
	c := &counting{WriteCloser: b}
	u := upper{c}
	l := &limited{u, 8}
	m := multi{[]Writer{l, upper{upper{b}}}}
	fprint(m, "hello ", "world")
	fprint(l, "xyz")
	fmt.Printf("%q %d %d\n", b.data, c.n, l.left)
	var wc WriteCloser = c
	fmt.Println(wc.Close(), wc.Close())
	fprint(c, "late")
	var w Writer = u
	if _, ok := w.(Closer); ok {
		fmt.Println("upper is a Closer")
	}
	if cl, ok := Writer(c).(Closer); ok {
		fmt.Println("counting closer:", cl.Close())
	}
	var wr Writer = &limited{}
	_, isC := wr.(Closer)
	fmt.Println(isC)
	defer func() { fmt.Println("recovered:", recover()) }()
	var up upper
	up.Write([]byte("x"))
}
