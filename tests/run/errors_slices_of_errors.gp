// phase: 05

// Collecting errors in slices and maps, joining them with spread, and picking the ones that match.

package main

import (
	"errors"
	"fmt"
)

type FieldErr struct {
	Field string
	Err   error
}

func (f *FieldErr) Error() string { return f.Field + ": " + f.Err.Error() }
func (f *FieldErr) Unwrap() error { return f.Err }

var (
	ErrEmpty = errors.New("empty")
	ErrLong  = errors.New("too long")
)

func validate(fields map[string]string, order []string) error {
	var errs []error
	for _, name := range order {
		v := fields[name]
		switch {
		case v == "":
			errs = append(errs, &FieldErr{name, ErrEmpty})
		case len(v) > 5:
			errs = append(errs, &FieldErr{name, ErrLong})
		default:
			errs = append(errs, nil)
		}
	}
	return errors.Join(errs...)
}

func main() {
	order := []string{"name", "city", "zip", "street"}
	err := validate(map[string]string{"name": "ann", "city": "", "zip": "123456", "street": ""}, order)
	fmt.Printf("%q\n", err.Error())
	fmt.Println(errors.Is(err, ErrEmpty), errors.Is(err, ErrLong))
	var fe *FieldErr
	fmt.Println(errors.As(err, &fe), fe.Field)
	fmt.Println(validate(map[string]string{"name": "a", "city": "b", "zip": "c", "street": "d"}, order) == nil)
	byField := map[string]error{}
	for _, e := range err.(interface{ Unwrap() []error }).Unwrap() {
		var f *FieldErr
		if errors.As(e, &f) {
			byField[f.Field] = f.Err
		}
	}
	fmt.Println(len(byField), byField["city"] == ErrEmpty, byField["zip"] == ErrLong, byField["name"] == nil)
}

// Output:
// "city: empty\nzip: too long\nstreet: empty"
// true true
// true city
// true
// 3 true true true
