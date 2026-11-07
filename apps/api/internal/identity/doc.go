// Package identity is Lebane ID (lane A, R-ID-1…4): signup with an emailed 6-digit code,
// login with rotating refresh tokens and optional TOTP, device keys, step-up challenges and
// the local identity provider's JWKS. It owns users, sessions, refresh_tokens, devices,
// email_codes and step_up_challenges.
//
// Dependencies point INTO this package: orgs calls Service.UserIDByEmail; identity reaches
// memberships and email delivery only through the ports in port.go (wired in cmd/api).
package identity
