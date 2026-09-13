package command

import "testing"

func TestTypedParsing(t *testing.T) {
	for _, input := range []string{"SET k 42", "SET k -2.5", "SET k 1e3"} {
		cmd, err := Parse(input)
		if err != nil || !cmd.IsNumber {
			t.Fatal(input, cmd, err)
		}
	}
	for _, input := range []string{`SET k "42"`, `SET k '42'`, "SET k NaN", "SET k Inf", "SET k 1e999"} {
		cmd, err := Parse(input)
		if err != nil || cmd.IsNumber {
			t.Fatal(input, cmd, err)
		}
	}
	cmd, err := Parse("get where value >= 21")
	if err != nil || cmd.Kind != KindGetWhere || !cmd.OperandIsNumber || cmd.Operator != ">=" {
		t.Fatal(cmd, err)
	}
	cmd, err = Parse(`GET WHERE value equals "21"`)
	if err != nil || cmd.OperandIsNumber {
		t.Fatal(cmd, err)
	}
	for _, input := range []string{"GET WHERE value", "GET WHERE bad equals a", "GET WHERE value wrong a", "GET WHERE key > a extra"} {
		if _, err := Parse(input); err == nil {
			t.Fatal(input)
		}
	}
}
