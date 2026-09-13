package engine

import (
	"testing"
	"wasmredis/internal/command"
)

func TestBatchContinuesAfterError(t *testing.T) {
	e := New()
	results := e.ExecuteBatch([]command.Command{
		{Kind: command.KindSet, Key: "k", Value: "first"},
		{Kind: "unknown"},
		{Kind: command.KindSet, Key: "k", Value: "last"},
		{Kind: command.KindGet, Key: "k"},
	})
	if len(results) != 4 || results[0].Err != nil || results[1].Err == nil || results[3].Value != "last" {
		t.Fatal(results)
	}
	results = e.ExecuteBatchStrings([]string{"SET k next", "SET", "GET k"})
	if len(results) != 3 || results[1].Err == nil || results[2].Value != "next" {
		t.Fatal(results)
	}
}
