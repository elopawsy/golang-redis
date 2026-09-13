package engine

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalid  = errors.New("données invalides")
	ErrConflict = errors.New("opération impossible dans cet état")
)

func NewQueues() *Queues {
	return &Queues{queues: make(map[string][]Task)}
}

func (q *Queues) Create(name string) error {
	if len(name) == 0 || len(name) > 64 || strings.Trim(name, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_") != "" {
		return fmt.Errorf("%w : nom de 1 à 64 lettres, chiffres, tirets ou underscores", ErrInvalid)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, exists := q.queues[name]; exists {
		return fmt.Errorf("%w : cette file existe déjà", ErrConflict)
	}
	q.queues[name] = []Task{}
	return nil
}

func (q *Queues) List() []QueueSummary {
	q.mu.Lock()
	defer q.mu.Unlock()
	result := make([]QueueSummary, 0, len(q.queues))
	for name, tasks := range q.queues {
		counts := map[string]int{Pending: 0, Running: 0, Completed: 0, Failed: 0}
		for _, task := range tasks {
			counts[task.Status]++
		}
		result = append(result, QueueSummary{Name: name, Total: len(tasks), Counts: counts})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (q *Queues) Add(name, payload string) (Task, error) {
	if strings.TrimSpace(payload) == "" || len(payload) > 4096 {
		return Task{}, fmt.Errorf("%w : contenu requis, 4096 octets maximum", ErrInvalid)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, exists := q.queues[name]; !exists {
		return Task{}, ErrNotFound
	}
	q.nextID++
	task := Task{ID: q.nextID, Payload: payload, Status: Pending, CreatedAt: time.Now()}
	q.queues[name] = append(q.queues[name], task)
	return task, nil
}

func ValidStatus(status string) bool {
	return status == Pending || status == Running || status == Completed || status == Failed
}

func (q *Queues) Page(name, status string, offset, limit int) (TaskPage, error) {
	if offset < 0 || limit < 1 || limit > 200 || (status != "" && !ValidStatus(status)) {
		return TaskPage{}, ErrInvalid
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	tasks, exists := q.queues[name]
	if !exists {
		return TaskPage{}, ErrNotFound
	}
	page := TaskPage{Items: []Task{}, Offset: offset}
	for _, task := range tasks {
		if status != "" && task.Status != status {
			continue
		}
		if page.Total >= offset && len(page.Items) < limit {
			page.Items = append(page.Items, task)
		}
		page.Total++
	}
	return page, nil
}

func (q *Queues) Claim(name string) (Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	tasks, exists := q.queues[name]
	if !exists {
		return Task{}, ErrNotFound
	}
	for i := range tasks {
		if tasks[i].Status == Pending {
			tasks[i].Status = Running
			return tasks[i], nil
		}
	}
	return Task{}, fmt.Errorf("%w : aucune tâche en attente", ErrConflict)
}

func (q *Queues) Update(name string, id int, status string) (Task, error) {
	if !ValidStatus(status) {
		return Task{}, ErrInvalid
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	tasks, exists := q.queues[name]
	if !exists {
		return Task{}, ErrNotFound
	}
	for i := range tasks {
		if tasks[i].ID != id {
			continue
		}
		current := tasks[i].Status
		if !(current == Running && (status == Completed || status == Failed) || current == Failed && status == Pending) {
			return Task{}, ErrConflict
		}
		tasks[i].Status = status
		return tasks[i], nil
	}
	return Task{}, ErrNotFound
}
