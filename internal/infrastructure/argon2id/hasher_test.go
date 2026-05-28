package argon2id_test

import (
	"context"
	"testing"

	"github.com/dev1klas/1klas-identity/internal/infrastructure/argon2id"
)

func TestHashVerifyRoundTrip(t *testing.T) {
	// Smaller params for speed.
	h := argon2id.New(argon2id.Params{MemoryKiB: 8 * 1024, Time: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32})
	ctx := context.Background()

	hash, err := h.Hash(ctx, "correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	ok, err := h.Verify(ctx, hash, "correct horse battery staple")
	if err != nil || !ok {
		t.Fatalf("verify good password: ok=%v err=%v", ok, err)
	}

	ok, err = h.Verify(ctx, hash, "wrong password attempt!")
	if err != nil {
		t.Fatalf("verify bad password err: %v", err)
	}
	if ok {
		t.Fatal("verify accepted wrong password")
	}
}
