package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
	"wasmredis/internal/api"
	"wasmredis/internal/engine"
)

func runServer(ctx context.Context, addr string, demo bool, demoTasks int, eng *engine.Engine) error {
	queues := engine.NewQueues()
	if demo {
		started := time.Now()
		if err := seedDemo(queues, demoTasks); err != nil {
			return err
		}
		log.Printf("démonstration : 3 files de %d tâche%s créées en %s",
			demoTasks, plural(demoTasks), time.Since(started).Round(time.Millisecond))
	}
	server := &http.Server{
		Addr: addr, Handler: api.New(queues, eng),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second, IdleTimeout: time.Minute,
	}
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	log.Printf("WasmRedis : http://%s — files de tâches en mémoire", addr)
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		log.Print("fermeture du serveur HTTP")
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := server.Shutdown(shutdown)
		if err != nil {
			err = errors.Join(err, server.Close())
		}
		serveErr := <-done
		if !errors.Is(serveErr, http.ErrServerClosed) {
			err = errors.Join(err, serveErr)
		}
		log.Print("serveur HTTP arrêté")
		return err
	}
}

func seedDemo(q *engine.Queues, tasks int) error {
	for _, name := range []string{"emails", "images", "exports"} {
		if err := q.Create(name); err != nil {
			return err
		}
		for i := 0; i < tasks; i++ {
			if _, err := q.Add(name, fmt.Sprintf("Tâche %s %04d", name, i+1)); err != nil {
				return err
			}
		}
		claims := 30
		if claims > tasks {
			claims = tasks
		}
		for i := 0; i < claims; i++ {
			task, err := q.Claim(name)
			if err != nil {
				return err
			}
			if i < 25 {
				status := engine.Completed
				if i%5 == 0 {
					status = engine.Failed
				}
				if _, err := q.Update(name, task.ID, status); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
