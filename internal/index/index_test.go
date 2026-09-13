package index

import (
	"math/rand"
	"reflect"
	"sort"
	"strconv"
	"testing"
)

func TestIndexMatchesScan(t *testing.T) {
	idx := New(3)
	source := map[string]string{}
	random := rand.New(rand.NewSource(42))
	for step := 0; step < 500; step++ {
		key := strconv.Itoa(random.Intn(80))
		if random.Intn(4) == 0 {
			delete(source, key)
			idx.Delete(key)
		} else {
			text := strconv.Itoa(random.Intn(30) - 15)
			source[key] = text
			if err := idx.Set(key, text, true); err != nil {
				t.Fatal(err)
			}
		}
		for _, operator := range []string{"equals", ">", ">=", "<", "<="} {
			want := []string{}
			for key, text := range source {
				n, _ := strconv.Atoi(text)
				match := operator == "equals" && n == 3 || operator == ">" && n > 3 || operator == ">=" && n >= 3 || operator == "<" && n < 3 || operator == "<=" && n <= 3
				if match {
					want = append(want, key)
				}
			}
			sort.Strings(want)
			got, err := idx.Find("value", operator, "3", true)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("étape %d %s : %v != %v (%v)", step, operator, got, want, err)
			}
		}
	}
}

func TestTypesDuplicatesAndKeys(t *testing.T) {
	idx := New(2)
	for key, text := range map[string]string{"a": "2", "b": "2.0", "c": "-0", "d": "0"} {
		if err := idx.Set(key, text, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := idx.Set("text", "2", false); err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		field, op, value string
		number           bool
		want             []string
	}{
		{"value", "equals", "2", true, []string{"a", "b"}},
		{"value", "equals", "2", false, []string{"text"}},
		{"value", "equals", "0", true, []string{"c", "d"}},
		{"value", "contains", "2", false, []string{"text"}},
		{"key", ">", "b", false, []string{"c", "d", "text"}},
		{"key", "<=", "b", false, []string{"a", "b"}},
	}
	for _, check := range checks {
		got, err := idx.Find(check.field, check.op, check.value, check.number)
		if err != nil || !reflect.DeepEqual(got, check.want) {
			t.Fatal(check, got, err)
		}
	}
	idx.Delete("a")
	idx.Delete("b")
	got, err := idx.Find("value", "equals", "2", true)
	if err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	if idx.values.Len() != 2 {
		t.Fatal("valeurs supprimées conservées dans le B-Tree")
	}
}
