package openapi_test

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	openapi "lenapay/apps/api/api"
)

// P1: the embedded copy is exactly contracts/openapi.yaml (run `make gen` after editing it).
func TestSpecIsTheContract(t *testing.T) {
	t.Parallel()
	contract, err := os.ReadFile("../../../contracts/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(contract, []byte("\r\n"), []byte("\n")), bytes.ReplaceAll(embedded, []byte("\r\n"), []byte("\n"))) {
		t.Fatal("apps/api/api/openapi.yaml differs from contracts/openapi.yaml: run make gen")
	}
}

// AG3: every request object rejects unknown keys, every string has maxLength (or is an enum,
// uuid, date or a binary upload capped by the body limit), every array maxItems, every number
// a maximum.
func TestRequestSchemasAreCapped(t *testing.T) {
	t.Parallel()
	spec, err := openapi.Load()
	if err != nil {
		t.Fatal(err)
	}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			for _, p := range op.Parameters {
				checkSchema(t, fmt.Sprintf("%s %s param %s", method, path, p.Value.Name), p.Value.Schema)
			}
			if op.RequestBody == nil {
				continue
			}
			for contentType, media := range op.RequestBody.Value.Content {
				checkSchema(t, fmt.Sprintf("%s %s %s", method, path, contentType), media.Schema)
			}
		}
	}
}

func checkSchema(t *testing.T, where string, ref *openapi3.SchemaRef) {
	t.Helper()
	if ref == nil || ref.Value == nil {
		return
	}
	s := ref.Value
	switch {
	case s.Type.Is("object"):
		if s.AdditionalProperties.Has == nil || *s.AdditionalProperties.Has {
			t.Errorf("%s: object without additionalProperties: false", where)
		}
		for name, prop := range s.Properties {
			checkSchema(t, where+"."+name, prop)
		}
	case s.Type.Is("string"):
		bounded := s.MaxLength != nil || len(s.Enum) > 0 ||
			s.Format == "uuid" || s.Format == "date" || s.Format == "date-time" || s.Format == "binary"
		if !bounded {
			t.Errorf("%s: string without maxLength", where)
		}
	case s.Type.Is("array"):
		if s.MaxItems == nil {
			t.Errorf("%s: array without maxItems", where)
		}
		checkSchema(t, where+"[]", s.Items)
	case s.Type.Is("integer"), s.Type.Is("number"):
		if s.Max == nil {
			t.Errorf("%s: number without maximum", where)
		}
	}
}

// Every operation has an operationId and documents the error envelope.
func TestOperationsDeclareTheErrorEnvelope(t *testing.T) {
	t.Parallel()
	spec, err := openapi.Load()
	if err != nil {
		t.Fatal(err)
	}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			if op.OperationID == "" {
				t.Errorf("%s %s: no operationId", method, path)
			}
			if op.Responses.Default() == nil {
				t.Errorf("%s %s: no default (error envelope) response", method, path)
			}
		}
	}
}
