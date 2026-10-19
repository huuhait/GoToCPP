package reg

var Log []string

type Registry[K comparable, V any] struct {
	items map[K]V
	order []K
}

func New[K comparable, V any]() *Registry[K, V] {
	Log = append(Log, "new")
	return &Registry[K, V]{items: map[K]V{}}
}

func (r *Registry[K, V]) Add(k K, v V) *Registry[K, V] {
	Log = append(Log, "add")
	r.items[k] = v
	r.order = append(r.order, k)
	return r
}

func (r *Registry[K, V]) Keys() []K { return r.order }

var Default = New[string, int]().Add("one", 1).Add("two", 2)

func init() { Log = append(Log, "reg init") }
