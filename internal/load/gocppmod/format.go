package gocppmod

import "strings"

// Format returns the text of the file: one statement per line, the words
// of a line separated by one space, the lines of a block indented by a tab,
// and the comments and the blank lines kept (several blank lines become
// one). A word is written as it was read, so a statement that gocpp does
// not know comes out unchanged. Format of the output of Format is the
// same text.
func Format(f *FileSyntax) []byte {
	var b strings.Builder
	for i, s := range f.Stmt {
		if i > 0 && blankBefore(s) {
			b.WriteByte('\n')
		}
		switch s := s.(type) {
		case *CommentBlock:
			writeComments(&b, s.Before, "")
		case *Line:
			writeComments(&b, s.Before, "")
			writeLine(&b, "", s.Token, s.Suffix)
		case *Block:
			writeComments(&b, s.Before, "")
			open := append(append([]string(nil), s.Token...), string(s.Open))
			writeLine(&b, "", open, s.Suffix)
			if s.Open == '{' {
				for _, raw := range s.Raw {
					b.WriteString(raw)
					b.WriteByte('\n')
				}
				if !s.Unterminated {
					writeLine(&b, "", []string{"}"}, s.CloseSuffix)
				}
				continue
			}
			for _, l := range s.Line {
				if l.Blank {
					b.WriteByte('\n')
				}
				writeComments(&b, l.Before, "\t")
				writeLine(&b, "\t", l.Token, l.Suffix)
			}
			writeComments(&b, s.CloseBefore, "\t")
			if !s.Unterminated {
				writeLine(&b, "", []string{")"}, s.CloseSuffix)
			}
		}
	}
	return []byte(b.String())
}

func blankBefore(s Stmt) bool {
	switch s := s.(type) {
	case *Line:
		return s.Blank
	case *Block:
		return s.Blank
	case *CommentBlock:
		return s.Blank
	}
	return false
}

func writeComments(b *strings.Builder, list []Comment, indent string) {
	for _, c := range list {
		if c.Token != "" {
			b.WriteString(indent)
			b.WriteString(c.Token)
		}
		b.WriteByte('\n')
	}
}

func writeLine(b *strings.Builder, indent string, token []string, suffix []Comment) {
	b.WriteString(indent)
	b.WriteString(strings.Join(token, " "))
	for _, c := range suffix {
		b.WriteByte(' ')
		b.WriteString(c.Token)
	}
	b.WriteByte('\n')
}
