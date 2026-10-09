package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	accountgenerated "github.com/SosnovichIvan/arhdesign/apps/api/internal/accountapi/generated"
	generated "github.com/SosnovichIvan/arhdesign/apps/api/internal/api/generated"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/config"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/globalchat"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/handler"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/monitoring"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/notification"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/preferences"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/repository"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/service"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/technicalsupport"
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
	var emailOutbox *notification.EmailOutbox
	var telegramAccountOutbox *notification.TelegramAccountOutbox
	var accountService *account.Service
	var technicalService *technicalsupport.Service
	var monitoringService *monitoring.Service
	var accountCipher account.PayloadCipher
	if configuration.AccountEnabled {
		tokens, err := account.NewTokenManager([]byte(configuration.AccountTokenHMACSecret), configuration.AccountTokenHMACKeyVersion)
		if err != nil {
			slog.Error("invalid account token configuration", "error", err)
			os.Exit(1)
		}
		cipher, err := account.NewPayloadCipher(configuration.OutboxEncryptionKey, configuration.OutboxEncryptionKeyVersion)
		if err != nil {
			slog.Error("invalid outbox encryption configuration", "error", err)
			os.Exit(1)
		}
		accountCipher = cipher
		monitoringService, err = monitoring.NewService(retentionStore, monitoring.NewRuntimeCollector(configuration.MonitoringBackupDirectory), cipher, time.Now, configuration.MonitoringTimezone, configuration.MonitoringReportHour, configuration.ReleaseVersion)
		if err != nil {
			slog.Error("monitoring service initialization failed", "error", err)
			os.Exit(1)
		}
		accountService, err = account.NewService(retentionStore, tokens, cipher, configuration.PublicOrigin, configuration.EmailFrom, time.Now)
		if err != nil {
			slog.Error("account service initialization failed", "error", err)
			os.Exit(1)
		}
		accountService.ConfigureBootstrap(configuration.AdminUsername, configuration.AdminPassword)
		accountService.ConfigureTelegramBot(configuration.TelegramBotUsername)
		if _, _, err = accountService.EnsureTechnicalAdmin(context.Background(), configuration.TechnicalAdminUsername, configuration.TechnicalAdminEmail, configuration.TechnicalAdminPassword); err != nil {
			slog.Error("technical administrator bootstrap failed", "error_class", "technical_admin_bootstrap")
			os.Exit(1)
		}
		accountEndpoint, err := handler.NewAccountEndpoint(accountService)
		if err != nil {
			slog.Error("account endpoint initialization failed", "error", err)
			os.Exit(1)
		}
		projectService, err := project.NewService(retentionStore, []byte(configuration.AccountTokenHMACSecret), time.Now)
		if err != nil {
			slog.Error("project service initialization failed", "error", err)
			os.Exit(1)
		}
		if err = projectService.ConfigureNotifications(cipher); err != nil {
			slog.Error("project notification configuration failed", "error", err)
			os.Exit(1)
		}
		accountEndpoint.ConfigureProjects(projectService)
		preferencesService, err := preferences.NewService(retentionStore)
		if err != nil {
			slog.Error("settings service initialization failed", "error", err)
			os.Exit(1)
		}
		accountEndpoint.ConfigurePreferences(preferencesService)
		globalChatService, err := globalchat.NewService(retentionStore, []byte(configuration.AccountTokenHMACSecret), cipher, time.Now)
		if err != nil {
			slog.Error("global chat service initialization failed", "error", err)
			os.Exit(1)
		}
		accountEndpoint.ConfigureGlobalChats(globalChatService)
		technicalService, err = technicalsupport.NewService(retentionStore, cipher, []byte(configuration.AccountTokenHMACSecret), configuration.AccountTokenHMACKeyVersion, time.Now)
		if err != nil {
			slog.Error("technical support service initialization failed", "error", err)
			os.Exit(1)
		}
		accountEndpoint.ConfigureTechnicalSupport(technicalService)
		accountgenerated.HandlerWithOptions(accountEndpoint, accountgenerated.StdHTTPServerOptions{BaseURL: "/api", BaseRouter: mux})
		emailOutbox, err = notification.NewEmailOutbox(retentionStore, cipher, notification.NewSMTPTransport(configuration.SMTPAddress, configuration.SMTPUsername, configuration.SMTPPassword), slog.Default(), time.Now)
		if err != nil {
			slog.Error("email outbox initialization failed", "error", err)
			os.Exit(1)
		}
	}
	if configuration.TelegramBotToken != "" && retentionStore != nil {
		telegramClient := notification.NewTelegram(configuration.TelegramBotToken, "", http.DefaultClient)
		if configuration.TelegramRelayURL != "" {
			telegramClient = notification.NewTelegramViaRelay(configuration.TelegramRelayURL, configuration.TelegramRelaySecret, http.DefaultClient)
		}
		mux.Handle("POST /api/telegram/webhook", handler.NewTelegramWebhook(configuration.TelegramWebhookSecret, accountService, telegramClient))
		if accountService != nil {
			telegramAccountOutbox, err = notification.NewTelegramAccountOutbox(retentionStore, accountCipher, telegramClient, slog.Default(), time.Now)
			if err != nil {
				slog.Error("Telegram account outbox initialization failed", "error", err)
				os.Exit(1)
			}
		}
	}
	mux.HandleFunc("GET /healthz", handler.Health)
	mux.HandleFunc("GET /readyz", handler.Health)
	generated.HandlerWithOptions(contactEndpoint, generated.StdHTTPServerOptions{BaseURL: "/api", BaseRouter: mux})
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if retentionStore != nil {
		go runRetentionCleanup(signalContext, retentionStore, configuration.RetentionDays)
		if emailOutbox != nil {
			go emailOutbox.Run(signalContext, time.Second)
		}
		if telegramAccountOutbox != nil {
			go telegramAccountOutbox.Run(signalContext, time.Second)
		}
		if monitoringService != nil {
			go runMonitoring(signalContext, monitoringService)
		}
		if configuration.TelegramBotToken != "" {
			telegramClient := notification.NewTelegram(configuration.TelegramBotToken, "", http.DefaultClient)
			if configuration.TelegramRelayURL != "" {
				telegramClient = notification.NewTelegramViaRelay(configuration.TelegramRelayURL, configuration.TelegramRelaySecret, http.DefaultClient)
			}
			go runTelegramRetentionCleanup(signalContext, notification.NewTelegramRetention(telegramClient, retentionStore))
		}
	}
	server := &http.Server{Addr: ":" + configuration.Port, Handler: handler.TechnicalIncidentMiddleware(mux, technicalService, slog.Default(), time.Now), ReadHeaderTimeout: 5 * time.Second}
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

func runMonitoring(ctx context.Context, monitor *monitoring.Service) {
	run := func() {
		requestContext, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		if err := monitor.Tick(requestContext); err != nil {
			slog.Warn("monitoring tick failed", "error_class", fmt.Sprintf("%T", err))
		}
	}
	run()
	timer := time.NewTimer(monitoring.UntilNextQuarterHour(time.Now()))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			run()
			timer.Reset(monitoring.UntilNextQuarterHour(time.Now()))
		}
	}
}

func runTelegramRetentionCleanup(ctx context.Context, retention notification.TelegramRetention) {
	cleanup := func() {
		requestContext, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		deleted, err := retention.DeleteDue(requestContext, time.Now())
		if err != nil {
			slog.Warn("telegram notification retention cleanup failed", "error_class", "telegram_retention")
			return
		}
		if deleted > 0 {
			slog.Info("telegram notification retention cleanup completed", "deleted_count", deleted)
		}
	}
	cleanup()
	ticker := time.NewTicker(time.Hour)
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
		expiredCooldowns, err := store.DeleteExpiredCooldowns(ctx, time.Now())
		if err != nil {
			slog.Warn("contact cooldown cleanup failed", "error_class", "cooldown_retention")
			return
		}
		if expiredCooldowns > 0 {
			slog.Info("contact cooldown cleanup completed", "deleted_count", expiredCooldowns)
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

func configuredNotifiers(configuration config.Config, store *repository.Postgres) []notification.Sender {
	senders := make([]notification.Sender, 0, 2)
	if configuration.SMTPAddress != "" && configuration.EmailFrom != "" && configuration.EmailTo != "" {
		transport := notification.NewSMTPTransport(configuration.SMTPAddress, configuration.SMTPUsername, configuration.SMTPPassword)
		senders = append(senders, notification.NewEmail(configuration.EmailFrom, configuration.EmailTo, transport))
	}
	if configuration.TelegramBotToken != "" && store != nil {
		retention := time.Duration(configuration.TelegramNotificationRetentionHours) * time.Hour
		if configuration.TelegramRelayURL != "" {
			senders = append(senders, notification.NewTelegramSubscribersViaRelay(configuration.TelegramRelayURL, configuration.TelegramRelaySecret, store, http.DefaultClient, retention))
		} else {
			senders = append(senders, notification.NewTelegramSubscribers(configuration.TelegramBotToken, store, http.DefaultClient, retention))
		}
	} else if configuration.TelegramBotToken != "" && configuration.TelegramChatID != "" {
		senders = append(senders, notification.NewTelegram(configuration.TelegramBotToken, configuration.TelegramChatID, http.DefaultClient))
	}
	return senders
}
