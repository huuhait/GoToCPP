// phase: 05
// lsan: cycle

// Closures behind interfaces: counters, functional options, hooks, composition and memoization. The recursive
// memoized fib is a closure that holds itself through a variable: a reference cycle (divergence D1), so a
// sanitizer run turns LeakSanitizer off for this test.
package main

import "fmt"

type Counter interface {
	Inc() int
	Reset()
}

type closureCounter struct {
	inc   func() int
	reset func()
}

func (c closureCounter) Inc() int { return c.inc() }
func (c closureCounter) Reset()   { c.reset() }

func NewCounter(step int) Counter {
	n := 0
	return closureCounter{
		inc:   func() int { n += step; return n },
		reset: func() { n = 0 },
	}
}

type Option func(*Server)
type Server struct {
	name  string
	port  int
	hooks []func(string) string
	log   Logger
}
type Logger interface{ Logf(format string, args ...any) }
type prefixLogger struct {
	prefix string
	lines  *[]string
}

func (p prefixLogger) Logf(format string, args ...any) {
	*p.lines = append(*p.lines, p.prefix+fmt.Sprintf(format, args...))
}

func WithPort(p int) Option         { return func(s *Server) { s.port = p } }
func WithHook(h func(string) string) Option {
	return func(s *Server) { s.hooks = append(s.hooks, h) }
}
func WithLogger(l Logger) Option { return func(s *Server) { s.log = l } }

func New(name string, opts ...Option) *Server {
	s := &Server{name: name, port: 80}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *Server) Handle(req string) string {
	if s.log != nil {
		s.log.Logf("%s handling %q on %d", s.name, req, s.port)
	}
	for _, h := range s.hooks {
		req = h(req)
	}
	return req
}

func compose(fs ...func(int) int) func(int) int {
	return func(x int) int {
		for i := len(fs) - 1; i >= 0; i-- {
			x = fs[i](x)
		}
		return x
	}
}

func memo(f func(int) int) func(int) int {
	cache := map[int]int{}
	return func(n int) int {
		if v, ok := cache[n]; ok {
			return v
		}
		v := f(n)
		cache[n] = v
		return v
	}
}

func main() {
	c1, c2 := NewCounter(1), NewCounter(10)
	fmt.Println(c1.Inc(), c1.Inc(), c2.Inc(), c1.Inc(), c2.Inc())
	c1.Reset()
	fmt.Println(c1.Inc(), c2.Inc())
	inc := c1.Inc
	reset := Counter.Reset
	reset(c1)
	fmt.Println(inc(), inc())
	var lines []string
	srv := New("srv", WithPort(8080), WithLogger(prefixLogger{"[log] ", &lines}),
		WithHook(func(s string) string { return s + "!" }),
		WithHook(func(s string) string { return "<" + s + ">" }))
	fmt.Println(srv.Handle("hi"), srv.Handle("yo"))
	fmt.Println(lines)
	fmt.Println(New("plain").Handle("x"))
	double := func(x int) int { return 2 * x }
	plus3 := func(x int) int { return x + 3 }
	fmt.Println(compose(double, plus3)(4), compose()(4), compose(plus3, double, plus3)(1))
	var fib func(int) int
	calls := 0
	fib = memo(func(n int) int {
		calls++
		if n < 2 {
			return n
		}
		return fib(n-1) + fib(n-2)
	})
	fmt.Println(fib(30), calls)
}
