// phase: 09

package main

import (
	"errors"
	"fmt"
	"reflect"
)

type MyErr struct{ Code int }

func (e *MyErr) Error() string { return fmt.Sprint("code ", e.Code) }

func main() {
	errType := reflect.TypeOf((*error)(nil)).Elem()
	e1 := errors.New("plain")
	e2 := fmt.Errorf("wrap: %w", e1)
	e3 := error(&MyErr{7})
	for _, e := range []error{e1, e2, e3} {
		t := reflect.TypeOf(e)
		fmt.Println(t, t.Kind(), t.Implements(errType), t.Elem().Kind(), reflect.ValueOf(e).Elem().NumField())
	}
	v := reflect.ValueOf(e3)
	fmt.Println(v.Elem().Field(0).Int())
}

// Output:
// *errors.errorString ptr true struct 1
// *fmt.wrapError ptr true struct 2
// *main.MyErr ptr true struct 1
// 7
