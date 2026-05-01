package main

import (
	"embed"
	"path/filepath"
	"reflect"
	"testing"
)

//go:embed testdata/*
var testData embed.FS

func TestSetDiff(t *testing.T) {
	tests := []struct {
		name        string
		dir         string
		allowlist   string
		wantUnknown []string
	}{
		{
			name:        "exact match",
			dir:         "exact_match",
			wantUnknown: nil,
		},
		{
			name:        "simple typo",
			dir:         "simple_typo",
			wantUnknown: []string{"image.reppo"},
		},
		{
			name:        "empty map allows nested keys",
			dir:         "empty_map",
			wantUnknown: nil,
		},
		{
			name:        "empty sequence allows nested keys",
			dir:         "empty_seq",
			wantUnknown: nil,
		},
		{
			name:        "nested typo within existing structure",
			dir:         "nested_typo",
			wantUnknown: []string{"image.tga"},
		},
		{
			name:        "mixed known and unknown keys",
			dir:         "mixed_keys",
			wantUnknown: []string{"global.unknown"},
		},
		{
			name:        "allowlist feature",
			dir:         "allowlist",
			allowlist:   "allowlist.yaml",
			wantUnknown: []string{"typoKey"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chartPath := filepath.Join("testdata", tt.dir, "chart.yaml")
			chartData, err := testData.ReadFile(chartPath)
			if err != nil {
				t.Fatalf("failed to read chart file %s: %v", chartPath, err)
			}

			overridePath := filepath.Join("testdata", tt.dir, "override.yaml")
			overrideData, err := testData.ReadFile(overridePath)
			if err != nil {
				t.Fatalf("failed to read override file %s: %v", overridePath, err)
			}

			chartKeys, chartExtensible, err := parseYAML(chartData)
			if err != nil {
				t.Fatalf("failed to parse chart: %v", err)
			}

			if tt.allowlist != "" {
				allowlistPath := filepath.Join("testdata", tt.dir, tt.allowlist)
				allowlistData, err := testData.ReadFile(allowlistPath)
				if err != nil {
					t.Fatalf("failed to read allowlist file %s: %v", allowlistPath, err)
				}
				extraKeys, extraExtensible, err := parseYAML(allowlistData)
				if err != nil {
					t.Fatalf("failed to parse allowlist: %v", err)
				}
				for k := range extraKeys {
					chartKeys[k] = struct{}{}
				}
				for k := range extraExtensible {
					chartExtensible[k] = struct{}{}
				}
			}

			overrideKeys, _, err := parseYAML(overrideData)
			if err != nil {
				t.Fatalf("failed to parse override: %v", err)
			}

			got := setDiff(overrideKeys, chartKeys, chartExtensible)
			if !reflect.DeepEqual(got, tt.wantUnknown) {
				t.Errorf("setDiff() = %v, want %v", got, tt.wantUnknown)
			}
		})
	}
}
