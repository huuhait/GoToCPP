package diff

import (
	"go/parser"
	"go/token"
)

func parseOnly(src string) (any, error) {
	return parser.ParseFile(token.NewFileSet(), "x.go", src, 0)
}
