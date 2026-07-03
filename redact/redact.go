// Package redact masks secret-like environment values before they are
// rendered or logged. Same approach as ctop's redact package, written
// fresh for actop.
package redact

import "regexp"

// Redacted is the placeholder shown in place of a secret value.
const Redacted = "[REDACTED]"

// secretKeyPattern matches env var names that commonly hold credentials.
var secretKeyPattern = regexp.MustCompile(`(?i)(secret|passwd|password|token|api_?key|access_?key|private_?key|credential|auth|dsn|cert|cookie|jwt)`)

// credentialValuePattern matches connection-string values that embed
// credentials regardless of the key name, e.g. postgres://user:pass@host/db.
var credentialValuePattern = regexp.MustCompile(`://[^/@\s]+:[^@\s]+@`)

// Env returns the value of a single KEY=VALUE pair, masked if the key looks
// secret-like or the value embeds credentials.
func Env(key, value string) string {
	if value == "" {
		return value
	}
	if secretKeyPattern.MatchString(key) || credentialValuePattern.MatchString(value) {
		return Redacted
	}
	return value
}
