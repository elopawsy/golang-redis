package engine

import "wasmredis/internal/command"

func (e *Engine) ExecuteBatch(commands []command.Command) []Result {
	results := make([]Result, len(commands))
	for i, cmd := range commands {
		results[i] = e.Execute(cmd)
	}
	return results
}

func (e *Engine) ExecuteBatchStrings(lines []string) []Result {
	results := make([]Result, len(lines))
	for i, line := range lines {
		results[i] = e.ExecuteString(line)
	}
	return results
}
