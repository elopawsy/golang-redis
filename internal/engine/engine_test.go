package engine

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCommands(t *testing.T) {
	e := New()
	steps := []struct {
		input string
		value string
		err   error
	}{
		{"PING", "PONG", nil},
		{`SET name "Ada Lovelace"`, "OK", nil},
		{"GET name", "Ada Lovelace", nil},
		{"SET name Grace", "OK", nil},
		{"GET name", "Grace", nil},
		{"DEL name", "OK", nil},
		{"GET name", "", ErrNotFound},
		{"DELETE absent", "OK", nil},
		{`SET empty ""`, "OK", nil},
		{"GET empty", "", nil},
	}
	for _, step := range steps {
		result := e.ExecuteString(step.input)
		if result.Value != step.value || !errors.Is(result.Err, step.err) {
			t.Fatalf("%s : obtenu %+v ; attendu %q, %v", step.input, result, step.value, step.err)
		}
	}
}

func TestExpiration(t *testing.T) {
	e := New()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	e.now = func() time.Time { return now }
	if result := e.ExecuteString("SET code secret EX 10"); result.Err != nil {
		t.Fatal(result.Err)
	}
	now = now.Add(9 * time.Second)
	if value, err := e.Get("code"); err != nil || value != "secret" {
		t.Fatalf("expiration trop tôt : %q, %v", value, err)
	}
	now = now.Add(time.Second)
	if _, err := e.Get("code"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("la clé doit expirer exactement à son échéance : %v", err)
	}
	if _, exists := e.state["code"]; exists {
		t.Fatal("la clé expirée doit être retirée de la map")
	}
}

func TestSetReplacesTTL(t *testing.T) {
	e := New()
	now := time.Now()
	e.now = func() time.Time { return now }
	for _, input := range []string{"SET k old EX 1", "SET k new"} {
		if result := e.ExecuteString(input); result.Err != nil {
			t.Fatal(result.Err)
		}
	}
	now = now.Add(time.Hour)
	if value, err := e.Get("k"); value != "new" || err != nil {
		t.Fatalf("SET sans EX doit retirer l'ancien TTL : %q, %v", value, err)
	}
}

func TestInvalidSetKeepsOldValue(t *testing.T) {
	e := New()
	if err := e.Set("k", "old", 0); err != nil {
		t.Fatal(err)
	}
	if result := e.ExecuteString("SET k new EX -1"); result.Err == nil {
		t.Fatal("la commande doit échouer")
	}
	if err := e.Set("k", "new", -time.Second); err == nil {
		t.Fatal("un TTL négatif doit être refusé même sans parser")
	}
	if value, err := e.Get("k"); value != "old" || err != nil {
		t.Fatalf("une erreur ne doit pas modifier la valeur : %q, %v", value, err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	e := New()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			if err := e.Set(key, "value", 0); err != nil {
				t.Error(err)
			}
			if value, err := e.Get(key); value != "value" || err != nil {
				t.Errorf("Get() = %q, %v", value, err)
			}
			if err := e.Delete(key); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}
