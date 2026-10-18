package other

import (
	"example.com/anonemb/base"
	"example.com/anonemb/mk"
)

func Wrap() any {
	v := mk.Tagged(5)
	return v
}

func IsSame(a any) bool {
	_, ok := a.(struct {
		base.Base
		Tag string
	})
	return ok
}
