package config

import "testing"

func TestExpandEnvStringSimple(t *testing.T) {
	t.Setenv("ASDF", "hello")
	out, err := ExpandEnvString("${ASDF}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hello" {
		t.Fatalf("expected hello, got %q", out)
	}
}

func TestExpandEnvStringDefaultWhenUnset(t *testing.T) {
	out, err := ExpandEnvString("${ASDF:-fallback}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "fallback" {
		t.Fatalf("expected fallback, got %q", out)
	}
}

func TestExpandEnvStringDefaultWhenEmpty(t *testing.T) {
	t.Setenv("ASDF", "")
	out, err := ExpandEnvString("${ASDF:-fallback}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "fallback" {
		t.Fatalf("expected fallback, got %q", out)
	}
}

func TestExpandEnvStringMultiple(t *testing.T) {
	t.Setenv("USER_A", "alpha")
	t.Setenv("USER_B", "beta")
	out, err := ExpandEnvString("x-${USER_A}-y-${USER_B}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "x-alpha-y-beta" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestExpandEnvStringUnresolved(t *testing.T) {
	_, err := ExpandEnvString("${MISSING_ENV}")
	if err == nil {
		t.Fatal("expected unresolved env error")
	}
}

func TestExpandEnvStringInvalidPlaceholder(t *testing.T) {
	_, err := ExpandEnvString("${123BAD}")
	if err == nil {
		t.Fatal("expected invalid placeholder error")
	}
}
