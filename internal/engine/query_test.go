package engine

import (
	"testing"
	"time"
	"wasmredis/internal/config"
)

func TestTypedQueriesAndIndexMaintenance(t *testing.T) {
	store := &memoryStorage{}
	e, err := NewWithStorage(store, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	e.now = func() time.Time { return now }
	for _, input := range []string{"SET age 42", `SET label "42"`, "SET low 2", "SET ttl 50 EX 1"} {
		if result := e.ExecuteString(input); result.Err != nil {
			t.Fatal(result.Err)
		}
	}
	checks := []struct {
		input string
		count int
	}{
		{"GET WHERE value >= 21", 2},
		{`GET WHERE value equals "42"`, 1},
		{"GET WHERE value equals 42", 1},
		{"GET WHERE key contains a", 2},
	}
	for _, check := range checks {
		result := e.ExecuteString(check.input)
		if result.Err != nil || len(result.Entries) != check.count {
			t.Fatal(check, result)
		}
	}
	if result := e.ExecuteString("SET age young"); result.Err != nil {
		t.Fatal(result.Err)
	}
	if result := e.ExecuteString("GET WHERE value >= 21"); result.Err != nil || len(result.Entries) != 1 {
		t.Fatal(result)
	}
	now = now.Add(time.Second)
	if result := e.ExecuteString("GET WHERE value >= 21"); result.Err != nil || len(result.Entries) != 0 {
		t.Fatal(result)
	}
	if err := e.Delete("label"); err != nil {
		t.Fatal(err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	restored, err := NewWithStorage(store, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	result := restored.ExecuteString("GET WHERE value equals young")
	if result.Err != nil || len(result.Entries) != 1 || result.Entries[0].Key != "age" {
		t.Fatal(result)
	}
	result = restored.ExecuteString("GET WHERE value >= 21")
	if result.Err != nil || len(result.Entries) != 0 {
		t.Fatal(result)
	}
}
