package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	CooldownHMACSecret    string
	DatabaseURL           string
	EmailFrom             string
	EmailTo               string
	Port                  string
	RetentionDays         int
	SMTPAddress           string
	SMTPPassword          string
	SMTPUsername          string
	TelegramBotToken      string
	TelegramChatID        string
	TelegramWebhookSecret string
	AdminUsername         string
	AdminPassword         string
}

func Load(getenv func(string) string) (Config, error) {
	retentionDays := 365
	if configuredDays := getenv("CONTACT_RETENTION_DAYS"); configuredDays != "" {
		parsedDays, err := strconv.Atoi(configuredDays)
		if err != nil || parsedDays < 1 || parsedDays > 3650 {
			return Config{}, fmt.Errorf("CONTACT_RETENTION_DAYS must be between 1 and 3650")
		}
		retentionDays = parsedDays
	}
	config := Config{
		CooldownHMACSecret: getenv("COOLDOWN_HMAC_SECRET"), DatabaseURL: getenv("DATABASE_URL"), EmailFrom: getenv("EMAIL_FROM"), EmailTo: getenv("EMAIL_TO"), Port: getenv("PORT"), RetentionDays: retentionDays, SMTPAddress: getenv("SMTP_ADDRESS"), SMTPPassword: getenv("SMTP_PASSWORD"), SMTPUsername: getenv("SMTP_USERNAME"), TelegramBotToken: getenv("TELEGRAM_BOT_TOKEN"), TelegramChatID: getenv("TELEGRAM_CHAT_ID"), TelegramWebhookSecret: getenv("TELEGRAM_WEBHOOK_SECRET"), AdminUsername: getenv("ADMIN_USERNAME"), AdminPassword: getenv("ADMIN_PASSWORD"),
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
	return config, nil
}

func FromEnvironment() (Config, error) { return Load(os.Getenv) }
