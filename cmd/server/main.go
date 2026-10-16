// Package main bootstraps HTTP server, NATS outbox dispatcher (GAP 1+6) and consumer (GAP 2).
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	nats "github.com/nats-io/nats.go"
	js "github.com/nats-io/nats.go/jetstream"

	cfg "tenant-provisioning-control-plane/internal/config"
	migratepkg "tenant-provisioning-control-plane/internal/migrate"
	models "tenant-provisioning-control-plane/internal/model"
	natspkg  "tenant-provisioning-control-plane/internal/nats" // directory → package natspublisher
	storesp  "tenant-provisioning-control-plane/internal/store"
	routerp  "tenant-provisioning-control-plane/router"
	consumerp "tenant-provisioning-control-plane/internal/consumer"
	lcipkg   "tenant-provisioning-control-plane/internal/lifecycle"
)

func main() {
	conf, err := cfg.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := sql.Open("sqlite3", conf.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(context.Background(), "PRAGMA journal_mode=WAL"); err != nil {
		log.Printf("db warn: WAL not available: %v", err)
	}

	if err := migratepkg.RunMigrations(db, "migrations"); err != nil {
		log.Fatalf("migration: %v", err)
	}

	fmt.Println(models.NowUTC())

	r := routerp.New(db, os.Stdout)
	addr := fmt.Sprintf(":%d", conf.Port)
	log.Printf("Server starting on %s (db=%s, nats=%s)", addr, conf.DBPath, conf.NATSURL)

	// ── Connect NATS once; share between publisher and consumer. ────
	nc, err := connectWithDelay(conf.NATSURL, 5*time.Second, 10)
	if err != nil {
		log.Fatalf("[nats] connect: %v", err)
	}

	jsCtx, err := js.New(nc)
	if err != nil {
		log.Fatalf("[nats] jetstream new: %v", err)
	}

	streamName := "TASKS_CREATE"
	_, _ = jsCtx.CreateStream(context.Background(), js.StreamConfig{
		Name:     streamName,
		Subjects: []string{"tasks.create.>", "tasks.progress.>"},
	})
	log.Printf("[nats] stream %s ready", streamName)

	// ── GAP 1+6: outbound dispatcher (outbox → NATS publish). ──────
	fetcher := &outbxFetchAdapter{db: db} // implements natspkg.Fetcher
	pub, err := natspkg.New(natspkg.Config{
		JetStream:  jsCtx,
		StreamName: streamName,
		Limit:      50, Interval: 2 * time.Second,
		Retry:  lcipkg.DefaultRetryPolicy(), // GAP 6 + T24
		Fetcher: fetcher,
		Log:     log.New(os.Stderr, "[publisher] ", log.LstdFlags),
	})
	if err != nil {
		log.Fatalf("[publisher] new: %v", err)
	}

	ctxPub, cancelPub := context.WithCancel(context.Background())
	go pub.Run(ctxPub)
	pubCh := make(chan string, 1)
	go logStats(pub, pubCh)
	<-pubCh // fire initial log.
	log.Printf("[publisher] start interval=%v stream=%s", 2*time.Second, streamName)

	// ── GAP 2: inbound consumer (tasks.progress.→ DB). ─────────────
	tenantRepo := storesp.NewTenantRepository(db)
	taskRepo   := storesp.NewTaskRepository(db)
	orderG     := lcipkg.NewOrderGuard()

	consumer, err := consumerp.New(consumerp.Config{
		JetStream:  jsCtx, StreamName: streamName,
		Consumer:   "control-plane", TenantRepo: tenantRepo,
		TaskRepo: taskRepo, DB: db, OrderGuard: orderG,
		Log: log.New(os.Stderr, "[consumer] ", log.LstdFlags),
	})
	if err != nil {
		log.Fatalf("[consumer] new: %v", err)
	}

	go func() {
		if err := consumer.Run(context.Background()); err != nil {
			log.Printf("[consumer] run error: %v", err)
		}
	}()
	log.Printf("[consumer] start stream=%s durable=control-plane", streamName)

	srv := &http.Server{Addr: addr, Handler: r}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()
	fmt.Println("API ready at http://localhost" + addr)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	cancelPub()
	log.Println("shutting down...")
	shutdownCtx, cancelSH := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelSH()
	srv.Shutdown(shutdownCtx)
}

func logStats(p *natspkg.Publisher, ch chan string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	<-time.After(500 * time.Millisecond) // initial read.
	ch <- "started"
	for range ticker.C {
		d, f := p.Stats()
		log.Printf("[publisher] stats processed=%d failed=%d", d, f)
	}
}

func connectWithDelay(url string, timeout time.Duration, retries int) (*nats.Conn, error) {
	var lastErr error
	for i := 0; i < retries; i++ {
		c, err := nats.Connect(url)
		if err == nil && c.IsConnected() { return c, nil }
		if err != nil { lastErr = err }
		time.Sleep(timeout / time.Duration(retries))
	}
	return nil, fmt.Errorf("after %d retries: %w", retries, lastErr)
}

// outbxFetchAdapter bridges our DB → the Fetcher interface on natspublisher.
type outbxFetchAdapter struct{ db *sql.DB }

func (a *outbxFetchAdapter) FetchPending(limit int) ([]natspkg.OutboxEvent, error) {
	rows, err := a.db.Query(
		`SELECT id, subject, payload, attempts FROM outbox_events WHERE attempts < 10 ORDER BY rowid LIMIT ?`, limit)
	if err != nil { return nil, err }
	defer rows.Close()

	var evts []natspkg.OutboxEvent
	for rows.Next() {
		e := natspkg.OutboxEvent{}
		var pl string
		if err := rows.Scan(&e.ID, &e.Subject, &pl, &e.Attempts); err != nil { return nil, err }
		e.Payload = []byte(pl)
		evts = append(evts, e)
	}
	return evts, rows.Err()
}

func (a *outbxFetchAdapter) MarkSuccessful(id string) error {
	_, err := a.db.Exec(`DELETE FROM outbox_events WHERE id=? AND attempts<10`, id)
	return err
}

func (a *outbxFetchAdapter) MarkFailed(id string) (bool, error) {
	res, err := a.db.Exec(`UPDATE outbox_events SET attempts=attempts+2 WHERE id=? AND attempts<10`, id)
	if err != nil { return false, err }
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}
