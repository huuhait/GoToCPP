package main

import "example.com/pk/model"

// Things and Table are aliases to composite types of another package; box.gp does not import it.
type Things = []model.Thing

type Table = map[string][]model.Thing
