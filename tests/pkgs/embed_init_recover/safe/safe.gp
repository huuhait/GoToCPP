package safe

import "fmt"

type RecErr struct{ V any }

func (e *RecErr) Error() string { return fmt.Sprint("recovered: ", e.V) }

func Try(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &RecErr{r}
		}
	}()
	f()
	return nil
}

func Recover() {
	if r := recover(); r != nil {
		fmt.Println("safe.Recover got", r)
	}
}

type Closer struct{ Name string }

func (c *Closer) Close() {
	if r := recover(); r != nil {
		fmt.Println("closer", c.Name, "recovered", r)
	}
}
