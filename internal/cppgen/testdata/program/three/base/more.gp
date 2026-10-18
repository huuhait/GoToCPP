package base

func Describe(s Shape) string {
	if _, ok := s.(Rect); ok {
		return "rect"
	}
	return "shape"
}
