package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// flattenKeys recursively walks a yaml.Node and emits dot-notation key paths.
// Lists are indexed as key[0], key[1], …
// It also identifies "extensible" keys (empty maps/sequences) which allow any nested keys.
func flattenKeys(node *yaml.Node, prefix string, out map[string]struct{}, extensible map[string]struct{}) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			flattenKeys(child, prefix, out, extensible)
		}

	case yaml.MappingNode:
		if len(node.Content) == 0 && prefix != "" {
			extensible[prefix] = struct{}{}
		}
		// MappingNode children alternate: key, value, key, value, …
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]

			var fullKey string
			if prefix == "" {
				fullKey = keyNode.Value
			} else {
				fullKey = prefix + "." + keyNode.Value
			}

			out[fullKey] = struct{}{}
			flattenKeys(valNode, fullKey, out, extensible)
		}

	case yaml.SequenceNode:
		if len(node.Content) == 0 && prefix != "" {
			extensible[prefix] = struct{}{}
		}
		for i, child := range node.Content {
			indexedKey := fmt.Sprintf("%s[%d]", prefix, i)
			out[indexedKey] = struct{}{}
			flattenKeys(child, indexedKey, out, extensible)
		}

		// ScalarNode: leaf value — nothing to recurse into
	}
}

// parseYAML unmarshals YAML data and flattens it into a set of dot-notation keys.
// It returns a map of all found keys and a map of keys that are "extensible" (empty maps/sequences).
func parseYAML(data []byte) (map[string]struct{}, map[string]struct{}, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, nil, err
	}

	keys := make(map[string]struct{})
	extensible := make(map[string]struct{})
	flattenKeys(&doc, "", keys, extensible)
	return keys, extensible, nil
}

// loadKeys reads a YAML file from disk (or stdin if path is "-") and parses its keys.
func loadKeys(path string) (map[string]struct{}, map[string]struct{}, error) {
	var data []byte
	var err error

	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", path, err)
	}

	keys, extensible, err := parseYAML(data)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	return keys, extensible, nil
}

// setDiff calculates the difference between override keys and chart keys.
// It filters out keys that are allowed due to a parent being "extensible" in the chart.
func setDiff(override, chart map[string]struct{}, extensible map[string]struct{}) []string {
	var diff []string
	for k := range override {
		if _, ok := chart[k]; ok {
			continue
		}

		// Walk up the key path to see if any parent is "extensible"
		isAllowed := false
		p := k
		for {
			idx := strings.LastIndexAny(p, ".[")
			if idx == -1 {
				break
			}
			p = p[:idx]
			if _, ok := extensible[p]; ok {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			diff = append(diff, k)
		}
	}
	sort.Strings(diff)
	return diff
}

// usage prints the command-line usage information to stderr.
func usage() {
	fmt.Fprintf(os.Stderr, `checkvalues — verify that all keys in an override values file
exist in a Helm chart's default values.yaml.

Usage:
  checkvalues [flags] <override.yaml> <chart-values.yaml>

Flags:
  -allowlist string    path to a YAML file containing extra valid keys
  -h, --help           show this help

Exit codes:
  0  all keys present
  1  unknown keys found
  2  usage / IO error
`)
}

// main is the entry point of the program. It parses flags, loads files, and prints the result.
func main() {
	allowlistPath := flag.String("allowlist", "", "path to a YAML file containing extra valid keys")
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()

	if len(args) != 2 {
		if len(args) != 0 {
			fmt.Fprintln(os.Stderr, "error: expected exactly two arguments")
		}
		usage()
		if len(args) == 0 {
			os.Exit(0)
		}
		os.Exit(2)
	}

	overrideFile := args[0]
	chartFile := args[1]

	overrideKeys, _, err := loadKeys(overrideFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	chartKeys, chartExtensible, err := loadKeys(chartFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	// Load allowlist if provided and merge into chart keys/extensible maps
	if *allowlistPath != "" {
		extraKeys, extraExtensible, err := loadKeys(*allowlistPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading allowlist: %v\n", err)
			os.Exit(2)
		}
		for k := range extraKeys {
			chartKeys[k] = struct{}{}
		}
		for k := range extraExtensible {
			chartExtensible[k] = struct{}{}
		}
	}

	unknown := setDiff(overrideKeys, chartKeys, chartExtensible)

	if len(unknown) == 0 {
		fmt.Printf("✅  All %d key(s) in %s exist in %s\n",
			len(overrideKeys), overrideFile, chartFile)
		os.Exit(0)
	}

	fmt.Printf("❌  %d key(s) in %s not found in %s:\n\n",
		len(unknown), overrideFile, chartFile)

	// Group keys by their top-level parent for readability
	grouped := make(map[string][]string)
	var topLevel []string

	for _, k := range unknown {
		dot := strings.IndexByte(k, '.')
		bracket := strings.IndexByte(k, '[')

		var top string
		switch {
		case dot == -1 && bracket == -1:
			top = k
		case dot != -1 && (bracket == -1 || dot < bracket):
			top = k[:dot]
		default:
			top = k[:bracket]
		}

		if _, seen := grouped[top]; !seen {
			topLevel = append(topLevel, top)
		}
		grouped[top] = append(grouped[top], k)
	}

	sort.Strings(topLevel)

	for _, top := range topLevel {
		keys := grouped[top]
		if len(keys) == 1 && keys[0] == top {
			// Simple top-level key with no children in the diff
			fmt.Printf("  %s\n", top)
		} else {
			fmt.Printf("  %s\n", top)
			for _, k := range keys {
				if k != top {
					fmt.Printf("    %s\n", k)
				}
			}
		}
	}

	fmt.Println()
	os.Exit(1)
}
