// Copyright (c) 2015-2026 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package quick

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

type testConfig struct {
	Version string            `json:"version"`
	Aliases map[string]string `json:"aliases"`
}

func TestConfigSaveLoadAndBackup(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "nested", "config.json")
	original := &testConfig{Version: "1", Aliases: map[string]string{"old": "value"}}
	cfg, err := NewConfig(original)
	if err != nil {
		t.Fatal(err)
	}
	if err = cfg.Save(filename); err != nil {
		t.Fatal(err)
	}
	if stat, statErr := os.Stat(filename); statErr != nil {
		t.Fatal(statErr)
	} else if runtime.GOOS != "windows" && stat.Mode().Perm() != 0o600 {
		t.Fatalf("expected mode 0600, got %o", stat.Mode().Perm())
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	expected := "{\n\t\"version\": \"1\",\n\t\"aliases\": {\n\t\t\"old\": \"value\"\n\t}\n}"
	if runtime.GOOS == "windows" {
		expected = strings.ReplaceAll(expected, "\n", "\r\n")
	}
	if string(data) != expected {
		t.Fatalf("unexpected JSON output:\n%s", data)
	}

	original.Aliases = map[string]string{"new": "value"}
	if err = cfg.Save(filename); err != nil {
		t.Fatal(err)
	}

	loaded := &testConfig{}
	loadedCfg, err := LoadConfig(filename, loaded)
	if err != nil {
		t.Fatal(err)
	}
	if loadedCfg.Version() != "1" {
		t.Fatalf("expected version 1, got %q", loadedCfg.Version())
	}
	if loadedCfg.Data() != loaded {
		t.Fatal("expected Data to return the configured value")
	}
	if !reflect.DeepEqual(loaded.Aliases, original.Aliases) {
		t.Fatalf("expected aliases %#v, got %#v", original.Aliases, loaded.Aliases)
	}

	backup := &testConfig{}
	if _, err = LoadConfig(filename+".old", backup); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(backup.Aliases, map[string]string{"old": "value"}) {
		t.Fatalf("unexpected backup aliases: %#v", backup.Aliases)
	}
}

func TestNewConfigValidation(t *testing.T) {
	tests := []struct {
		name string
		data any
	}{
		{name: "nil", data: nil},
		{name: "not struct", data: "config"},
		{name: "missing version", data: &struct{ Name string }{}},
		{name: "non-string version", data: &struct{ Version int }{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewConfig(test.data); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestLoadConfigErrors(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name     string
		contents string
		message  string
	}{
		{name: "syntax", contents: `{ "version": "1",`, message: "Unable to parse JSON schema due to a syntax error"},
		{name: "type", contents: `{ "version": 1 }`, message: "cannot be converted into the Go 'string' type"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filename := filepath.Join(dir, test.name+".json")
			if err := os.WriteFile(filename, []byte(test.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadConfig(filename, &testConfig{}); err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("expected error containing %q, got %v", test.message, err)
			}
		})
	}
}
