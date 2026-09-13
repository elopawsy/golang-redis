package api

import "wasmredis/internal/engine"

type queueRequest struct {
	Name string `json:"name"`
}

type taskRequest struct {
	Payload string `json:"payload"`
}

type statusRequest struct {
	Status string `json:"status"`
}

type commandRequest struct {
	Command string `json:"command"`
}

type batchRequest struct {
	Commands []string `json:"commands"`
}

type commandResponse struct {
	Value    string         `json:"value"`
	IsNumber bool           `json:"isNumber"`
	Entries  []engine.Entry `json:"entries"`
	Error    string         `json:"error,omitempty"`
}
