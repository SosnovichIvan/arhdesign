package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	config, err := Load(func(key string) string {
		if key == "DATABASE_URL" {
			return "postgres://example"
		}
		if key == "COOLDOWN_HMAC_SECRET" {
			return "secret"
		}
		return ""
	})
	if err != nil || config.Port != "8080" || config.TelegramNotificationRetentionHours != 24 {
		t.Fatalf("config = %#v, err = %v", config, err)
	}
	_, err = Load(func(key string) string {
		if key == "DATABASE_URL" {
			return "postgres://example"
		}
		return ""
	})
	if err == nil {
		t.Fatal("missing secret must fail")
	}
	_, err = Load(func(key string) string {
		if key == "SMTP_ADDRESS" {
			return "smtp.example.com:587"
		}
		return ""
	})
	if err == nil {
		t.Fatal("partial SMTP configuration must fail")
	}
	_, err = Load(func(key string) string {
		if key == "TELEGRAM_BOT_TOKEN" {
			return "token"
		}
		return ""
	})
	if err == nil {
		t.Fatal("partial Telegram configuration must fail")
	}
	_, err = Load(func(key string) string {
		if key == "CONTACT_RETENTION_DAYS" {
			return "0"
		}
		return ""
	})
	if err == nil {
		t.Fatal("invalid retention days must fail")
	}
	_, err = Load(func(key string) string {
		if key == "TELEGRAM_NOTIFICATION_RETENTION_HOURS" {
			return "48"
		}
		return ""
	})
	if err == nil {
		t.Fatal("invalid Telegram retention hours must fail")
	}
}

func TestFromEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("COOLDOWN_HMAC_SECRET", "secret")
	t.Setenv("CONTACT_RETENTION_DAYS", "30")
	t.Setenv("TELEGRAM_NOTIFICATION_RETENTION_HOURS", "12")
	configuration, err := FromEnvironment()
	if err != nil || configuration.RetentionDays != 30 || configuration.TelegramNotificationRetentionHours != 12 {
		t.Fatalf("config = %#v, err = %v", configuration, err)
	}
}

func TestMonitoringScheduleConfiguration(t *testing.T) {
	configuration, err := Load(func(key string) string {
		switch key {
		case "MONITORING_TIMEZONE":
			return "Europe/Moscow"
		case "MONITORING_REPORT_HOUR":
			return "7"
		case "MONITORING_BACKUP_DIRECTORY":
			return "/backups"
		}
		return ""
	})
	if err != nil || configuration.MonitoringTimezone != "Europe/Moscow" || configuration.MonitoringReportHour != 7 || configuration.MonitoringBackupDirectory != "/backups" {
		t.Fatalf("config=%#v error=%v", configuration, err)
	}
	for _, values := range []map[string]string{{"MONITORING_TIMEZONE": "Mars/Olympus"}, {"MONITORING_REPORT_HOUR": "24"}} {
		if _, err := Load(func(key string) string { return values[key] }); err == nil {
			t.Fatal("invalid monitoring schedule accepted")
		}
	}
}

func TestTelegramRelayConfiguration(t *testing.T) {
	valid := map[string]string{
		"ADMIN_PASSWORD":          "password",
		"ADMIN_USERNAME":          "admin",
		"TELEGRAM_BOT_TOKEN":      "token",
		"TELEGRAM_RELAY_SECRET":   "relay-secret",
		"TELEGRAM_RELAY_URL":      "https://relay.example.com/sendMessage",
		"TELEGRAM_WEBHOOK_SECRET": "webhook-secret",
	}
	configuration, err := Load(func(key string) string { return valid[key] })
	if err != nil || configuration.TelegramRelayURL != valid["TELEGRAM_RELAY_URL"] {
		t.Fatalf("config = %#v, err = %v", configuration, err)
	}
	for name, mutate := range map[string]func(map[string]string){
		"missing relay secret": func(values map[string]string) { delete(values, "TELEGRAM_RELAY_SECRET") },
		"missing bot token":    func(values map[string]string) { delete(values, "TELEGRAM_BOT_TOKEN") },
		"non-HTTPS URL":        func(values map[string]string) { values["TELEGRAM_RELAY_URL"] = "http://relay.example.com/sendMessage" },
	} {
		t.Run(name, func(t *testing.T) {
			values := make(map[string]string, len(valid))
			for key, value := range valid {
				values[key] = value
			}
			mutate(values)
			if _, err := Load(func(key string) string { return values[key] }); err == nil {
				t.Fatal("invalid relay configuration must fail")
			}
		})
	}
}

func TestAccountConfiguration(t *testing.T) {
	valid := map[string]string{
		"ACCOUNT_AUTH_ENABLED":           "true",
		"DATABASE_URL":                   "postgres://example",
		"COOLDOWN_HMAC_SECRET":           "cooldown-secret",
		"SMTP_ADDRESS":                   "smtp.example.com:587",
		"EMAIL_FROM":                     "from@example.com",
		"EMAIL_TO":                       "contact@example.com",
		"PUBLIC_ORIGIN":                  "https://designer-svetlana.ru",
		"ACCOUNT_TOKEN_HMAC_SECRET":      strings.Repeat("h", 32),
		"ACCOUNT_TOKEN_HMAC_KEY_VERSION": "2",
		"OUTBOX_ENCRYPTION_KEY":          base64.RawStdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))),
		"OUTBOX_ENCRYPTION_KEY_VERSION":  "3",
		"TECH_ADMIN_USERNAME":            "technical.admin",
		"TECH_ADMIN_EMAIL":               "technical-admin@example.com",
		"TECH_ADMIN_PASSWORD":            "technical-admin-password-2026",
	}
	configuration, err := Load(func(key string) string { return valid[key] })
	if err != nil || !configuration.AccountEnabled || len(configuration.OutboxEncryptionKey) != 32 || configuration.AccountTokenHMACKeyVersion != 2 || configuration.OutboxEncryptionKeyVersion != 3 {
		t.Fatalf("config = %#v, err = %v", configuration, err)
	}
	valid["TECH_ADMIN_PASSWORD"] = "Semen15052018!"
	if _, err = Load(func(key string) string { return valid[key] }); err != nil {
		t.Fatalf("14-character technical administrator exception rejected: %v", err)
	}
	valid["TECH_ADMIN_PASSWORD"] = "shorter-pass!"
	if _, err = Load(func(key string) string { return valid[key] }); err == nil {
		t.Fatal("technical administrator password shorter than 14 characters accepted")
	}
	valid["TECH_ADMIN_PASSWORD"] = "technical-admin-password-2026"

	for name, mutate := range map[string]func(map[string]string){
		"short HMAC secret":               func(values map[string]string) { values["ACCOUNT_TOKEN_HMAC_SECRET"] = "short" },
		"bad encryption key":              func(values map[string]string) { values["OUTBOX_ENCRYPTION_KEY"] = "not-base64" },
		"origin with path":                func(values map[string]string) { values["PUBLIC_ORIGIN"] = "https://designer-svetlana.ru/path" },
		"bad key version":                 func(values map[string]string) { values["OUTBOX_ENCRYPTION_KEY_VERSION"] = "0" },
		"bad enabled flag":                func(values map[string]string) { values["ACCOUNT_AUTH_ENABLED"] = "sometimes" },
		"missing technical administrator": func(values map[string]string) { delete(values, "TECH_ADMIN_USERNAME") },
	} {
		t.Run(name, func(t *testing.T) {
			values := make(map[string]string, len(valid))
			for key, value := range valid {
				values[key] = value
			}
			mutate(values)
			if _, err := Load(func(key string) string { return values[key] }); err == nil {
				t.Fatal("invalid account configuration must fail")
			}
		})
	}
}
