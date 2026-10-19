package fuzzkit

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Dump returns a text that describes the syntax tree n without positions, comments, parentheses,
// the resolver's objects and scopes, and the order of imports: two trees that Dump gives the same
// text for are the same program, which is what the formatter must keep. (gofmt sorts the imports
// of a block, and may add or drop parentheses, so neither is compared.)
func Dump(n ast.Node) string {
	var b strings.Builder
	dump(&b, reflect.ValueOf(n))
	return b.String()
}

var (
	posType     = reflect.TypeFor[token.Pos]()
	commentType = reflect.TypeFor[*ast.CommentGroup]()
	//lint:ignore SA1019 the parser fork still fills the resolver objects, which Dump must skip
	objType = reflect.TypeFor[*ast.Object]()
	//lint:ignore SA1019 same
	scopeType = reflect.TypeFor[*ast.Scope]()
)

func dump(b *strings.Builder, v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			if v.Type() == reflect.TypeFor[*ast.FieldList]() {
				b.WriteString("FieldList{}") // no results and an empty list of results are one type
				return
			}
			b.WriteString("nil")
			return
		}
		if v.Kind() == reflect.Pointer {
			switch n := v.Interface().(type) {
			case *ast.ParenExpr:
				dump(b, reflect.ValueOf(n.X))
				return
			case *ast.BasicLit:
				if n.Kind == token.INT || n.Kind == token.FLOAT || n.Kind == token.IMAG {
					// The formatter normalizes number literals (0XFF, 1E5, 0X1P-2 and the legacy octal
					// imaginary 0123i): the same number.
					b.WriteString("Num(" + normalizeNumber(n.Value) + ")")
					return
				}
			case *ast.FieldList:
				if n != nil && len(n.List) == 0 {
					// "func()()" and "func()" are one type: the printer drops the empty list of results
					// (and "()" of a func type has no other meaning).
					b.WriteString("FieldList{}")
					return
				}
			}
		}
		dump(b, v.Elem())
	case reflect.Struct:
		t := v.Type()
		b.WriteString(t.Name())
		b.WriteByte('{')
		for i := range t.NumField() {
			f := t.Field(i)
			ft := f.Type
			if ft == posType || ft == commentType || ft == objType || ft == scopeType || !f.IsExported() {
				continue
			}
			switch f.Name {
			case "Comments", "Unresolved", "Imports", "Doc", "Comment", "Obj", "Scope", "FileStart", "FileEnd", "GoVersion", "Implicit":
				continue
			}
			b.WriteString(f.Name)
			b.WriteByte('=')
			if f.Name == "Specs" && t == reflect.TypeFor[ast.GenDecl]() && v.FieldByName("Tok").Interface() == token.IMPORT {
				dumpSorted(b, v.Field(i))
			} else {
				dump(b, v.Field(i))
			}
			b.WriteByte(';')
		}
		b.WriteByte('}')
	case reflect.Slice:
		b.WriteByte('[')
		first := true
		for i := range v.Len() {
			if e := v.Index(i); e.Kind() == reflect.Interface && !e.IsNil() {
				if _, ok := e.Interface().(*ast.EmptyStmt); ok {
					continue // the formatter drops a ";" statement of a list
				}
			}
			if !first {
				b.WriteByte(',')
			}
			first = false
			dump(b, v.Index(i))
		}
		b.WriteByte(']')
	case reflect.String:
		b.WriteString(strconv.Quote(v.String()))
	default:
		fmt.Fprint(b, v.Interface())
	}
}

// normalizeNumber lowercases a number literal (the prefix and the exponent are the ones that gofmt
// changes; the digits of a hex number are the same value in either case) and drops the leading
// zeros of an imaginary literal that has only decimal digits (0123i is 123i).
func normalizeNumber(v string) string {
	v = strings.ToLower(v)
	if strings.HasSuffix(v, "i") {
		body := strings.TrimSuffix(v, "i")
		if body != "" && strings.Trim(body, "0123456789") == "" {
			body = strings.TrimLeft(body, "0")
			if body == "" {
				body = "0"
			}
			return body + "i"
		}
	}
	return v
}

// dumpSorted dumps the elements of a slice in the order of their dumps (the specs of an import
// block, which the formatter sorts).
func dumpSorted(b *strings.Builder, v reflect.Value) {
	parts := make([]string, v.Len())
	for i := range parts {
		var sb strings.Builder
		dump(&sb, v.Index(i))
		parts[i] = sb.String()
	}
	sort.Strings(parts)
	parts = slices.Compact(parts) // the formatter drops a repeated import spec
	b.WriteString("[" + strings.Join(parts, ",") + "]")
}
