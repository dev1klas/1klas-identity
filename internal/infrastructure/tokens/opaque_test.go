package tokens_test

import (
	"testing"

	"github.com/dev1klas/1klas-identity/internal/infrastructure/tokens"
)

func TestNewTokenIsUniqueAndOpaque(t *testing.T) {
	g := tokens.New()
	a, err := g.NewToken()
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	b, err := g.NewToken()
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if a.String() == b.String() {
		t.Fatal("two consecutive tokens collided")
	}
	if len(a.String()) < 32 {
		t.Fatalf("token too short: %d", len(a.String()))
	}
}
