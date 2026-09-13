package index

import (
	"errors"
	"github.com/google/btree"
	"math"
	"sort"
	"strconv"
	"strings"
)

func New(degree int) *Index {
	if degree < 2 {
		degree = 2
	}
	return &Index{
		keys:    btree.NewOrderedG[string](degree),
		values:  btree.NewG[value](degree, less),
		entries: map[string]value{}, inverted: map[value]map[string]struct{}{},
	}
}

func less(a, b value) bool {
	if a.number != b.number {
		return !a.number
	}
	if a.number {
		return a.numeric < b.numeric
	}
	return a.text < b.text
}

func scalar(text string, number bool) (value, error) {
	result := value{text: text, number: number}
	if number {
		n, err := strconv.ParseFloat(text, 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return value{}, errors.New("nombre fini requis")
		}
		if n == 0 {
			n = 0
		}
		result.numeric = n
		result.text = strconv.FormatFloat(n, 'g', -1, 64)
	}
	return result, nil
}

func (idx *Index) Set(key, text string, number bool) error {
	item, err := scalar(text, number)
	if err != nil {
		return err
	}
	idx.Delete(key)
	idx.entries[key] = item
	idx.keys.ReplaceOrInsert(key)
	if idx.inverted[item] == nil {
		idx.inverted[item] = map[string]struct{}{}
		idx.values.ReplaceOrInsert(item)
	}
	idx.inverted[item][key] = struct{}{}
	return nil
}

func (idx *Index) Delete(key string) {
	item, exists := idx.entries[key]
	if !exists {
		return
	}
	delete(idx.entries, key)
	idx.keys.Delete(key)
	delete(idx.inverted[item], key)
	if len(idx.inverted[item]) == 0 {
		delete(idx.inverted, item)
		idx.values.Delete(item)
	}
}

func (idx *Index) Find(field, operator, operand string, number bool) ([]string, error) {
	if field != "key" && field != "value" {
		return nil, errors.New("champ attendu : key ou value")
	}
	switch operator {
	case "equals", "contains", ">", ">=", "<", "<=":
	default:
		return nil, errors.New("opérateur inconnu")
	}
	if field == "key" {
		number = false
	}
	pivot, err := scalar(operand, number)
	if err != nil {
		return nil, err
	}
	keys := []string{}
	if operator == "contains" {
		for key, item := range idx.entries {
			if field == "key" && strings.Contains(key, operand) || field == "value" && !item.number && strings.Contains(item.text, operand) {
				keys = append(keys, key)
			}
		}
	} else if field == "key" {
		visit := func(key string) bool {
			if operator == ">" && key == operand || operator == "<" && key == operand {
				return true
			}
			keys = append(keys, key)
			return true
		}
		switch operator {
		case "equals":
			if _, exists := idx.entries[operand]; exists {
				keys = append(keys, operand)
			}
		case ">", ">=":
			idx.keys.AscendGreaterOrEqual(operand, visit)
		case "<", "<=":
			idx.keys.DescendLessOrEqual(operand, visit)
		}
	} else {
		visit := func(item value) bool {
			if item.number != pivot.number {
				return false
			}
			if (operator == ">" || operator == "<") && item == pivot {
				return true
			}
			for key := range idx.inverted[item] {
				keys = append(keys, key)
			}
			return true
		}
		switch operator {
		case "equals":
			for key := range idx.inverted[pivot] {
				keys = append(keys, key)
			}
		case ">", ">=":
			idx.values.AscendGreaterOrEqual(pivot, visit)
		case "<", "<=":
			idx.values.DescendLessOrEqual(pivot, visit)
		}
	}
	sort.Strings(keys)
	return keys, nil
}
