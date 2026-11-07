package app_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	openapi "github.com/erfajc97/lenapay/apps/api/api"
	"github.com/erfajc97/lenapay/apps/api/internal/app"
	"github.com/erfajc97/lenapay/apps/api/internal/platform/auth"
)

func hasHeader(op *openapi3.Operation, name string) bool {
	for _, p := range op.Parameters {
		if p.Value != nil && p.Value.In == "header" && p.Value.Name == name {
			return true
		}
	}
	return false
}

// A14/AG15: every operation of the contract has exactly one rule, no rule names an unknown
// operation, and `security: []` is declared exactly on the operations whose rule needs no token.
func TestAccessRulesCoverTheSpec(t *testing.T) {
	t.Parallel()
	spec, err := openapi.Load()
	if err != nil {
		t.Fatal(err)
	}
	rules := app.AccessRules()
	seen := map[string]bool{}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			id := op.OperationID
			seen[id] = true
			rule, ok := rules[id]
			if !ok || rule.IsDeny() {
				t.Errorf("%s %s (%s) has no access rule", method, path, id)
				continue
			}
			security := spec.Security
			if op.Security != nil {
				security = *op.Security
			}
			var schemes []string
			for _, req := range security {
				for name := range req {
					schemes = append(schemes, name)
				}
			}
			switch {
			case slices.Contains(schemes, "internalApiKey"):
				if !rule.IsInternal() {
					t.Errorf("%s: internalApiKey in the contract but not an Internal rule", id)
				}
			case len(schemes) == 0:
				if rule.NeedsToken() || rule.IsInternal() {
					t.Errorf("%s: security: [] in the contract but the rule needs credentials", id)
				}
			default:
				if !rule.NeedsToken() {
					t.Errorf("%s: bearer in the contract but the rule needs no token", id)
				}
			}
			if strings.HasPrefix(path, "/orgs/{orgId}") != rule.IsOrg() {
				t.Errorf("%s: org path %v but org rule %v", id, strings.HasPrefix(path, "/orgs/{orgId}"), rule.IsOrg())
			}
			if rule.IsOrg() && !slices.Contains(auth.Roles(), rule.MinRole()) {
				t.Errorf("%s: org rule without a valid minimum role", id)
			}
			if hasHeader(op, auth.HeaderStepUp) != rule.NeedsStepUp() {
				t.Errorf("%s: X-Step-Up-Token in contract %v, step-up rule %v", id, hasHeader(op, auth.HeaderStepUp), rule.NeedsStepUp())
			}
			if hasHeader(op, "Idempotency-Key") && !rule.NeedsStepUp() {
				t.Errorf("%s: money mutations (Idempotency-Key) also need step-up", id)
			}
		}
	}
	for id := range rules {
		if !seen[id] {
			t.Errorf("rule for unknown operation %q", id)
		}
	}
}
