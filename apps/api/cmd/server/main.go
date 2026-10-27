package main

import (
	"context"
	"docping/api/internal/app"
	"docping/api/internal/billing"
	"docping/api/internal/mailer"
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
	objects, err := storage.New(startup, config.DataDir, config.S3Endpoint, config.S3Bucket, config.S3Access, config.S3Secret, config.Env, config.S3AcceptUnconfirmedEncryption)
	cancel()
	if err != nil {
		log.Fatal("private storage unavailable: ", err)
	}
	if config.S3AcceptUnconfirmedEncryption {
		log.Print("WARNING: S3_ACCEPT_UNCONFIRMED_ENCRYPTION is on -- an uploaded file is kept even when the provider does not confirm SSE-S3; encryption at rest is trusted, not verified")
	}
	application := &app.App{Config: config, Store: store, Objects: objects}
	if config.ResendKey != "" {
		application.Mail = mailer.New(config.ResendKey)
	}
	log.Printf("e-mail sai por %s, remetente %s", config.MailTransport(), config.Sender())
	if config.Billing() {
		application.Stripe = billing.New(config.StripeKey)
	} else {
		log.Print("billing is off: STRIPE_SECRET_KEY is not set, so every workspace keeps the allowance it has")
	}
	server := &http.Server{Addr: ":" + config.Port, Handler: application.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 90 * time.Second, WriteTimeout: 120 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	application.StartAnalysis(ctx)
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
