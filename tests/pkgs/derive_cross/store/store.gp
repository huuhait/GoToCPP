// Package store uses the derived methods of model in the types it derives itself.
package store

import "example.com/derive/model"

#Derive(Equal, Clone)
type Shelf struct {
	model.Item
	Next   *model.Item
	Others []model.Item
	Kinds  map[model.Kind][]model.Item
	Stack  model.Stack[string]
}
