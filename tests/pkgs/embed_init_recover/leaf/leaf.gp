package leaf

import "example.com/app/base"

var CC = base.A + 100

func init() { base.Log("leaf.init") }

type Mood int

const (
	Happy Mood = iota
	Sad
)

func (m Mood) String() string {
	switch m {
	case Happy:
		return "happy"
	}
	return "sad"
}
