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
	return &Queues{queues: make(map[string]*queue)}
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
	q.queues[name] = &queue{
		tasks:  []Task{},
		counts: map[string]int{Pending: 0, Running: 0, Completed: 0, Failed: 0},
	}
	return nil
}

func (q *Queues) List() []QueueSummary {
	q.mu.Lock()
	defer q.mu.Unlock()
	result := make([]QueueSummary, 0, len(q.queues))
	for name, entry := range q.queues {
		counts := make(map[string]int, len(entry.counts))
		for status, total := range entry.counts {
			counts[status] = total
		}
		result = append(result, QueueSummary{Name: name, Total: len(entry.tasks), Counts: counts})
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
	entry, exists := q.queues[name]
	if !exists {
		return Task{}, ErrNotFound
	}
	q.nextID++
	task := Task{ID: q.nextID, Payload: payload, Status: Pending, CreatedAt: time.Now()}
	entry.tasks = append(entry.tasks, task)
	entry.counts[Pending]++
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
	entry, exists := q.queues[name]
	if !exists {
		return TaskPage{}, ErrNotFound
	}
	page := TaskPage{Items: []Task{}, Offset: offset}
	if status == "" {
		page.Total = len(entry.tasks)
		for i := offset; i < len(entry.tasks) && len(page.Items) < limit; i++ {
			page.Items = append(page.Items, entry.tasks[i])
		}
		return page, nil
	}
	page.Total = entry.counts[status]
	seen := 0
	for _, task := range entry.tasks {
		if task.Status != status {
			continue
		}
		if seen >= offset {
			page.Items = append(page.Items, task)
		}
		seen++
		if len(page.Items) == limit {
			break
		}
	}
	return page, nil
}

func (q *Queues) Claim(name string) (Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	entry, exists := q.queues[name]
	if !exists {
		return Task{}, ErrNotFound
	}
	for i := range entry.tasks {
		if entry.tasks[i].Status == Pending {
			entry.tasks[i].Status = Running
			entry.counts[Pending]--
			entry.counts[Running]++
			return entry.tasks[i], nil
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
	entry, exists := q.queues[name]
	if !exists {
		return Task{}, ErrNotFound
	}
	for i := range entry.tasks {
		if entry.tasks[i].ID != id {
			continue
		}
		current := entry.tasks[i].Status
		if !(current == Running && (status == Completed || status == Failed) || current == Failed && status == Pending) {
			return Task{}, ErrConflict
		}
		entry.tasks[i].Status = status
		entry.counts[current]--
		entry.counts[status]++
		return entry.tasks[i], nil
	}
	return Task{}, ErrNotFound
}
