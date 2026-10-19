package bindgen

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

// A rawDecl is one child of the TranslationUnitDecl of clang's JSON dump.
//
// The dump of a header that includes the C library is hundreds of megabytes
// (the SDL umbrella header: 446 MB, nearly all of it function bodies and
// vector intrinsics), so it is never decoded as a whole. scanTU cuts it into
// its top-level declarations and hands each one to the caller, who decodes
// only those that matter.
type rawDecl struct {
	// Kind is the "kind" of the declaration, read from the start of its
	// text ("" when it is not there).
	Kind string

	// File is the file that clang had printed last when the declaration
	// began. The JSON names a file only where it changes, so the file of a
	// declaration is the first "file" in its own text, or this one.
	File string

	// Data is the text of the declaration. It is only valid during the call
	// of the visitor. Nil when Skipped.
	Data []byte

	// Skipped says that the declaration was larger than the limit and its
	// text was not kept.
	Skipped bool
}

// maxDeclBytes limits one declaration that scanTU keeps (a variable for the tests).
var maxDeclBytes = 64 << 20

// errNotDump says that the text is not a clang JSON dump.
var errNotDump = errors.New("the output is not clang's JSON AST dump")

// scanTU reads the JSON dump of a translation unit from r and calls visit for
// each declaration of the top level, in order. It tracks the "file" keys of
// the whole text, so File is right for every declaration, and it keeps
// no more than one declaration in memory. visit returns an error to stop.
//
// It reads clang's output as a byte stream with a small state machine
// (strings and brackets), not with encoding/json: it has to run at the speed
// of the pipe.
func scanTU(r io.Reader, visit func(d *rawDecl) error) error {
	br := bufio.NewReaderSize(r, 1<<20)

	var (
		depth    int    // nesting of { and [ (the root object is 1)
		inStr    bool   // inside a string
		esc      bool   // the last byte in a string was a backslash
		str      []byte // the string being read (cut at 4 KB)
		strLong  bool
		lastStr  string // the last string that was closed
		key      string // the last string that was followed by ':'
		afterCol bool   // the last token was ':' (the next string is a value)
		inInner  bool   // inside the "inner" array of the root
		incDepth int    // depth of an "includedFrom" object, else 0
		file     string // the file of the last location
		seenRoot bool

		node     []byte // the declaration being read
		inNode   bool
		big      bool
		bigKind  string // the kind of a declaration that is too large to keep
		nodeFile string
	)
	buf := make([]byte, 1<<20)
	for {
		n, err := br.Read(buf)
		chunk := buf[:n]
		start := -1 // start of the part of the chunk that belongs to the node
		if inNode {
			start = 0
		}
		for i := 0; i < len(chunk); i++ {
			c := chunk[i]
			if inStr {
				if !esc {
					// the fast path: a run of the string with no backslash
					seg := chunk[i:]
					q := bytes.IndexByte(seg, '"')
					if q >= 0 {
						seg = seg[:q]
					}
					if bytes.IndexByte(seg, '\\') < 0 {
						if !strLong {
							if len(str)+len(seg) > 4096 {
								strLong = true
							} else {
								str = append(str, seg...)
							}
						}
						if q < 0 {
							i = len(chunk)
							continue
						}
						i += q
						c = '"'
					}
				}
				if esc {
					esc = false
					if !strLong {
						str = append(str, c)
					}
					continue
				}
				switch c {
				case '\\':
					esc = true
					if !strLong {
						str = append(str, c)
					}
				case '"':
					inStr = false
					lastStr = string(str)
					if afterCol {
						afterCol = false
						if key == "file" && incDepth == 0 && !strLong {
							file = unescapeJSONString(lastStr)
						}
					}
				default:
					if !strLong {
						if len(str) >= 4096 {
							strLong = true
						} else {
							str = append(str, c)
						}
					}
				}
				continue
			}
			if skipByte[c] {
				// white space and commas: most of the dump
				for i+1 < len(chunk) && skipByte[chunk[i+1]] {
					i++
				}
				continue
			}
			switch c {
			case '"':
				inStr = true
				str = str[:0]
				strLong = false
			case ':':
				key = lastStr
				afterCol = true
			case '{', '[':
				afterCol = false
				depth++
				if c == '[' && depth == 2 && key == "inner" {
					inInner = true
				}
				if key == "includedFrom" && c == '{' && incDepth == 0 {
					incDepth = depth
				}
				if c == '{' && depth == 3 && inInner {
					inNode = true
					big = false
					node = node[:0]
					nodeFile = file
					start = i
				}
				if depth == 1 {
					seenRoot = true
				}
			case '}', ']':
				if depth == incDepth && incDepth != 0 {
					incDepth = 0
				}
				if c == '}' && depth == 3 && inNode {
					if !big {
						if len(node)+i+1-start > maxDeclBytes {
							big = true
							bigKind = kindOf(append(node[:len(node):len(node)], chunk[start:min(i+1, start+256)]...))
						} else {
							node = append(node, chunk[start:i+1]...)
						}
					}
					d := &rawDecl{File: nodeFile, Skipped: big}
					if big {
						d.Kind = bigKind
					} else {
						d.Kind, d.Data = kindOf(node), node
					}
					inNode = false
					start = -1
					if err := visit(d); err != nil {
						return err
					}
				}
				if c == ']' && depth == 2 {
					inInner = false
				}
				depth--
			default:
				afterCol = false // a number or a word after ':' is the value
			}
		}
		if inNode && start >= 0 {
			if !big {
				if len(node)+len(chunk)-start > maxDeclBytes {
					big = true
					bigKind = kindOf(append(node[:len(node):len(node)], chunk[start:min(len(chunk), start+256)]...))
					node = node[:0]
				} else {
					node = append(node, chunk[start:]...)
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	if !seenRoot || depth != 0 {
		if !seenRoot {
			return errNotDump
		}
		return fmt.Errorf("%w: it ends inside a declaration", errNotDump)
	}
	return nil
}

// skipByte marks the bytes that mean nothing to the scanner outside strings.
var skipByte = [256]bool{' ': true, '\n': true, '\t': true, '\r': true, ',': true}

// kindOf returns the "kind" of a declaration from the start of its text
// (the second key, after "id").
func kindOf(data []byte) string {
	head := data
	if len(head) > 256 {
		head = head[:256]
	}
	const k = `"kind":`
	for i := 0; i+len(k) < len(head); i++ {
		if string(head[i:i+len(k)]) != k {
			continue
		}
		j := i + len(k)
		for j < len(head) && (head[j] == ' ' || head[j] == '\n') {
			j++
		}
		if j >= len(head) || head[j] != '"' {
			return ""
		}
		j++
		e := j
		for e < len(head) && head[e] != '"' {
			e++
		}
		return string(head[j:e])
	}
	return ""
}

// unescapeJSONString undoes the escapes of a JSON string that was kept raw
// (file names have a backslash only on Windows, as \\).
func unescapeJSONString(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			continue
		}
		out := make([]byte, 0, len(s))
		out = append(out, s[:i]...)
		for j := i; j < len(s); j++ {
			if s[j] == '\\' && j+1 < len(s) {
				j++
				switch s[j] {
				case 'n':
					out = append(out, '\n')
				case 't':
					out = append(out, '\t')
				default:
					out = append(out, s[j])
				}
				continue
			}
			out = append(out, s[j])
		}
		return string(out)
	}
	return s
}
