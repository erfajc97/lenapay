// Package openapi embeds the contract, so the request validator, the guard and the
// route-marker test read the very file clients read. api/openapi.yaml is a byte-for-byte copy
// of contracts/openapi.yaml (P1) made by `make gen`; TestSpecIsTheContract keeps them equal.
package openapi

import (
	"embed"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

//go:embed openapi.yaml
var files embed.FS

// Load parses and validates the spec. Each call returns a fresh copy.
func Load() (*openapi3.T, error) {
	data, err := files.ReadFile("openapi.yaml")
	if err != nil {
		return nil, fmt.Errorf("openapi: read: %w", err)
	}
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(data)
	if err != nil {
		return nil, fmt.Errorf("openapi: parse: %w", err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		return nil, fmt.Errorf("openapi: invalid: %w", err)
	}
	doc.Servers = nil // match on paths only, whatever host serves the API
	return doc, nil
}
