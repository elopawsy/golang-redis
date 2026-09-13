package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"wasmredis/internal/config"
	"wasmredis/internal/engine"
	"wasmredis/internal/storage"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() (result error) {
	cli := flag.Bool("cli", false, "ouvrir le terminal clé-valeur")
	addr := flag.String("addr", "127.0.0.1:8080", "adresse du serveur HTTP")
	demo := flag.Bool("demo", false, "créer 3000 tâches de démonstration")
	memory := flag.Bool("memory", false, "désactiver la persistance clé-valeur")
	flag.Parse()
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return err
	}
	var eng *engine.Engine
	var err error
	if *memory {
		eng, err = engine.NewMemory(cfg)
	} else {
		unlock, lockErr := storage.LockFiles(cfg.AOFPath, cfg.SnapshotPath)
		if lockErr != nil {
			return lockErr
		}
		defer func() { result = errors.Join(result, unlock()) }()
		eng, err = engine.NewWithStorage(storage.NewFileStorage(cfg.AOFPath, cfg.SnapshotPath), cfg)
	}
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, eng.Close()) }()
	signals, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithCancel(signals)
	defer cancel()
	background := make(chan error, 1)
	go func() { background <- eng.StartBackground(ctx); cancel() }()
	if *cli {
		result = runCLI(ctx, eng, *memory)
	} else {
		result = runServer(ctx, *addr, *demo, eng)
	}
	cancel()
	return errors.Join(result, <-background)
}
