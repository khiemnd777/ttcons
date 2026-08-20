package core

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"consflow/backend/internal/shared/config"
)

func systemSettingsBackupFixture(t *testing.T) (*SystemSettingsStore, config.Config, map[string]storedSystemSetting) {
	t.Helper()
	cfg := config.Load()
	cfg.StoragePath = t.TempDir()
	store := NewSystemSettingsStore(nil, cfg)
	rows := map[string]storedSystemSetting{}
	for _, definition := range sortedSystemSettingDefinitions() {
		value, err := normalizeSystemSettingValue(definition, systemSettingConfigValue(cfg, definition.Key))
		if err != nil {
			t.Fatalf("normalize %s: %v", definition.Key, err)
		}
		rows[definition.Key] = storedSystemSetting{
			Key: definition.Key, Category: definition.Category, ValueType: definition.ValueType,
			IsSecret: definition.Sensitive, RequiresRestart: definition.RequiresRestart,
			Value: value, Configured: strings.TrimSpace(toText(value)) != "", Version: 7,
		}
	}
	return store, cfg, rows
}

func TestSystemSettingsBackupExportExcludesSecretValues(t *testing.T) {
	_, cfg, rows := systemSettingsBackupFixture(t)
	secret := rows["SMTP_PASSWORD"]
	secret.Value = "smtp-plaintext-must-not-leak"
	secret.Configured = true
	rows["SMTP_PASSWORD"] = secret

	raw, document, err := buildSystemSettingsBackup(rows, cfg, time.Date(2026, time.August, 20, 9, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("build backup: %v", err)
	}
	if bytes.Contains(raw, []byte("smtp-plaintext-must-not-leak")) {
		t.Fatal("backup leaked SMTP secret plaintext")
	}
	if len(document.Settings) != len(systemSettingDefinitions)-1 {
		t.Fatalf("settings count=%d want=%d", len(document.Settings), len(systemSettingDefinitions)-1)
	}
	if len(document.ExcludedSecrets) != 1 || document.ExcludedSecrets[0].Key != "SMTP_PASSWORD" || !document.ExcludedSecrets[0].Configured {
		t.Fatalf("excluded secrets=%+v", document.ExcludedSecrets)
	}
	parsed, err := parseSystemSettingsBackup(raw)
	if err != nil {
		t.Fatalf("parse exported backup: %v", err)
	}
	if parsed.ExportedAt != "2026-08-20T09:30:00Z" || parsed.RegistryFingerprint != systemSettingsRegistryFingerprint() {
		t.Fatalf("unexpected export metadata: %+v", parsed)
	}
}

func TestSystemSettingsBackupPreviewClassifiesChangesAndCompatibility(t *testing.T) {
	store, cfg, rows := systemSettingsBackupFixture(t)
	raw, document, err := buildSystemSettingsBackup(rows, cfg, time.Now())
	if err != nil {
		t.Fatalf("build backup: %v", err)
	}
	_ = raw
	settings := make([]systemSettingsBackupEntry, 0, len(document.Settings))
	for _, entry := range document.Settings {
		if entry.Key == "HTTP_READ_TIMEOUT" {
			continue
		}
		if entry.Key == "client_payment_min_percent" {
			entry.Value = float64(40)
		}
		settings = append(settings, entry)
	}
	settings = append(settings, systemSettingsBackupEntry{Key: "REMOVED_FUTURE_SETTING", ValueType: "string", Value: "legacy"})
	document.Settings = settings
	raw, err = json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal changed backup: %v", err)
	}
	analysis, err := store.analyzeBackupRows(raw, rows)
	if err != nil {
		t.Fatalf("analyze backup: %#v", err)
	}
	if analysis.ChangedCount != 1 || analysis.ImmediateCount != 1 || analysis.RestartRequiredCount != 0 {
		t.Fatalf("change counts=%+v", analysis.responseRecord())
	}
	if analysis.UnsupportedCount != 1 || analysis.PreservedMissingCount != 1 {
		t.Fatalf("compatibility counts=%+v", analysis.responseRecord())
	}
	if analysis.ExpectedVersions["client_payment_min_percent"] != 7 || len(analysis.Updates) != 1 {
		t.Fatalf("expected versions=%+v updates=%+v", analysis.ExpectedVersions, analysis.Updates)
	}
	if analysis.FileHash != systemSettingsBackupHash(raw) {
		t.Fatalf("file hash=%q", analysis.FileHash)
	}
}

func TestSystemSettingsBackupRejectsSensitiveCoreDuplicateAndInvalidValues(t *testing.T) {
	store, cfg, rows := systemSettingsBackupFixture(t)
	_, baseline, err := buildSystemSettingsBackup(rows, cfg, time.Now())
	if err != nil {
		t.Fatalf("build backup: %v", err)
	}
	tests := []struct {
		name      string
		mutate    func(*systemSettingsBackupDocument)
		wantError string
	}{
		{
			name: "sensitive key", wantError: "secret",
			mutate: func(document *systemSettingsBackupDocument) {
				document.Settings = append(document.Settings, systemSettingsBackupEntry{Key: "SMTP_PASSWORD", ValueType: "secret", Value: "plaintext"})
			},
		},
		{
			name: "core env key", wantError: ".env",
			mutate: func(document *systemSettingsBackupDocument) {
				document.Settings = append(document.Settings, systemSettingsBackupEntry{Key: "DATABASE_URL", ValueType: "string", Value: "postgres://example"})
			},
		},
		{
			name: "duplicate key", wantError: "bị lặp",
			mutate: func(document *systemSettingsBackupDocument) {
				document.Settings = append(document.Settings, document.Settings[0])
			},
		},
		{
			name: "invalid known value", wantError: "phải là số",
			mutate: func(document *systemSettingsBackupDocument) {
				for index := range document.Settings {
					if document.Settings[index].Key == "client_payment_min_percent" {
						document.Settings[index].Value = "not-a-number"
					}
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := baseline
			document.Settings = append([]systemSettingsBackupEntry(nil), baseline.Settings...)
			test.mutate(&document)
			raw, marshalErr := json.Marshal(document)
			if marshalErr != nil {
				t.Fatalf("marshal: %v", marshalErr)
			}
			_, analyzeErr := store.analyzeBackupRows(raw, rows)
			if analyzeErr == nil || !strings.Contains(strings.ToLower(analyzeErr.Error()), strings.ToLower(test.wantError)) {
				t.Fatalf("error=%v want substring %q", analyzeErr, test.wantError)
			}
		})
	}
}

func TestSystemSettingsBackupRejectsOversizedJSON(t *testing.T) {
	raw := bytes.Repeat([]byte(" "), maxSystemSettingsBackupBytes+1)
	if _, err := parseSystemSettingsBackup(raw); err == nil || !strings.Contains(err.Error(), "256 KB") {
		t.Fatalf("error=%v", err)
	}
}

func toText(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(string(backupTestJSON(value)), `"`), `"`))
}

func backupTestJSON(value any) []byte {
	raw, _ := json.Marshal(value)
	return raw
}
