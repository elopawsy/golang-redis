package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"wasmredis/internal/engine"
)

func TestBatchAndTypedQueryHTTP(t *testing.T) {
	handler := New(engine.NewQueues(), engine.New())
	call := func(path, body string) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("POST", path, strings.NewReader(body)))
		if response.Code != 200 {
			t.Fatal(response.Code, response.Body.String())
		}
		return response
	}
	result := call("/api/batch", `{"commands":["SET age 42","SET label \"42\"","UNKNOWN","GET WHERE value >= 21"]}`)
	var batch []commandResponse
	if err := json.Unmarshal(result.Body.Bytes(), &batch); err != nil {
		t.Fatal(err)
	}
	if len(batch) != 4 || batch[2].Error == "" || len(batch[3].Entries) != 1 || batch[3].Entries[0].Key != "age" {
		t.Fatal(batch)
	}
	result = call("/api/command", `{"command":"GET WHERE value > 100"}`)
	var query commandResponse
	if err := json.Unmarshal(result.Body.Bytes(), &query); err != nil {
		t.Fatal(err)
	}
	if query.Entries == nil || len(query.Entries) != 0 {
		t.Fatal(query)
	}
	call("/api/flush", "")
	call("/api/snapshot", "")
}
