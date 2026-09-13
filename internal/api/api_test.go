package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"wasmredis/internal/engine"
)

func TestHTTPWorkflow(t *testing.T) {
	handler := New(engine.NewQueues(), engine.New())
	call := func(method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(body)))
		if response.Code != status {
			t.Fatalf("%s %s : %d %s (attendu %d)", method, path, response.Code, response.Body.String(), status)
		}
		return response
	}
	if body := call("GET", "/api/queues", "", 200).Body.String(); strings.TrimSpace(body) != "[]" {
		t.Fatal(body)
	}
	call("POST", "/api/queues", `{"name":"emails"}`, 201)
	call("POST", "/api/queues", `{"name":"emails"}`, 409)
	call("POST", "/api/queues", `{"name":"x","unknown":true}`, 400)
	call("POST", "/api/queues", `{"name":"x"} {}`, 400)
	call("POST", "/api/queues", `{`, 400)
	call("POST", "/api/queues", strings.Repeat(" ", 17000), 400)
	call("POST", "/api/queues/missing/tasks", `{"payload":"hello"}`, 404)
	call("POST", "/api/queues/emails/tasks", `{"payload":"hello"}`, 201)
	pageResponse := call("GET", "/api/queues/emails/tasks?offset=0&limit=20", "", 200)
	var page engine.TaskPage
	if err := json.Unmarshal(pageResponse.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].Payload != "hello" {
		t.Fatalf("page : %+v", page)
	}
	call("PATCH", "/api/queues/emails/tasks/1", `{"status":"completed"}`, 409)
	call("POST", "/api/queues/emails/claim", "", 200)
	call("PATCH", "/api/queues/emails/tasks/1", `{"status":"completed"}`, 200)
	call("POST", "/api/queues/emails/claim", "", 409)
	for _, query := range []string{"offset=-1", "offset=abc", "limit=201", "limit=0", "status=no"} {
		call("GET", "/api/queues/emails/tasks?"+query, "", http.StatusBadRequest)
	}
	call("PATCH", "/api/queues/emails/tasks/no", `{"status":"failed"}`, 400)
	call("PATCH", "/api/queues/emails/tasks/999", `{"status":"failed"}`, 404)
	call("GET", "/api/no", "", 404)
	call("POST", "/api/command", `{"command":"SET name Ada"}`, 200)
	if body := call("POST", "/api/command", `{"command":"GET name"}`, 200).Body.String(); !strings.Contains(body, "Ada") {
		t.Fatal(body)
	}
	call("POST", "/api/command", `{"command":"UNKNOWN"}`, 400)
}
