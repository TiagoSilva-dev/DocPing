package main

import (
	"context"
	"docping/api/internal/app"
	"docping/api/internal/persistence"
	"docping/api/internal/storage"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	_ "time/tzdata"
)

func main() {
	config, err := app.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	store, err := persistence.Open(config.Driver, config.DSN, filepath.Join(config.DataDir, "state.json"), config.Env)
	if err != nil {
		log.Fatal("database unavailable: ", err)
	}
	defer store.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 30*time.Second)
	objects, err := storage.New(startup, config.DataDir, config.S3Endpoint, config.S3Bucket, config.S3Access, config.S3Secret, config.Scanner, config.Env, config.AllowUnscanned)
	cancel()
	if err != nil {
		log.Fatal("private storage unavailable: ", err)
	}
	application := &app.App{Config: config, Store: store, Objects: objects}
	server := &http.Server{Addr: ":" + config.Port, Handler: application.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 90 * time.Second, WriteTimeout: 120 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	go application.StartWorker(ctx)
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("DocPing API listening on :%s (%s, %s persistence)", config.Port, config.Env, config.Driver)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
