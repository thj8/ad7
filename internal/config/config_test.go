package config

import (
	"os"
	"testing"
)

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "config_test_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	f.WriteString(content)
	f.Close()
	return f.Name()
}

func TestLoad_JWTSecretRequired(t *testing.T) {
	path := writeTestConfig(t, `
server:
  port: 8080
db:
  host: localhost
  port: 3306
  user: root
  password: test
  dbname: ctf
`)
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for missing jwt.secret")
	}
}

func TestLoad_JWTSecretTooShort(t *testing.T) {
	path := writeTestConfig(t, `
server:
  port: 8080
db:
  host: localhost
  port: 3306
  user: root
  password: test
  dbname: ctf
jwt:
  secret: "short"
`)
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for short jwt.secret")
	}
}

func TestLoad_JWTSecretDefault(t *testing.T) {
	path := writeTestConfig(t, `
server:
  port: 8080
db:
  host: localhost
  port: 3306
  user: root
  password: test
  dbname: ctf
jwt:
  secret: "change-me-in-production"
`)
	_, err := Load(path)
	if err == nil {
		t.Error("SECURITY: expected error for known-default jwt.secret")
	}
}

func TestLoad_JWTSecretValid(t *testing.T) {
	path := writeTestConfig(t, `
server:
  port: 8080
db:
  host: localhost
  port: 3306
  user: root
  password: test
  dbname: ctf
jwt:
  secret: "a-valid-production-secret-with-sufficient-length"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error for valid secret, got: %v", err)
	}
	if cfg.JWT.Secret != "a-valid-production-secret-with-sufficient-length" {
		t.Errorf("secret mismatch")
	}
}
