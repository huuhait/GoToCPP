package use

import "example.com/cont/cont"

// Containers over structs of another package: the descriptor of the container (here) points at the
// descriptor of the element (there).
type Items []cont.Item
type Names map[string]cont.Base
type Pair [2]cont.Item
type Ptr *cont.Base
type Nodes []cont.Tree

func (i Items) Len() int { return len(i) }

func Boxes() []any {
	return []any{Items{{1}, {2}}, Names{"n": {4}}, Pair{{5}, {6}}, Ptr(&cont.Base{8}), Nodes{{Name: "t"}}}
}
