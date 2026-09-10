package config

import "testing"

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
	if err != nil || config.Port != "8080" {
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
}

func TestFromEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("COOLDOWN_HMAC_SECRET", "secret")
	t.Setenv("CONTACT_RETENTION_DAYS", "30")
	configuration, err := FromEnvironment()
	if err != nil || configuration.RetentionDays != 30 {
		t.Fatalf("config = %#v, err = %v", configuration, err)
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
