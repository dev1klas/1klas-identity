// Package argon2id implements user.PasswordHasher using the argon2id KDF.
package argon2id

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"

	"github.com/dev1klas/1klas-identity/internal/domain/user"
)

// Params are the argon2id parameters used to produce new hashes.
type Params struct {
	MemoryKiB   uint32 // m
	Time        uint32 // t
	Parallelism uint8  // p
	SaltLen     uint32
	KeyLen      uint32
}

// Default returns the CTO-mandated argon2id parameters (m=64MiB, t=3, p=1,
// salt=16, key=32).
func Default() Params {
	return Params{
		MemoryKiB:   64 * 1024,
		Time:        3,
		Parallelism: 1,
		SaltLen:     16,
		KeyLen:      32,
	}
}

// Hasher implements user.PasswordHasher.
type Hasher struct {
	params Params
}

// New returns a Hasher with the given params.
func New(p Params) *Hasher { return &Hasher{params: p} }

// Hash produces a PHC-formatted argon2id hash for raw.
func (h *Hasher) Hash(_ context.Context, raw string) (user.PasswordHash, error) {
	if len(raw) > user.PasswordMaxLen {
		return user.PasswordHash{}, errors.New("argon2id: password exceeds max length")
	}
	salt := make([]byte, h.params.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return user.PasswordHash{}, err
	}
	key := argon2.IDKey(
		[]byte(raw),
		salt,
		h.params.Time,
		h.params.MemoryKiB,
		h.params.Parallelism,
		h.params.KeyLen,
	)
	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		h.params.MemoryKiB,
		h.params.Time,
		h.params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
	return user.NewPasswordHash(encoded), nil
}

// Verify returns true iff raw matches the encoded hash. Always runs the KDF.
func (h *Hasher) Verify(_ context.Context, hash user.PasswordHash, raw string) (bool, error) {
	if len(raw) > user.PasswordMaxLen {
		return false, nil
	}
	parts := strings.Split(hash.String(), "$")
	// expected: ["", "argon2id", "v=19", "m=...,t=...,p=...", "<salt>", "<key>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("argon2id: malformed hash")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errors.New("argon2id: unsupported version")
	}

	var memory, time uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &parallelism); err != nil {
		return false, errors.New("argon2id: malformed params")
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, errors.New("argon2id: malformed salt")
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, errors.New("argon2id: malformed key")
	}

	got := argon2.IDKey([]byte(raw), salt, time, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
