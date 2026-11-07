// Package realtime serves the org event stream (lane C, R-IN-3): GET /orgs/{orgId}/events
// (operationId streamOrgEvents) is hand-mounted outside the request timeout because it is a
// long-lived SSE response; the guard has already resolved the caller's membership, so the
// stream only ever carries the principal's org (T). Producers publish with
// platform/realtime.Publish inside their own transactions.
package realtime
