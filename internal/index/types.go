package index

import "github.com/google/btree"

type value struct {
	text    string
	number  bool
	numeric float64
}

type Index struct {
	keys     *btree.BTreeG[string]
	values   *btree.BTreeG[value]
	entries  map[string]value
	inverted map[value]map[string]struct{}
}
