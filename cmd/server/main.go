package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3"

	config "tenant-provisioning-control-plane/internal/config"
	migrate "tenant-provisioning-control-plane/internal/migrate"
	model "tenant-provisioning-control-plane/internal/model"
	store "tenant-provisioning-control-plane/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := sql.Open("sqlite3", cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := migrate.RunMigrations(db, "migrations"); err != nil {
		log.Fatalf("migration: %v", err)
	}

	store.NewTenantRepository(db)
	store.NewTaskRepository(db)

	fmt.Println(model.NowUTC())

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Server starting on %s (db=%s, nats=%s)", addr, cfg.DBPath, cfg.NATSURL)

	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down")
}

