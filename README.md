# checkvalues

A lightweight CLI tool to verify that all keys in a Helm override values file exist in the chart's default `values.yaml`. It helps catch typos and deprecated keys before they cause issues during deployment.

> [!NOTE]
> This repository was built entirely using **Gemini CLI**.

## Features

- **Recursive Key Flattening**: Supports deep nested structures and list indexing (`key[0].item`).
- **Extensible Keys**: Automatically allows any nested keys under empty maps (`{}`) or sequences (`[]`) in the chart (e.g., `resources: {}`, `tolerations: []`).
- **Allowlist Support**: Merge supplementary "hidden" keys from a separate file.
- **Automated CI**: Includes GitHub Actions for continuous verification.

## Installation

Install the CLI tool using `go install`:

```bash
go install github.com/mosanden/checkvalues@latest
```

## Usage

```bash
helm pull --untar stefanprodan/podinfo
checkvalues [flags] <override.yaml> podinfo/values.yaml
```
or
```bash
helm show values stefanprodan/podinfo | checkvalues [flags] <override.yaml> -
```


### Flags
- `-allowlist <path>`: Path to a YAML file containing extra valid keys that aren't in the chart's `values.yaml`.

### Example
```bash
checkvalues -allowlist extra-keys.yaml my-values.yaml chart/values.yaml
```

## Testing

Run the comprehensive test suite (powered by Go's `embed` package):

```bash
go test -v .
```

## Exit Codes
- `0`: All keys present and valid.
- `1`: Unknown keys found.
- `2`: Usage or IO error.
