package config

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AccountEnabled                     bool
	AccountTokenHMACSecret             string
	AccountTokenHMACKeyVersion         int
	CooldownHMACSecret                 string
	DatabaseURL                        string
	EmailFrom                          string
	EmailTo                            string
	Port                               string
	RetentionDays                      int
	SMTPAddress                        string
	SMTPPassword                       string
	SMTPUsername                       string
	TelegramBotToken                   string
	TelegramBotUsername                string
	TelegramChatID                     string
	TelegramNotificationRetentionHours int
	TelegramRelaySecret                string
	TelegramRelayURL                   string
	TelegramWebhookSecret              string
	AdminUsername                      string
	AdminPassword                      string
	TechnicalAdminUsername             string
	TechnicalAdminEmail                string
	TechnicalAdminPassword             string
	OutboxEncryptionKey                []byte
	OutboxEncryptionKeyVersion         int
	PublicOrigin                       string
	MonitoringTimezone                 string
	MonitoringReportHour               int
	MonitoringBackupDirectory          string
	ReleaseVersion                     string
}

func Load(getenv func(string) string) (Config, error) {
	accountEnabled := false
	if configured := getenv("ACCOUNT_AUTH_ENABLED"); configured != "" {
		parsed, err := strconv.ParseBool(configured)
		if err != nil {
			return Config{}, fmt.Errorf("ACCOUNT_AUTH_ENABLED must be true or false")
		}
		accountEnabled = parsed
	}
	accountTokenKeyVersion, err := positiveVersion(getenv("ACCOUNT_TOKEN_HMAC_KEY_VERSION"))
	if err != nil {
		return Config{}, fmt.Errorf("ACCOUNT_TOKEN_HMAC_KEY_VERSION must be a positive integer")
	}
	outboxKeyVersion, err := positiveVersion(getenv("OUTBOX_ENCRYPTION_KEY_VERSION"))
	if err != nil {
		return Config{}, fmt.Errorf("OUTBOX_ENCRYPTION_KEY_VERSION must be a positive integer")
	}
	retentionDays := 365
	if configuredDays := getenv("CONTACT_RETENTION_DAYS"); configuredDays != "" {
		parsedDays, err := strconv.Atoi(configuredDays)
		if err != nil || parsedDays < 1 || parsedDays > 3650 {
			return Config{}, fmt.Errorf("CONTACT_RETENTION_DAYS must be between 1 and 3650")
		}
		retentionDays = parsedDays
	}
	telegramNotificationRetentionHours := 24
	if configuredHours := getenv("TELEGRAM_NOTIFICATION_RETENTION_HOURS"); configuredHours != "" {
		parsedHours, err := strconv.Atoi(configuredHours)
		if err != nil || parsedHours < 1 || parsedHours > 24 {
			return Config{}, fmt.Errorf("TELEGRAM_NOTIFICATION_RETENTION_HOURS must be between 1 and 24")
		}
		telegramNotificationRetentionHours = parsedHours
	}
	monitoringReportHour := 9
	if configuredHour := getenv("MONITORING_REPORT_HOUR"); configuredHour != "" {
		parsedHour, err := strconv.Atoi(configuredHour)
		if err != nil || parsedHour < 0 || parsedHour > 23 {
			return Config{}, fmt.Errorf("MONITORING_REPORT_HOUR must be between 0 and 23")
		}
		monitoringReportHour = parsedHour
	}
	monitoringTimezone := getenv("MONITORING_TIMEZONE")
	if monitoringTimezone == "" {
		monitoringTimezone = "Europe/Moscow"
	}
	if _, err := time.LoadLocation(monitoringTimezone); err != nil {
		return Config{}, fmt.Errorf("MONITORING_TIMEZONE must be a valid IANA timezone")
	}
	config := Config{
		AccountTokenHMACSecret: getenv("ACCOUNT_TOKEN_HMAC_SECRET"), AccountTokenHMACKeyVersion: accountTokenKeyVersion,
		CooldownHMACSecret: getenv("COOLDOWN_HMAC_SECRET"), DatabaseURL: getenv("DATABASE_URL"), EmailFrom: getenv("EMAIL_FROM"), EmailTo: getenv("EMAIL_TO"), Port: getenv("PORT"), RetentionDays: retentionDays, SMTPAddress: getenv("SMTP_ADDRESS"), SMTPPassword: getenv("SMTP_PASSWORD"), SMTPUsername: getenv("SMTP_USERNAME"), TelegramBotToken: getenv("TELEGRAM_BOT_TOKEN"), TelegramBotUsername: getenv("TELEGRAM_BOT_USERNAME"), TelegramChatID: getenv("TELEGRAM_CHAT_ID"), TelegramNotificationRetentionHours: telegramNotificationRetentionHours, TelegramRelaySecret: getenv("TELEGRAM_RELAY_SECRET"), TelegramRelayURL: getenv("TELEGRAM_RELAY_URL"), TelegramWebhookSecret: getenv("TELEGRAM_WEBHOOK_SECRET"), AdminUsername: getenv("ADMIN_USERNAME"), AdminPassword: getenv("ADMIN_PASSWORD"), TechnicalAdminUsername: getenv("TECH_ADMIN_USERNAME"), TechnicalAdminEmail: getenv("TECH_ADMIN_EMAIL"), TechnicalAdminPassword: getenv("TECH_ADMIN_PASSWORD"),
		OutboxEncryptionKeyVersion: outboxKeyVersion, PublicOrigin: getenv("PUBLIC_ORIGIN"), MonitoringTimezone: monitoringTimezone, MonitoringReportHour: monitoringReportHour, MonitoringBackupDirectory: getenv("MONITORING_BACKUP_DIRECTORY"), ReleaseVersion: getenv("RELEASE_VERSION"),
	}
	if config.Port == "" {
		config.Port = "8080"
	}
	if config.DatabaseURL != "" && config.CooldownHMACSecret == "" {
		return Config{}, fmt.Errorf("COOLDOWN_HMAC_SECRET is required when DATABASE_URL is set")
	}
	if (config.SMTPAddress != "" || config.EmailFrom != "" || config.EmailTo != "") && (config.SMTPAddress == "" || config.EmailFrom == "" || config.EmailTo == "") {
		return Config{}, fmt.Errorf("SMTP_ADDRESS, EMAIL_FROM and EMAIL_TO must be set together")
	}
	if config.TelegramChatID != "" && config.TelegramBotToken == "" {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is required when TELEGRAM_CHAT_ID is set")
	}
	if config.TelegramBotToken != "" && (config.TelegramWebhookSecret == "" || config.AdminUsername == "" || config.AdminPassword == "") {
		return Config{}, fmt.Errorf("TELEGRAM_WEBHOOK_SECRET, ADMIN_USERNAME and ADMIN_PASSWORD are required with TELEGRAM_BOT_TOKEN")
	}
	if accountEnabled && config.TelegramBotToken != "" && config.TelegramBotUsername == "" {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_USERNAME is required for account Telegram handoff")
	}
	if (config.TelegramRelayURL == "") != (config.TelegramRelaySecret == "") {
		return Config{}, fmt.Errorf("TELEGRAM_RELAY_URL and TELEGRAM_RELAY_SECRET must be set together")
	}
	if config.TelegramRelayURL != "" {
		if config.TelegramBotToken == "" {
			return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is required with TELEGRAM_RELAY_URL")
		}
		relayURL, err := url.ParseRequestURI(config.TelegramRelayURL)
		if err != nil || relayURL.Scheme != "https" || relayURL.Host == "" {
			return Config{}, fmt.Errorf("TELEGRAM_RELAY_URL must be a valid HTTPS URL")
		}
	}
	if accountEnabled {
		if config.DatabaseURL == "" || config.SMTPAddress == "" || config.EmailFrom == "" || config.PublicOrigin == "" || len(config.AccountTokenHMACSecret) < 32 {
			return Config{}, fmt.Errorf("account auth requires DATABASE_URL, SMTP_ADDRESS, EMAIL_FROM, PUBLIC_ORIGIN and ACCOUNT_TOKEN_HMAC_SECRET with at least 32 bytes")
		}
		outboxKey, err := base64.RawStdEncoding.DecodeString(getenv("OUTBOX_ENCRYPTION_KEY"))
		if err != nil || len(outboxKey) != 32 {
			return Config{}, fmt.Errorf("OUTBOX_ENCRYPTION_KEY must be an unpadded base64-encoded 32-byte key")
		}
		origin, err := url.ParseRequestURI(config.PublicOrigin)
		if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") || origin.Host == "" || origin.Path != "" {
			return Config{}, fmt.Errorf("PUBLIC_ORIGIN must be an absolute HTTP(S) origin without a path")
		}
		config.OutboxEncryptionKey = outboxKey
		if config.TechnicalAdminUsername == "" || config.TechnicalAdminEmail == "" || len([]rune(config.TechnicalAdminPassword)) < 14 {
			return Config{}, fmt.Errorf("account auth requires TECH_ADMIN_USERNAME, TECH_ADMIN_EMAIL and TECH_ADMIN_PASSWORD with at least 14 characters")
		}
		config.AccountEnabled = true
	}
	return config, nil
}

func positiveVersion(value string) (int, error) {
	if value == "" {
		return 1, nil
	}
	version, err := strconv.Atoi(value)
	if err != nil || version < 1 {
		return 0, fmt.Errorf("version must be positive")
	}
	return version, nil
}

func FromEnvironment() (Config, error) { return Load(os.Getenv) }
