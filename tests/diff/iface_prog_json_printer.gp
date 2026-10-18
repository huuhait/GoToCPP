// phase: 05

// A JSON-like encoder over any: type switches with Marshaler, nested containers, wrapped errors with errors.As on a pointer type and an interface.
package main

import (
	"errors"
	"fmt"
)

type JSON any

func quote(s string) string {
	out := []byte{'"'}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '\\':
			out = append(out, '\\', c)
		case '\n':
			out = append(out, '\\', 'n')
		default:
			out = append(out, c)
		}
	}
	return string(append(out, '"'))
}

func sortedKeys(m map[string]JSON) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

type Marshaler interface{ MarshalJSON() (string, error) }

type Temp float64

func (t Temp) MarshalJSON() (string, error) { return fmt.Sprintf("%.1f", float64(t)), nil }

type Bad struct{}

func (Bad) MarshalJSON() (string, error) { return "", errors.New("bad value") }

type UnsupportedError struct{ Type string }

func (e *UnsupportedError) Error() string { return "unsupported type " + e.Type }

func enc(v JSON, indent string) (string, error) {
	switch x := v.(type) {
	case nil:
		return "null", nil
	case bool:
		return fmt.Sprint(x), nil
	case int, int64, uint8:
		return fmt.Sprint(x), nil
	case float64:
		return fmt.Sprint(x), nil
	case string:
		return quote(x), nil
	case Marshaler:
		s, err := x.MarshalJSON()
		if err != nil {
			return "", fmt.Errorf("marshaler %T: %w", x, err)
		}
		return s, nil
	case []JSON:
		if len(x) == 0 {
			return "[]", nil
		}
		s := "[\n"
		for i, e := range x {
			es, err := enc(e, indent+"  ")
			if err != nil {
				return "", fmt.Errorf("[%d]: %w", i, err)
			}
			s += indent + "  " + es
			if i < len(x)-1 {
				s += ","
			}
			s += "\n"
		}
		return s + indent + "]", nil
	case map[string]JSON:
		if len(x) == 0 {
			return "{}", nil
		}
		s := "{\n"
		keys := sortedKeys(x)
		for i, k := range keys {
			es, err := enc(x[k], indent+"  ")
			if err != nil {
				return "", fmt.Errorf("%s: %w", k, err)
			}
			s += indent + "  " + quote(k) + ": " + es
			if i < len(keys)-1 {
				s += ","
			}
			s += "\n"
		}
		return s + indent + "}", nil
	}
	return "", &UnsupportedError{fmt.Sprintf("%T", v)}
}

func main() {
	doc := map[string]JSON{
		"name":  "gocpp \"demo\"\n",
		"ok":    true,
		"n":     42,
		"pi":    3.5,
		"temp":  Temp(36.6),
		"none":  nil,
		"list":  []JSON{1, "two", []JSON{}, map[string]JSON{}, []JSON{nil, false}},
		"inner": map[string]JSON{"b": 2, "a": []JSON{int64(1), uint8(2)}},
	}
	s, err := enc(doc, "")
	fmt.Println(s, err)
	for _, bad := range []JSON{
		map[string]JSON{"x": []JSON{1, Bad{}}},
		[]JSON{struct{}{}},
		map[string]JSON{"c": complex(1, 2)},
		map[int]string{},
	} {
		_, err := enc(bad, "")
		var ue *UnsupportedError
		var m Marshaler
		fmt.Println(err, errors.As(err, &ue), errors.As(err, &m))
		if ue != nil {
			fmt.Println(" type:", ue.Type)
		}
	}
}
