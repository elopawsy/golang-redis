package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"wasmredis/internal/engine"
)

func runCLI(ctx context.Context, eng *engine.Engine, memory bool) error {
	fmt.Println("WasmRedis — SET, GET, DEL, PING, GET WHERE.")
	if memory {
		fmt.Println("Mode mémoire : données perdues à la fermeture.")
	} else {
		fmt.Println("Persistance active : journal, snapshots et restauration.")
	}
	fmt.Println("Quitter : EXIT, QUIT ou Ctrl-D.")
	lines := make(chan string)
	readError := make(chan error, 1)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
		readError <- scanner.Err()
	}()
	for {
		fmt.Print("> ")
		select {
		case <-ctx.Done():
			return nil
		case line, ok := <-lines:
			if !ok {
				return <-readError
			}
			line = strings.TrimSpace(line)
			if strings.EqualFold(line, "EXIT") || strings.EqualFold(line, "QUIT") {
				return nil
			}
			if line == "" {
				continue
			}
			result := eng.ExecuteString(line)
			if result.Err != nil {
				fmt.Println("erreur:", result.Err)
				continue
			}
			if result.Entries != nil {
				data, err := json.Marshal(result.Entries)
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			} else {
				fmt.Println(result.Value)
			}
		}
	}
}
