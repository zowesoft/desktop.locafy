package settings

import (
	"testing"
)

func TestServiceJSONValidation(t *testing.T) {
	database := openMigrated(t)
	service := NewService(database)

	if err := service.SetJSON("config", []byte(`{"theme":"dark"}`)); err != nil {
		t.Fatalf("set valid JSON: %v", err)
	}
	setting, err := service.Get("config")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if setting.Type != TypeJSON || setting.Value != `{"theme":"dark"}` {
		t.Fatalf("setting = %+v, want json type with stored payload", setting)
	}

	if err := service.SetJSON("config", []byte(`{broken`)); err == nil {
		t.Fatal("storing invalid JSON succeeded, want error")
	}
	if _, err := service.Get("config"); err != nil {
		t.Fatalf("get after failed set: %v", err)
	}
}

func TestServiceDelete(t *testing.T) {
	database := openMigrated(t)
	service := NewService(database)

	if err := service.SetString("theme", "dark"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := service.Delete("theme"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := service.Get("theme"); err == nil {
		t.Fatal("get after delete succeeded, want ErrNotFound")
	}

	// Deleting an absent key is a no-op, not an error.
	if err := service.Delete("theme"); err != nil {
		t.Fatalf("delete absent key: %v", err)
	}
}

func TestServiceRoundTrip(t *testing.T) {
	database := openMigrated(t)
	service := NewService(database)

	if err := service.SetBool("auto_start", true); err != nil {
		t.Fatalf("set bool: %v", err)
	}
	if err := service.SetNumber("max_log_size", 10.5); err != nil {
		t.Fatalf("set number: %v", err)
	}

	autoStart, err := service.Get("auto_start")
	if err != nil {
		t.Fatalf("get bool: %v", err)
	}
	if autoStart.Type != TypeBoolean || autoStart.Value != "true" {
		t.Fatalf("bool setting = %+v, want boolean true", autoStart)
	}

	maxSize, err := service.Get("max_log_size")
	if err != nil {
		t.Fatalf("get number: %v", err)
	}
	if maxSize.Type != TypeNumber || maxSize.Value != "10.5" {
		t.Fatalf("number setting = %+v, want number 10.5", maxSize)
	}
}
