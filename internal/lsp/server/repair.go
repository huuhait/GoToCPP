package server

import (
	"bytes"
	"errors"
	"fmt"
	"go/token"
	"slices"
	"sort"
	"sync"

	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/syntax/parser"
	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// A file that is being typed has a syntax error most of the time, and the parser's recovery is poor at one thing: a
// call whose `)` is missing takes the tokens after it for more arguments, up to the next `)`, which can be in
// another function, so the tree of the file ends there and every declaration after it is lost. The features
// that look at the whole file (hints, tokens) and at a place (hover, definition) then show nothing for them, and
// the `!` features, which need the checker's lowering, blink off with every keystroke (step 11.13, ADR-099).
//
// The server repairs the text for them, in its own way: the declarations that the first syntax errors are in
// are blanked (every byte but the line ends becomes a space, so all offsets stay), the repaired text is parsed
// and checked like any file (cache.Snapshot.PackageWithFile, the check completion uses for a fixed text), and the
// features answer from that check for everything that is not in a blanked declaration; what is in one still comes
// from the check of the text as it is (the partial check), which is all that exists for it. References and
// rename do not use this: a rename that did not see the blanked declarations would be wrong.

// declStarts finds column-one declaration tokens: func, type, var, const, macro or #. These are lexical
// recovery boundaries even after an unmatched brace/parenthesis, not textual matches inside strings/comments.
// Consecutive # lines belong to the following declaration. An unterminated raw string or block comment is
// consumed by the scanner through EOF: uncertain text inside it must not become a declaration boundary.
func declStarts(src []byte) []int {
	var starts []int
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	var sc scanner.Scanner
	sc.Init(file, src, func(token.Position, string) {}, scanner.Phase10Syntax)
	nextHashLine := -1
	for {
		pos, tok, lit := sc.Scan()
		if tok == token.EOF {
			break
		}
		off := file.Offset(pos)
		if off > 0 && src[off-1] != '\n' {
			continue
		}
		isHash := off < len(src) && src[off] == '#'
		isKw := tok == token.FUNC || tok == token.TYPE || tok == token.VAR || tok == token.CONST || tok == token.IDENT && lit == "macro"
		if !isKw && !isHash {
			continue
		}
		if off != nextHashLine {
			starts = append(starts, off)
		}
		nextHashLine = -1
		if isHash {
			if end := bytes.IndexByte(src[off:], '\n'); end >= 0 {
				nextHashLine = off + end + 1
			}
		}
	}
	return starts
}

// hasWord reports whether line starts with the word w (followed by a space, a tab, a paren or the end).
func hasWord(line []byte, w string) bool {
	if !bytes.HasPrefix(line, []byte(w)) {
		return false
	}
	rest := line[len(w):]
	return len(rest) == 0 || rest[0] == ' ' || rest[0] == '\t' || rest[0] == '('
}

// A repair is a text with declarations blanked, and where.
type repair struct {
	text    []byte
	blanked [][2]int // sorted, disjoint: the offsets of what is blanked (a declaration, or the body of a function)
}

// within reports whether the offset off is in what is blanked.
func (r *repair) within(off int) bool {
	i := sort.Search(len(r.blanked), func(i int) bool { return r.blanked[i][1] > off })
	return i < len(r.blanked) && r.blanked[i][0] <= off
}

// How much of a declaration is blanked.
const (
	keep int8 = iota // nothing
	body             // the body of a function: the signature stays, so that the callers check
	full             // the whole declaration
)

// blankRange makes src[lo:hi] white space but for the line ends.
func blankRange(text []byte, lo, hi int) {
	for j := lo; j < hi; j++ {
		if text[j] != '\n' && text[j] != '\r' {
			text[j] = ' '
		}
	}
}

// bodySpan returns the offsets lo and hi of the body of the function or macro that the declaration src[start:end]
// is: the text between its opening brace and the last closing brace of the declaration, which stay. ok is false for
// any other declaration, and when the braces are not there.
func bodySpan(src []byte, start, end int) (lo, hi int, ok bool) {
	seg := src[start:end]
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(seg))
	var sc scanner.Scanner
	sc.Init(file, seg, func(token.Position, string) {}, 0)
	depth, header, typeBrace := 0, false, 0 // typeBrace: braces of a struct or interface type in the signature
	var prev token.Token
	lo, hi = -1, -1
	for {
		pos, tok, lit := sc.Scan()
		if tok == token.EOF {
			break
		}
		off := file.Offset(pos)
		switch {
		case !header:
			switch tok {
			case token.LPAREN, token.LBRACK:
				depth++
			case token.RPAREN, token.RBRACK:
				depth--
			}
			if depth == 0 && (tok == token.FUNC || tok == token.IDENT && lit == "macro" && prevIsLineStart(seg, off)) {
				header = true
			}
		case lo < 0:
			switch tok {
			case token.LPAREN, token.LBRACK:
				depth++
			case token.RPAREN, token.RBRACK:
				depth--
			case token.LBRACE:
				switch {
				case depth > 0 || typeBrace > 0 || prev == token.STRUCT || prev == token.INTERFACE:
					typeBrace++
				default:
					lo = off + 1
				}
			case token.RBRACE:
				if typeBrace > 0 {
					typeBrace--
				}
			}
		default:
			if tok == token.RBRACE {
				hi = off
			}
		}
		prev = tok
	}
	if lo < 0 || hi < lo {
		return 0, 0, false
	}
	return start + lo, start + hi, true
}

// prevIsLineStart reports whether the token at off is the first on its line.
func prevIsLineStart(seg []byte, off int) bool {
	return len(bytes.TrimSpace(seg[bytes.LastIndexByte(seg[:off], '\n')+1:off])) == 0
}

// blankText returns src with the declarations blanked as how says.
func blankText(src []byte, starts []int, how []int8) []byte {
	text := bytes.Clone(src)
	for k, h := range how {
		if h == keep {
			continue
		}
		end := len(text)
		if k+1 < len(starts) {
			end = starts[k+1]
		}
		if h == body {
			if lo, hi, ok := bodySpan(src, starts[k], end); ok {
				blankRange(text, lo, hi)
				// a function with results needs a terminating statement: panic is one
				for j := lo; j+9 <= hi; j++ {
					if !bytes.ContainsAny(text[j:j+9], "\n\r") {
						copy(text[j:], `panic("")`)
						break
					}
				}
				continue
			}
		}
		blankRange(text, starts[k], end)
	}
	return text
}

// unbalanced says for each declaration whether its brackets do not match: more closing than opening ones at some
// place, or a different number of them at the end. The scanner reads the text, so brackets in strings and comments
// count for nothing. A missing parenthesis or brace is what the parser's recovery loses the rest of the file to.
func unbalanced(src []byte, starts []int) []bool {
	bad := make([]bool, len(starts))
	if len(starts) == 0 {
		return bad
	}
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	var sc scanner.Scanner
	sc.Init(file, src, func(token.Position, string) {}, 0)
	seg := -1
	var depth [3]int
	finish := func() {
		if seg >= 0 && depth != [3]int{} {
			bad[seg] = true
		}
		depth = [3]int{}
	}
	for {
		pos, tok, _ := sc.Scan()
		if tok == token.EOF {
			break
		}
		off := file.Offset(pos)
		for seg+1 < len(starts) && starts[seg+1] <= off {
			finish()
			seg++
		}
		if seg < 0 {
			continue // the header
		}
		switch tok {
		case token.LPAREN:
			depth[0]++
		case token.RPAREN:
			depth[0]--
		case token.LBRACE:
			depth[1]++
		case token.RBRACE:
			depth[1]--
		case token.LBRACK:
			depth[2]++
		case token.RBRACK:
			depth[2]--
		}
		if depth[0] < 0 || depth[1] < 0 || depth[2] < 0 {
			bad[seg] = true
		}
	}
	finish()
	return bad
}

// maxRepairPasses bounds the parses that repairText makes: a file with more broken places is not repaired.
const maxRepairPasses = 8

// spanOf returns the range blanked for the declaration k.
func spanOf(src []byte, starts []int, how []int8, k int) [2]int {
	end := len(src)
	if k+1 < len(starts) {
		end = starts[k+1]
	}
	if how[k] == body {
		if lo, hi, ok := bodySpan(src, starts[k], end); ok {
			return [2]int{lo, hi}
		}
	}
	return [2]int{starts[k], end}
}

// repairText returns the repair of the file pf, which has a syntax error or a load error at the offsets loadErrs
// (the macro uses that name nothing, say), or nil when blanking declarations does not give a file that parses (the
// error is in the header, or there are too many of them). A function is blanked down to its body first: its callers
// still check; only when its signature is what is wrong is it blanked whole.
func repairText(pf *cache.ParsedFile, loadErrs []int) *repair {
	starts := declStarts(pf.Src)
	if len(starts) == 0 {
		return nil
	}
	declAt := func(off int) int { return sort.Search(len(starts), func(i int) bool { return starts[i] > off }) - 1 }
	how := make([]int8, len(starts))
	for k, bad := range unbalanced(pf.Src, starts) {
		if bad {
			how[k] = body
		}
	}
	for _, off := range loadErrs {
		k := declAt(off)
		if k < 0 {
			return nil
		}
		how[k] = max(how[k], body)
	}
	src, err := pf.Src, pf.Err
	if slices.Contains(how, body) {
		src = blankText(pf.Src, starts, how)
		_, err = parser.ParseFile(token.NewFileSet(), pf.Path, src, cache.ParseFull)
	} else if err == nil {
		return nil
	}
	for pass := 0; pass < maxRepairPasses && err != nil; pass++ {
		var list scanner.ErrorList
		if !errors.As(err, &list) || len(list) == 0 {
			return nil
		}
		head := list[0]
		for _, e := range list {
			if e.Pos.Offset < head.Pos.Offset {
				head = e
			}
		}
		k := declAt(min(head.Pos.Offset, len(pf.Src)))
		if k < 0 {
			return nil // the header
		}
		if how[k] == full {
			// the error is at a declaration that is blanked already: the damage is before it (a brace the scan
			// of the brackets did not show)
			if k == 0 {
				return nil
			}
			k--
		}
		how[k]++
		src = blankText(pf.Src, starts, how)
		_, err = parser.ParseFile(token.NewFileSet(), pf.Path, src, cache.ParseFull)
	}
	if err != nil {
		return nil
	}
	r := &repair{text: src}
	for k, h := range how {
		if h == keep {
			continue
		}
		sp := spanOf(pf.Src, starts, how, k)
		if n := len(r.blanked); n > 0 && r.blanked[n-1][1] >= sp[0] {
			r.blanked[n-1][1] = max(r.blanked[n-1][1], sp[1])
		} else {
			r.blanked = append(r.blanked, sp)
		}
	}
	return r
}

// The repairs of the last files, by content: the features of a file that is being typed ask for it on every
// request.
var repairs struct {
	sync.Mutex
	m map[repairKey]*repair
}

type repairKey struct {
	path string
	hash cache.Hash
	errs string // the offsets of the load errors
}

func repairOf(pf *cache.ParsedFile, loadErrs []int) *repair {
	k := repairKey{pf.Path, pf.Hash, fmt.Sprint(loadErrs)}
	repairs.Lock()
	r, ok := repairs.m[k]
	repairs.Unlock()
	if ok {
		return r
	}
	r = repairText(pf, loadErrs)
	repairs.Lock()
	if repairs.m == nil || len(repairs.m) >= 16 {
		repairs.m = make(map[repairKey]*repair)
	}
	repairs.m[k] = r
	repairs.Unlock()
	return r
}
