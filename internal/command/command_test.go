package command

import (
	"errors"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input string
		want  Command
	}{
		{`SET name Ada`, Command{Kind: KindSet, Key: "name", Value: "Ada"}},
		{` set name "Ada Lovelace" `, Command{Kind: KindSet, Key: "name", Value: "Ada Lovelace"}},
		{`SET name 'Ada Lovelace'`, Command{Kind: KindSet, Key: "name", Value: "Ada Lovelace"}},
		{`SET name ""`, Command{Kind: KindSet, Key: "name"}},
		{`SET "full name" Ada`, Command{Kind: KindSet, Key: "full name", Value: "Ada"}},
		{`SET clé été ex 10`, Command{Kind: KindSet, Key: "clé", Value: "été", TTL: 10 * time.Second}},
		{`GET name`, Command{Kind: KindGet, Key: "name"}},
		{`del name`, Command{Kind: KindDelete, Key: "name"}},
		{`DELETE name`, Command{Kind: KindDelete, Key: "name"}},
		{`ping`, Command{Kind: KindPing}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Parse(tt.input)
			if err != nil || got != tt.want {
				t.Fatalf("Parse() = %+v, %v ; attendu %+v", got, err, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	for _, input := range []string{
		"", "  ", "UNKNOWN", "SET", "SET k", "SET k v extra", "GET", "GET k extra",
		"DELETE", "DEL k extra", "PING extra", `SET k "non fermé`,
		"SET k v EX", "SET k v PX 2", "SET k v EX 0", "SET k v EX -1",
		"SET k v EX 1.5", "SET k v EX abc", `SET k v EX ""`,
		"SET k v EX 9999999999999999999999999", "SET k v EX 1 extra",
	} {
		t.Run(input, func(t *testing.T) {
			if _, err := Parse(input); err == nil {
				t.Fatal("une erreur était attendue")
			}
		})
	}
	if _, err := Parse("unknown"); !errors.Is(err, ErrUnknown) {
		t.Fatalf("erreur inattendue : %v", err)
	}
}
