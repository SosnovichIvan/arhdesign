package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	generated "github.com/SosnovichIvan/arhdesign/apps/api/internal/api/generated"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/config"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/handler"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/notification"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/repository"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	configuration, err := config.FromEnvironment()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	cooldown := service.NewCooldown(time.Now)
	var retentionStore *repository.Postgres
	if configuration.DatabaseURL != "" {
		pool, err := pgxpool.New(context.Background(), configuration.DatabaseURL)
		if err != nil {
			slog.Error("postgres connection failed", "error", err)
			os.Exit(1)
		}
		defer pool.Close()
		retentionStore = repository.NewPostgres(pool)
	}
	notifier := notification.NewDispatcher(slog.Default(), 10*time.Second, configuredNotifiers(configuration, retentionStore)...)
	contactEndpoint := handler.NewContactEndpoint(cooldown, retentionStore, []byte(configuration.CooldownHMACSecret), service.NewRateLimiter(time.Now, time.Minute), notifier)
	if configuration.TelegramBotToken != "" && retentionStore != nil {
		mux.Handle("POST /api/telegram/webhook", handler.NewTelegramWebhook(configuration.TelegramWebhookSecret, configuration.AdminUsername, configuration.AdminPassword, retentionStore))
	}
	mux.HandleFunc("GET /healthz", handler.Health)
	mux.HandleFunc("GET /readyz", handler.Health)
	generated.HandlerWithOptions(contactEndpoint, generated.StdHTTPServerOptions{BaseURL: "/api", BaseRouter: mux})
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if retentionStore != nil {
		go runRetentionCleanup(signalContext, retentionStore, configuration.RetentionDays)
	}
	server := &http.Server{Addr: ":" + configuration.Port, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}()
	<-signalContext.Done()
	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
}

func runRetentionCleanup(ctx context.Context, store *repository.Postgres, retentionDays int) {
	cleanup := func() {
		deleted, err := store.DeleteSubmissionsOlderThan(ctx, time.Now().AddDate(0, 0, -retentionDays))
		if err != nil {
			slog.Warn("contact retention cleanup failed", "error", err)
			return
		}
		if deleted > 0 {
			slog.Info("contact retention cleanup completed", "deleted_count", deleted)
		}
	}
	cleanup()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
		}
	}
}

func configuredNotifiers(configuration config.Config, store repository.TelegramSubscriberStore) []notification.Sender {
	senders := make([]notification.Sender, 0, 2)
	if configuration.SMTPAddress != "" && configuration.EmailFrom != "" && configuration.EmailTo != "" {
		transport := notification.NewSMTPTransport(configuration.SMTPAddress, configuration.SMTPUsername, configuration.SMTPPassword)
		senders = append(senders, notification.NewEmail(configuration.EmailFrom, configuration.EmailTo, transport))
	}
	if configuration.TelegramBotToken != "" && store != nil {
		senders = append(senders, notification.NewTelegramSubscribers(configuration.TelegramBotToken, store, http.DefaultClient))
	} else if configuration.TelegramBotToken != "" && configuration.TelegramChatID != "" {
		senders = append(senders, notification.NewTelegram(configuration.TelegramBotToken, configuration.TelegramChatID, http.DefaultClient))
	}
	return senders
}
