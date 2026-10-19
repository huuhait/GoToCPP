// phase: 09
// Go 1.25's flag examples (example_test.go, example_flagset_test.go, example_func_test.go, example_textvar_test.go, example_value_test.go) in order. net.IP and net/url.URL are not in gocpp: ip and URL below are small stand-ins with the same methods and error texts. Expected output from real Go.

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ---- stand-ins for net.IP (IPv4 only) and net/url.URL ----

type IP []byte

func ipv4(a, b, c, d byte) IP { return IP{a, b, c, d} }

func parseIP(s string) IP {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return nil
	}
	r := make(IP, 4)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 255 || (len(p) > 1 && p[0] == '0') {
			return nil
		}
		r[i] = byte(n)
	}
	return r
}

func (a IP) String() string {
	if len(a) == 0 {
		return "<nil>"
	}
	return fmt.Sprintf("%d.%d.%d.%d", a[0], a[1], a[2], a[3])
}

func (a IP) IsLoopback() bool { return len(a) == 4 && a[0] == 127 }

func (a IP) MarshalText() ([]byte, error) {
	if len(a) == 0 {
		return []byte(""), nil
	}
	return []byte(a.String()), nil
}

func (a *IP) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*a = nil
		return nil
	}
	x := parseIP(string(text))
	if x == nil {
		return errors.New("invalid IP address: " + string(text))
	}
	*a = x
	return nil
}

type URL struct{ Scheme, Host, Path string }

func parseURL(s string) (*URL, error) {
	scheme, rest, ok := strings.Cut(s, "://")
	if !ok {
		return nil, errors.New("missing scheme")
	}
	host, path, _ := strings.Cut(rest, "/")
	if path != "" {
		path = "/" + path
	}
	return &URL{scheme, host, path}, nil
}

func (u *URL) String() string { return u.Scheme + "://" + u.Host + u.Path }

// ---- example_test.go ----

var species = flag.String("species", "gopher", "the species we are studying")

var gopherType string

func init() {
	const (
		defaultGopher = "pocket"
		usage         = "the variety of gopher"
	)
	flag.StringVar(&gopherType, "gopher_type", defaultGopher, usage)
	flag.StringVar(&gopherType, "g", defaultGopher, usage+" (shorthand)")
}

type interval []time.Duration

func (i *interval) String() string {
	return fmt.Sprint(*i)
}

func (i *interval) Set(value string) error {
	// If we wanted to allow the flag to be set multiple times,
	// accumulating values, we would delete this if statement.
	if len(*i) > 0 {
		return errors.New("interval flag already set")
	}
	for _, dt := range strings.Split(value, ",") {
		duration, err := time.ParseDuration(dt)
		if err != nil {
			return err
		}
		*i = append(*i, duration)
	}
	return nil
}

var intervalFlag interval

func init() {
	// Tie the command-line flag to the intervalFlag variable and
	// set a usage message.
	flag.Var(&intervalFlag, "deltaT", "comma-separated list of intervals to use between events")
}

func Example() {
	// What the package-level variables above registered on the command line.
	os.Args[0] = "prog"
	flag.CommandLine.Init("prog", flag.ContinueOnError) // not ExitOnError, which would end the program
	flag.CommandLine.SetOutput(os.Stdout)
	flag.PrintDefaults()
	flag.CommandLine.Parse([]string{"-species=wombat", "-g", "big", "-deltaT", "1s,2m", "x"})
	fmt.Println(*species, gopherType, intervalFlag, flag.Args())
	fmt.Println(flag.CommandLine.Parse([]string{"-deltaT", "3s"}))
}

// ---- example_flagset_test.go ----

func ExampleFlagSet() {
	start := func(args []string) {
		// A real program (not an example) would use flag.ExitOnError.
		fs := flag.NewFlagSet("start", flag.ContinueOnError)
		fs.SetOutput(os.Stdout) // the examples of Go print the errors to standard error, which a test does not compare
		addr := fs.String("addr", ":8080", "`address` to listen on")
		if err := fs.Parse(args); err != nil {
			fmt.Printf("error: %s", err)
			return
		}
		fmt.Printf("starting server on %s\n", *addr)
	}

	stop := func(args []string) {
		fs := flag.NewFlagSet("stop", flag.ContinueOnError)
		fs.SetOutput(os.Stdout)
		timeout := fs.Duration("timeout", time.Second, "stop timeout duration")
		if err := fs.Parse(args); err != nil {
			fmt.Printf("error: %s", err)
			return
		}
		fmt.Printf("stopping server (timeout=%v)\n", *timeout)
	}

	main := func(args []string) {
		subArgs := args[2:] // Drop program name and command.
		switch args[1] {
		case "start":
			start(subArgs)
		case "stop":
			stop(subArgs)
		default:
			fmt.Printf("error: unknown command - %q\n", args[1])
			// In a real program (not an example) print to os.Stderr and exit the program with non-zero value.
		}
	}

	main([]string{"httpd", "start", "-addr", ":9999"})
	main([]string{"httpd", "stop"})
	main([]string{"http", "start", "-log-level", "verbose"})
	fmt.Println()
}

// ---- example_func_test.go ----

func ExampleFunc() {
	fs := flag.NewFlagSet("ExampleFunc", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	var ip IP
	fs.Func("ip", "`IP address` to parse", func(s string) error {
		ip = parseIP(s)
		if ip == nil {
			return errors.New("could not parse IP")
		}
		return nil
	})

	fs.Parse([]string{"-ip", "127.0.0.1"})
	fmt.Printf("{ip: %v, loopback: %t}\n\n", ip, ip.IsLoopback())

	// 256 is not a valid IPv4 component
	fs.Parse([]string{"-ip", "256.0.0.1"})
	fmt.Printf("{ip: %v, loopback: %t}\n\n", ip, ip.IsLoopback())
}

func ExampleBoolFunc() {
	fs := flag.NewFlagSet("ExampleBoolFunc", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	fs.BoolFunc("log", "logs a dummy message", func(s string) error {
		fmt.Println("dummy message:", s)
		return nil
	})
	fs.Parse([]string{"-log"})
	fs.Parse([]string{"-log=0"})
}

// ---- example_textvar_test.go ----

func ExampleTextVar() {
	fs := flag.NewFlagSet("ExampleTextVar", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	var ip IP
	fs.TextVar(&ip, "ip", ipv4(192, 168, 0, 100), "`IP address` to parse")
	fs.Parse([]string{"-ip", "127.0.0.1"})
	fmt.Printf("{ip: %v}\n\n", ip)

	// 256 is not a valid IPv4 component
	ip = nil
	fs.Parse([]string{"-ip", "256.0.0.1"})
	fmt.Printf("{ip: %v}\n\n", ip)
}

// ---- example_value_test.go ----

type URLValue struct {
	URL *URL
}

func (v URLValue) String() string {
	if v.URL != nil {
		return v.URL.String()
	}
	return ""
}

func (v URLValue) Set(s string) error {
	if u, err := parseURL(s); err != nil {
		return err
	} else {
		*v.URL = *u
	}
	return nil
}

var u = &URL{}

func ExampleValue() {
	fs := flag.NewFlagSet("ExampleValue", flag.ExitOnError)
	fs.Var(&URLValue{u}, "url", "URL to parse")

	fs.Parse([]string{"-url", "https://golang.org/pkg/flag/"})
	fmt.Printf(`{scheme: %q, host: %q, path: %q}`, u.Scheme, u.Host, u.Path)
	fmt.Println()
}

func main() {
	for _, e := range []struct {
		name string
		f    func()
	}{
		{"Example", Example},
		{"ExampleFlagSet", ExampleFlagSet},
		{"ExampleFunc", ExampleFunc},
		{"ExampleBoolFunc", ExampleBoolFunc},
		{"ExampleTextVar", ExampleTextVar},
		{"ExampleValue", ExampleValue},
	} {
		fmt.Println("=== " + e.name)
		e.f()
	}
}

// Output:
// === Example
//   -deltaT value
//     	comma-separated list of intervals to use between events
//   -g string
//     	the variety of gopher (shorthand) (default "pocket")
//   -gopher_type string
//     	the variety of gopher (default "pocket")
//   -species string
//     	the species we are studying (default "gopher")
// wombat big [1s 2m0s] [x]
// invalid value "3s" for flag -deltaT: interval flag already set
// Usage of prog:
//   -deltaT value
//     	comma-separated list of intervals to use between events
//   -g string
//     	the variety of gopher (shorthand) (default "pocket")
//   -gopher_type string
//     	the variety of gopher (default "pocket")
//   -species string
//     	the species we are studying (default "gopher")
// invalid value "3s" for flag -deltaT: interval flag already set
// === ExampleFlagSet
// starting server on :9999
// stopping server (timeout=1s)
// flag provided but not defined: -log-level
// Usage of start:
//   -addr address
//     	address to listen on (default ":8080")
// error: flag provided but not defined: -log-level
// === ExampleFunc
// {ip: 127.0.0.1, loopback: true}
//
// invalid value "256.0.0.1" for flag -ip: could not parse IP
// Usage of ExampleFunc:
//   -ip IP address
//     	IP address to parse
// {ip: <nil>, loopback: false}
//
// === ExampleBoolFunc
// dummy message: true
// dummy message: 0
// === ExampleTextVar
// {ip: 127.0.0.1}
//
// invalid value "256.0.0.1" for flag -ip: invalid IP address: 256.0.0.1
// Usage of ExampleTextVar:
//   -ip IP address
//     	IP address to parse (default 192.168.0.100)
// {ip: <nil>}
//
// === ExampleValue
// {scheme: "https", host: "golang.org", path: "/pkg/flag/"}
