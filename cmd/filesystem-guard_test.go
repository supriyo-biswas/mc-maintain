// Copyright (c) 2015-2026 MinIO, Inc. and other contributors
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/supriyo-biswas/mc/pkg/probe"
)

func TestFilesystemGuardHelpers(t *testing.T) {
	oldAliases := aliasToConfigMap
	oldConfig := cacheCfgV10
	oldLoadConfig := loadMcConfig
	aliasToConfigMap = map[string]*aliasConfigV10{
		"guardremote": {
			URL:       "https://example.com",
			AccessKey: "access-key",
			SecretKey: "secret-key",
			API:       "S3v4",
			Path:      "auto",
		},
	}
	cacheCfgV10 = newConfigV10()
	loadMcConfig = func() (*configV10, *probe.Error) {
		return cacheCfgV10, nil
	}
	t.Cleanup(func() {
		aliasToConfigMap = oldAliases
		cacheCfgV10 = oldConfig
		loadMcConfig = oldLoadConfig
	})
	t.Setenv("MC_HOST_guardenvironment", "https://access-key:secret-key@example.net")

	local := filepath.Join(t.TempDir(), "local")
	tests := []struct {
		name     string
		wantErr  string
		validate func() error
	}{
		{
			name: "all configured aliases",
			validate: func() error {
				return probeCause(requireAliasedURLs("ls", "guardremote/bucket", "guardenvironment/bucket"))
			},
		},
		{
			name: "stdin is not a filesystem path",
			validate: func() error {
				return probeCause(requireAliasedURLsOrStdin("head", "-"))
			},
		},
		{
			name:    "stdin is local without explicit allowance",
			wantErr: "resolves to the local filesystem",
			validate: func() error {
				return probeCause(requireAliasedURLs("ls", "-"))
			},
		},
		{
			name:    "one local URL fails all URL policy",
			wantErr: "resolves to the local filesystem",
			validate: func() error {
				return probeCause(requireAliasedURLs("ls", "guardremote/bucket", local))
			},
		},
		{
			name:    "local only transfer fails",
			wantErr: "requires at least one source or target",
			validate: func() error {
				return probeCause(requireAnyAliasedURL("cp", local, local+"-copy"))
			},
		},
		{
			name: "local to remote transfer",
			validate: func() error {
				return probeCause(requireAnyAliasedURL("cp", local, "guardremote/bucket/object"))
			},
		},
		{
			name: "remote to local transfer",
			validate: func() error {
				return probeCause(requireAnyAliasedURL("diff", "guardremote/bucket", local))
			},
		},
		{
			name: "remote to remote transfer",
			validate: func() error {
				return probeCause(requireAnyAliasedURL("mirror", "guardremote/source", "guardenvironment/target"))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.validate()
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("expected success, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("expected error containing %q, got %v", test.wantErr, err)
			}
		})
	}
}

func probeCause(err *probe.Error) error {
	if err == nil {
		return nil
	}
	return err.ToGoError()
}

func TestLocalFilesystemCommandsAreRejected(t *testing.T) {
	tests := []struct {
		name string
		args func(root, source, target string) []string
	}{
		{name: "anonymous", args: func(_, source, _ string) []string { return []string{"anonymous", "set", "public", source} }},
		{name: "cat", args: func(_, source, _ string) []string { return []string{"cat", source} }},
		{name: "head", args: func(_, source, _ string) []string { return []string{"head", source} }},
		{name: "ls", args: func(root, _, _ string) []string { return []string{"ls", root} }},
		{name: "du", args: func(root, _, _ string) []string { return []string{"du", root} }},
		{name: "find", args: func(root, _, _ string) []string { return []string{"find", root} }},
		{name: "stat", args: func(_, source, _ string) []string { return []string{"stat", source} }},
		{name: "tree", args: func(root, _, _ string) []string { return []string{"tree", root} }},
		{name: "watch", args: func(root, _, _ string) []string { return []string{"watch", root} }},
		{name: "mb", args: func(_, _, target string) []string { return []string{"mb", target} }},
		{name: "rb", args: func(root, _, _ string) []string { return []string{"rb", "--force", root} }},
		{name: "rm", args: func(_, source, _ string) []string { return []string{"rm", "--force", source} }},
		{name: "pipe", args: func(_, _, target string) []string { return []string{"pipe", target} }},
		{name: "cp", args: func(_, source, target string) []string { return []string{"cp", source, target} }},
		{name: "mv", args: func(_, source, target string) []string { return []string{"mv", source, target} }},
		{name: "mirror", args: func(root, _, target string) []string { return []string{"mirror", root, target} }},
		{name: "diff", args: func(root, _, target string) []string { return []string{"diff", root, target} }},
		{name: "get", args: func(_, source, target string) []string { return []string{"get", source, target} }},
		{name: "put", args: func(_, source, target string) []string { return []string{"put", source, target} }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			target := filepath.Join(root, "target")
			if err := os.WriteFile(source, []byte("unchanged"), 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(root)
			if err != nil {
				t.Fatal(err)
			}

			output, exitErr := runFilesystemGuardCommand(t, test.args(root, source, target))
			if exitErr == nil {
				t.Fatalf("expected command failure, got output %q", output)
			}
			if !strings.Contains(output, "configured S3 alias") {
				t.Fatalf("expected alias guard error, got %q", output)
			}
			if strings.Contains(output, "Unable to ") {
				t.Fatalf("expected alias error without a redundant prefix, got %q", output)
			}

			contents, err := os.ReadFile(source)
			if err != nil {
				t.Fatalf("source was removed: %v", err)
			}
			if string(contents) != "unchanged" {
				t.Fatalf("source was modified: %q", contents)
			}
			after, err := os.Stat(root)
			if err != nil {
				t.Fatalf("root was removed: %v", err)
			}
			if after.Mode().Perm() != before.Mode().Perm() {
				t.Fatalf("root permissions changed from %v to %v", before.Mode().Perm(), after.Mode().Perm())
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatalf("target was created: %v", err)
			}
		})
	}
}

func TestRmStdinRejectsLocalFilesystem(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.WriteFile(source, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}

	output, exitErr := runFilesystemGuardCommand(t, []string{"rm", "--force", "--stdin"}, source+"\n")
	if exitErr == nil {
		t.Fatalf("expected command failure, got output %q", output)
	}
	if !strings.Contains(output, "configured S3 alias") {
		t.Fatalf("expected alias guard error, got %q", output)
	}
	if contents, err := os.ReadFile(source); err != nil || string(contents) != "unchanged" {
		t.Fatalf("source was modified or removed: contents=%q err=%v", contents, err)
	}
}

func TestFilesystemCommandsDoNotDefaultToWorkingDirectory(t *testing.T) {
	for _, command := range []string{"ls", "find", "tree"} {
		t.Run(command, func(t *testing.T) {
			output, exitErr := runFilesystemGuardCommand(t, []string{command})
			if exitErr == nil {
				t.Fatalf("expected command failure, got output %q", output)
			}
			if !strings.Contains(output, "USAGE:") {
				t.Fatalf("expected command help, got %q", output)
			}
		})
	}
}

func TestFilesystemGuardHelperProcess(_ *testing.T) {
	if os.Getenv("MC_TEST_FILESYSTEM_GUARD_HELPER") != "1" {
		return
	}

	separator := -1
	for i, arg := range os.Args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator == -1 {
		os.Exit(2)
	}

	app := registerApp("mc")
	if err := app.Run(append([]string{"mc"}, os.Args[separator+1:]...)); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func runFilesystemGuardCommand(t *testing.T, args []string, input ...string) (string, error) {
	t.Helper()
	configDir := t.TempDir()
	commandArgs := []string{"-test.run=TestFilesystemGuardHelperProcess", "--", args[0], "--no-color", "--config-dir", configDir}
	commandArgs = append(commandArgs, args[1:]...)
	cmd := exec.Command(os.Args[0], commandArgs...)
	cmd.Env = append(os.Environ(), "MC_TEST_FILESYSTEM_GUARD_HELPER=1")
	if len(input) > 0 {
		cmd.Stdin = strings.NewReader(input[0])
	}
	output, err := cmd.CombinedOutput()
	return string(output), err
}
