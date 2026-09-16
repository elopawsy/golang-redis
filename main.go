package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
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

func plural(n int) string {
	if n > 1 {
		return "s"
	}
	return ""
}

func run() (result error) {
	cli := flag.Bool("cli", false, "ouvrir le terminal clé-valeur")
	addr := flag.String("addr", "127.0.0.1:8080", "adresse du serveur HTTP")
	demo := flag.Bool("demo", false, "créer trois files de démonstration")
	demoTasks := flag.Int("demo-tasks", 1000, "nombre de tâches par file de démonstration")
	memory := flag.Bool("memory", false, "désactiver la persistance clé-valeur")
	flag.Parse()
	if *demo && *demoTasks < 1 {
		return errors.New("nombre de tâches de démonstration invalide")
	}
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return err
	}
	log.Printf("réglages : flush %s, snapshot %s, expiration %s, TTL par défaut %s, degré B-Tree %d",
		cfg.FlushInterval, cfg.SnapshotInterval, cfg.ExpiryScanInterval, cfg.DefaultTTL, cfg.BTreeOrder)
	var eng *engine.Engine
	var err error
	if *memory {
		log.Print("stockage : mémoire seule, aucune écriture disque")
		eng, err = engine.NewMemory(cfg)
	} else {
		log.Printf("stockage : journal %s, snapshot %s", cfg.AOFPath, cfg.SnapshotPath)
		unlock, lockErr := storage.LockFiles(cfg.AOFPath, cfg.SnapshotPath)
		if lockErr != nil {
			return lockErr
		}
		log.Print("verrous de fichiers acquis")
		defer func() {
			result = errors.Join(result, unlock())
			log.Print("verrous de fichiers libérés")
		}()
		started := time.Now()
		eng, err = engine.NewWithStorage(storage.NewFileStorage(cfg.AOFPath, cfg.SnapshotPath), cfg)
		if err == nil {
			log.Printf("restauration : %d clé%s en %s", eng.Len(), plural(eng.Len()), time.Since(started).Round(time.Millisecond))
		}
	}
	if err != nil {
		return err
	}
	defer func() {
		buffered := eng.Buffered()
		if closeErr := eng.Close(); closeErr != nil {
			log.Printf("dernier flush impossible : %d opération%s encore en tampon", buffered, plural(buffered))
			result = errors.Join(result, closeErr)
			return
		}
		log.Printf("moteur fermé : %d opération%s écrite%s au dernier flush", buffered, plural(buffered), plural(buffered))
	}()
	stopping := make(chan os.Signal, 1)
	signal.Notify(stopping, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stopping)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case received := <-stopping:
			log.Printf("signal %s reçu : arrêt propre, écriture du tampon avant la sortie", received)
			cancel()
		case <-ctx.Done():
		}
	}()
	background := make(chan error, 1)
	go func() { background <- eng.StartBackground(ctx); cancel() }()
	if *cli {
		result = runCLI(ctx, eng, *memory)
	} else {
		result = runServer(ctx, *addr, *demo, *demoTasks, eng)
	}
	cancel()
	return errors.Join(result, <-background)
}
