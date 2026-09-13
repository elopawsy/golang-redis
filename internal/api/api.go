package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"wasmredis/internal/engine"
)

func New(queues *engine.Queues, kv *engine.Engine) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/queues", func(w http.ResponseWriter, r *http.Request) {
		write(w, http.StatusOK, queues.List())
	})
	mux.HandleFunc("POST /api/queues", func(w http.ResponseWriter, r *http.Request) {
		var body queueRequest
		if !read(w, r, &body) {
			return
		}
		if err := queues.Create(body.Name); err != nil {
			fail(w, err)
			return
		}
		write(w, http.StatusCreated, map[string]string{"name": body.Name})
	})
	mux.HandleFunc("GET /api/queues/{name}/tasks", func(w http.ResponseWriter, r *http.Request) {
		offset, err := number(r, "offset", 0)
		if err != nil {
			fail(w, engine.ErrInvalid)
			return
		}
		limit, err := number(r, "limit", 100)
		if err != nil {
			fail(w, engine.ErrInvalid)
			return
		}
		page, err := queues.Page(r.PathValue("name"), r.URL.Query().Get("status"), offset, limit)
		if err != nil {
			fail(w, err)
			return
		}
		write(w, http.StatusOK, page)
	})
	mux.HandleFunc("POST /api/queues/{name}/tasks", func(w http.ResponseWriter, r *http.Request) {
		var body taskRequest
		if !read(w, r, &body) {
			return
		}
		task, err := queues.Add(r.PathValue("name"), body.Payload)
		if err != nil {
			fail(w, err)
			return
		}
		write(w, http.StatusCreated, task)
	})
	mux.HandleFunc("POST /api/queues/{name}/claim", func(w http.ResponseWriter, r *http.Request) {
		task, err := queues.Claim(r.PathValue("name"))
		if err != nil {
			fail(w, err)
			return
		}
		write(w, http.StatusOK, task)
	})
	mux.HandleFunc("PATCH /api/queues/{name}/tasks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil || id <= 0 {
			fail(w, engine.ErrInvalid)
			return
		}
		var body statusRequest
		if !read(w, r, &body) {
			return
		}
		task, err := queues.Update(r.PathValue("name"), id, body.Status)
		if err != nil {
			fail(w, err)
			return
		}
		write(w, http.StatusOK, task)
	})
	mux.HandleFunc("POST /api/command", func(w http.ResponseWriter, r *http.Request) {
		var body commandRequest
		if !read(w, r, &body) {
			return
		}
		result := kv.ExecuteString(body.Command)
		if result.Err != nil {
			write(w, http.StatusBadRequest, map[string]string{"error": result.Err.Error()})
			return
		}
		write(w, http.StatusOK, response(result))
	})
	mux.HandleFunc("POST /api/batch", func(w http.ResponseWriter, r *http.Request) {
		var body batchRequest
		if !read(w, r, &body) {
			return
		}
		if body.Commands == nil || len(body.Commands) > 1000 {
			fail(w, engine.ErrInvalid)
			return
		}
		results := kv.ExecuteBatchStrings(body.Commands)
		responses := make([]commandResponse, len(results))
		for i, result := range results {
			responses[i] = response(result)
		}
		write(w, http.StatusOK, responses)
	})
	mux.HandleFunc("POST /api/flush", func(w http.ResponseWriter, r *http.Request) {
		if err := kv.Flush(); err != nil {
			write(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		write(w, http.StatusOK, map[string]string{"value": "OK"})
	})
	mux.HandleFunc("POST /api/snapshot", func(w http.ResponseWriter, r *http.Request) {
		if err := kv.Snapshot(); err != nil {
			write(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		write(w, http.StatusOK, map[string]string{"value": "OK"})
	})
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		write(w, http.StatusNotFound, map[string]string{"error": "route inconnue"})
	})
	mux.Handle("/", http.FileServer(http.Dir("web/dist")))
	return mux
}

func response(result engine.Result) commandResponse {
	value := commandResponse{Value: result.Value, IsNumber: result.IsNumber, Entries: result.Entries}
	if result.Err != nil {
		value.Error = result.Err.Error()
	}
	return value
}

func number(r *http.Request, key string, fallback int) (int, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback, nil
	}
	return strconv.Atoi(value)
}

func read(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		fail(w, engine.ErrInvalid)
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		fail(w, engine.ErrInvalid)
		return false
	}
	return true
}

func fail(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, engine.ErrNotFound) {
		status = http.StatusNotFound
	}
	if errors.Is(err, engine.ErrConflict) {
		status = http.StatusConflict
	}
	write(w, status, map[string]string{"error": err.Error()})
}

func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
