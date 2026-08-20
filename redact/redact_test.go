package redact

import "testing"

func TestEnv(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  string
	}{
		{"password key", "POSTGRES_PASSWORD", "hunter2", Redacted},
		{"token key", "GITHUB_TOKEN", "ghp_abc", Redacted},
		{"auth key", "BASIC_AUTH", "u:p", Redacted},
		{"jwt key", "JWT_SIGNING", "x", Redacted},
		{"sha key stays visible", "PG_SHA256", "abc123", "abc123"},
		{"bare pass suffix", "DB_PASS", "hunter2", Redacted},
		{"bare pwd suffix", "MYSQL_PWD", "hunter2", Redacted},
		{"pgpassword", "PGPASSWORD", "hunter2", Redacted},
		{"stripe key", "STRIPE_KEY", "sk_live_x", Redacted},
		{"signing key", "SIGNING_KEY", "x", Redacted},
		{"ssh key", "SSH_KEY", "x", Redacted},
		{"key file path", "KEY_FILE", "/etc/tls/server.key", Redacted},
		{"compass stays visible", "COMPASS_URL", "https://c.example", "https://c.example"},
		{"keyboard stays visible", "KEYBOARD_LAYOUT", "de", "de"},
		{"monkey stays visible", "MONKEY_NAME", "bobo", "bobo"},
		{"connection string value", "DATABASE_URL", "postgres://user:pass@db/app", Redacted},
		{"plain url", "ENDPOINT", "https://example.com/x", "https://example.com/x"},
		// Connection strings without a user part: the usual shape for Redis,
		// Celery and AMQP. Key names here deliberately do not match
		// secretKeyPattern, so only the value path can catch them.
		{"redis url without user", "CACHE_HOST", "redis://:pw@host", Redacted},
		{"amqp url without user", "BROKER", "amqp://:secret@rabbit:5672/vhost", Redacted},
		{"mongodb srv url", "SERVICE_URL", "mongodb+srv://a:b@c/d", Redacted},
		{"url with port stays visible", "SERVICE_URL", "http://host:8080/path", "http://host:8080/path"},
		{"user without password stays visible", "SERVICE_URL", "http://user@host/p", "http://user@host/p"},
		{"at sign in path stays visible", "ASSET_URL", "https://cdn.example.com/a@b.png", "https://cdn.example.com/a@b.png"},
		{"user stays visible", "POSTGRES_USER", "ownerp", "ownerp"},
		{"path", "PATH", "/usr/bin", "/usr/bin"},
		{"empty value", "SECRET", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Env(tt.key, tt.value); got != tt.want {
				t.Errorf("Env(%q, %q) = %q, want %q", tt.key, tt.value, got, tt.want)
			}
		})
	}
}
