package colors

type Color int

const (
	Red Color = iota
	Green
	Blue
)

func (c Color) String() string {
	switch c {
	case Red:
		return "red"
	case Green:
		return "green"
	}
	return "blue"
}

var Palette = [...]Color{Red, Green, Blue}

var ByColor = map[Color]string{Red: "r", Green: "g"}
