package probe

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// The streaming reader of clang's filtered AST dump.
//
// `clang++ -Xclang -ast-dump=json -Xclang -ast-dump-filter=gp_probe_` prints
// one JSON object per matching declaration, one after the other (not a JSON
// array). Without the filter the dump is one object of hundreds of
// megabytes, so the reader never holds more than one top-level object, and
// not even that when the object is larger than the limit: ScanDecls finds the
// end of an object with a small state machine over a fixed buffer, hands the
// object's bytes to the caller (when they fit the limit), and drops the
// bytes of an object that doesn't fit, keeping only its name. The memory is
// the buffer plus the limit, whatever the dump's size.

// DefaultMaxDecl is the largest top-level object that ScanDecls hands over.
// The probe declarations of the real packages are a few kilobytes each (the
// constant probes, with their cast trees, are the largest).
const DefaultMaxDecl = 8 << 20

// A RawDecl is one top-level object of the dump.
type RawDecl struct {
	// Name is the "name" of the declaration ("gp_probe_3_c"), read from the
	// start of the object: it is there even when the object was too large.
	Name string

	// Size is the number of bytes of the object.
	Size int64

	// TooLarge says that the object is larger than the limit; Raw is nil.
	TooLarge bool

	// Head is the first bytes of the object (at most 1 KB), also when it is too
	// large: its kind and id. Valid only until fn returns.
	Head []byte

	// Raw is the JSON text of the object. It is valid only until fn returns:
	// the scanner reuses its buffer.
	Raw []byte
}

// ErrBadDump is the error for output that is not a stream of JSON objects.
var ErrBadDump = errors.New("the compiler's AST dump is not a stream of JSON objects")

// prefixLen is how much of an object's start is kept to find its name.
const prefixLen = 16 << 10

// ScanDecls reads the JSON objects of r and calls fn for each in order. maxDecl
// (DefaultMaxDecl when it is 0) limits the size of an object that is
// kept. It returns the number of objects and the first error: a read error,
// an error of fn (the scan stops), or ErrBadDump (text that is not an object,
// or an object that doesn't end). The text before the first '{' of the
// stream, if it is only white space, is fine; so is a truncated stream with no
// bytes after the last complete object.
func ScanDecls(r io.Reader, maxDecl int, fn func(RawDecl) error) (int, error) {
	if maxDecl <= 0 {
		maxDecl = DefaultMaxDecl
	}
	s := &scanner{r: r, buf: make([]byte, 64<<10)}
	n := 0
	var obj []byte // the object so far, up to maxDecl bytes
	prefix := make([]byte, 0, prefixLen)
	for {
		// Skip the white space between objects.
		c, err := s.skipSpace()
		if err == io.EOF {
			return n, nil
		}
		if err != nil {
			return n, err
		}
		if c != '{' {
			return n, fmt.Errorf("%w: unexpected %q", ErrBadDump, c)
		}
		obj = obj[:0]
		var size int64
		prefix = prefix[:0]
		depth, inStr, esc := 0, false, false
		tooLarge := false
		done := false
		for !done {
			if s.pos == s.end {
				if err := s.fill(); err != nil {
					if err == io.EOF {
						return n, fmt.Errorf("%w: an object doesn't end", ErrBadDump)
					}
					return n, err
				}
			}
			chunk := s.buf[s.pos:s.end]
			used := len(chunk)
			for i, b := range chunk {
				if inStr {
					switch {
					case esc:
						esc = false
					case b == '\\':
						esc = true
					case b == '"':
						inStr = false
					}
					continue
				}
				switch b {
				case '"':
					inStr = true
				case '{', '[':
					depth++
				case '}', ']':
					depth--
					if depth == 0 {
						used = i + 1
						done = true
					}
				}
				if done {
					break
				}
			}
			seg := chunk[:used]
			size += int64(len(seg))
			if len(prefix) < prefixLen {
				prefix = append(prefix, seg[:min(len(seg), prefixLen-len(prefix))]...)
			}
			if !tooLarge {
				if len(obj)+len(seg) > maxDecl {
					tooLarge = true
					obj = obj[:0]
				} else {
					obj = append(obj, seg...)
				}
			}
			s.pos += used
		}
		d := RawDecl{Size: size, TooLarge: tooLarge, Name: declName(prefix), Head: prefix[:min(len(prefix), 1024)]}
		if !tooLarge {
			d.Raw = obj
		}
		n++
		if err := fn(d); err != nil {
			return n, err
		}
	}
}

// declName returns the "name" of the object whose first bytes are prefix: the
// key of the top-level object itself (depth 1), not a "name" inside a nested
// object or in a string.
func declName(prefix []byte) string {
	depth := 0
	for i := 0; i < len(prefix); i++ {
		switch c := prefix[i]; c {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		case '"':
			j := i + 1
			for j < len(prefix) && prefix[j] != '"' {
				if prefix[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(prefix) {
				return ""
			}
			key := string(prefix[i+1 : j])
			i = j
			if depth != 1 || key != "name" {
				continue
			}
			k := j + 1
			for k < len(prefix) && (prefix[k] == ' ' || prefix[k] == '\n' || prefix[k] == '\t' || prefix[k] == '\r') {
				k++
			}
			if k >= len(prefix) || prefix[k] != ':' {
				continue // a string value that says "name"
			}
			k++
			for k < len(prefix) && (prefix[k] == ' ' || prefix[k] == '\n' || prefix[k] == '\t' || prefix[k] == '\r') {
				k++
			}
			if k >= len(prefix) || prefix[k] != '"' {
				return ""
			}
			e := k + 1
			for e < len(prefix) && prefix[e] != '"' {
				if prefix[e] == '\\' {
					e++
				}
				e++
			}
			if e >= len(prefix) {
				return ""
			}
			var name string
			if json.Unmarshal(prefix[k:e+1], &name) != nil {
				return ""
			}
			return name
		}
	}
	return ""
}

// A scanner reads r through a fixed buffer.
type scanner struct {
	r        io.Reader
	buf      []byte
	pos, end int
}

// fill reads more bytes into the empty buffer.
func (s *scanner) fill() error {
	for {
		n, err := s.r.Read(s.buf)
		s.pos, s.end = 0, n
		if n > 0 {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// skipSpace skips white space and returns the next byte without consuming
// it, or io.EOF.
func (s *scanner) skipSpace() (byte, error) {
	for {
		if s.pos == s.end {
			if err := s.fill(); err != nil {
				return 0, err
			}
		}
		switch c := s.buf[s.pos]; c {
		case ' ', '\t', '\n', '\r':
			s.pos++
		default:
			return c, nil
		}
	}
}
