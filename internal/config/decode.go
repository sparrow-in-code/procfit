package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	yaml "gopkg.in/yaml.v3"
)

// Format identifies a configuration encoding.
type Format string

const (
	FormatTOML Format = "toml"
	FormatYAML Format = "yaml"
	FormatJSON Format = "json"
)

// FormatFromExt maps a file extension to a Format.
func FormatFromExt(ext string) (Format, bool) {
	switch strings.ToLower(strings.TrimPrefix(ext, ".")) {
	case "toml":
		return FormatTOML, true
	case "yaml", "yml":
		return FormatYAML, true
	case "json":
		return FormatJSON, true
	default:
		return "", false
	}
}

// decodeRaw strictly decodes bytes in the given format into a rawConfig.
// "Strict" means unknown fields, duplicate keys, and type mismatches are errors
// so that identical inputs across formats produce identical results (RFC §9.4).
func decodeRaw(data []byte, format Format) (rawConfig, error) {
	var raw rawConfig
	switch format {
	case FormatTOML:
		dec := toml.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&raw); err != nil {
			return raw, fmt.Errorf("toml: %w", err)
		}
	case FormatYAML:
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		if err := dec.Decode(&raw); err != nil {
			return raw, fmt.Errorf("yaml: %w", err)
		}
	case FormatJSON:
		if err := checkJSONDuplicates(data); err != nil {
			return raw, fmt.Errorf("json: %w", err)
		}
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&raw); err != nil {
			return raw, fmt.Errorf("json: %w", err)
		}
	default:
		return raw, fmt.Errorf("unsupported config format %q", format)
	}
	return raw, nil
}

// checkJSONDuplicates rejects duplicate object keys, which the standard library
// silently accepts (last wins). JSON must get JSON-appropriate strictness
// (RFC §9.1).
func checkJSONDuplicates(data []byte) error {
	return walkJSONValue(json.NewDecoder(bytes.NewReader(data)))
}

func walkJSONValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch d {
	case '{':
		return walkJSONObject(dec)
	case '[':
		return walkJSONArray(dec)
	default:
		return nil
	}
}

func walkJSONObject(dec *json.Decoder) error {
	seen := map[string]bool{}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return err
		}
		key := kt.(string)
		if seen[key] {
			return fmt.Errorf("duplicate key %q", key)
		}
		seen[key] = true
		if err := walkJSONValue(dec); err != nil {
			return err
		}
	}
	_, err := dec.Token() // consume closing }
	return err
}

func walkJSONArray(dec *json.Decoder) error {
	for dec.More() {
		if err := walkJSONValue(dec); err != nil {
			return err
		}
	}
	_, err := dec.Token() // consume closing ]
	return err
}
