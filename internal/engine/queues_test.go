package engine

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestQueueLifecycle(t *testing.T) {
	q := NewQueues()
	if err := q.Create("emails"); err != nil {
		t.Fatal(err)
	}
	first, err := q.Add("emails", "premier")
	if err != nil {
		t.Fatal(err)
	}
	second, err := q.Add("emails", "second")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.Update("emails", first.ID, Completed); !errors.Is(err, ErrConflict) {
		t.Fatal("une tâche en attente ne peut pas être terminée", err)
	}
	claimed, err := q.Claim("emails")
	if err != nil || claimed.ID != first.ID || claimed.Status != Running {
		t.Fatalf("FIFO : %+v, %v", claimed, err)
	}
	if _, err := q.Update("emails", first.ID, Failed); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Update("emails", first.ID, Pending); err != nil {
		t.Fatal(err)
	}
	claimed, err = q.Claim("emails")
	if err != nil || claimed.ID != first.ID {
		t.Fatalf("la relance conserve la place d'origine : %+v %v", claimed, err)
	}
	if _, err := q.Update("emails", first.ID, Completed); err != nil {
		t.Fatal(err)
	}
	claimed, err = q.Claim("emails")
	if err != nil || claimed.ID != second.ID {
		t.Fatalf("seconde tâche : %+v, %v", claimed, err)
	}
	if _, err := q.Claim("emails"); !errors.Is(err, ErrConflict) {
		t.Fatal("file sans tâche en attente", err)
	}
	summary := q.List()[0]
	if summary.Total != 2 || summary.Counts[Completed] != 1 || summary.Counts[Running] != 1 {
		t.Fatalf("compteurs : %+v", summary)
	}
	summary.Counts[Completed] = 99
	if q.List()[0].Counts[Completed] != 1 {
		t.Fatal("la réponse ne doit pas partager l'état interne")
	}
}

func TestQueuePaginationAndValidation(t *testing.T) {
	q := NewQueues()
	for _, name := range []string{"", "a/b", "avec espace"} {
		if !errors.Is(q.Create(name), ErrInvalid) {
			t.Fatalf("nom accepté : %q", name)
		}
	}
	if err := q.Create("queue"); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(q.Create("queue"), ErrConflict) {
		t.Fatal("doublon accepté")
	}
	if _, err := q.Add("missing", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := q.Add("queue", " "); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	for i := 0; i < 250; i++ {
		if _, err := q.Add("queue", fmt.Sprint(i)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := q.Claim("queue"); err != nil {
		t.Fatal(err)
	}
	page, err := q.Page("queue", Pending, 100, 100)
	if err != nil || page.Total != 249 || len(page.Items) != 100 || page.Items[0].Payload != "101" {
		t.Fatalf("page incorrecte : %+v, %v", page, err)
	}
	page.Items[0].Payload = "modifié"
	again, _ := q.Page("queue", Pending, 100, 1)
	if again.Items[0].Payload != "101" {
		t.Fatal("la page modifie l'état")
	}
	page, err = q.Page("queue", "", 10000, 100)
	if err != nil || len(page.Items) != 0 || page.Total != 250 {
		t.Fatal("offset au-delà de la fin", err)
	}
	for _, args := range [][2]int{{-1, 10}, {0, 0}, {0, 201}} {
		if _, err := q.Page("queue", "", args[0], args[1]); !errors.Is(err, ErrInvalid) {
			t.Fatal("pagination invalide acceptée")
		}
	}
	if _, err := q.Page("queue", "invalid", 0, 10); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestConcurrentClaims(t *testing.T) {
	q := NewQueues()
	if err := q.Create("q"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		if _, err := q.Add("q", "task"); err != nil {
			t.Fatal(err)
		}
	}
	ids := make(chan int, 100)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, err := q.Claim("q")
			if err != nil {
				t.Error(err)
				return
			}
			ids <- task.ID
			q.List()
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[int]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatalf("tâche %d démarrée deux fois", id)
		}
		seen[id] = true
	}
	if len(seen) != 100 {
		t.Fatalf("%d tâches démarrées", len(seen))
	}
}
