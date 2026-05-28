package user_test

import (
	"strings"
	"testing"

	"github.com/dev1klas/1klas-identity/internal/domain/user"
)

func TestNewEmail_Normalises(t *testing.T) {
	cases := []struct {
		in, out string
	}{
		{"Foo@Example.com", "foo@example.com"},
		{"  test@example.com  ", "test@example.com"},
	}
	for _, c := range cases {
		e, err := user.NewEmail(c.in)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if e.String() != c.out {
			t.Fatalf("got %q want %q", e.String(), c.out)
		}
	}
}

func TestNewEmail_Rejects(t *testing.T) {
	bad := []string{
		"",
		"no-at",
		"bad@",
		"@bad",
		"a@b",
		strings.Repeat("a", 250) + "@x.io",
	}
	for _, s := range bad {
		if _, err := user.NewEmail(s); err == nil {
			t.Fatalf("expected error for %q", s)
		}
	}
}

func TestPasswordPolicy(t *testing.T) {
	if err := user.ValidatePasswordPolicy("short"); err == nil {
		t.Fatal("expected weak-password error for short input")
	}
	if err := user.ValidatePasswordPolicy("correct horse battery staple"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}
