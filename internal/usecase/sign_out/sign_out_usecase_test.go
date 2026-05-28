package sign_out_test

import (
	"context"
	"testing"
	"time"

	"github.com/dev1klas/1klas-identity/internal/domain/outbox"
	"github.com/dev1klas/1klas-identity/internal/domain/session"
	"github.com/dev1klas/1klas-identity/internal/domain/tenant"
	"github.com/dev1klas/1klas-identity/internal/domain/user"
	"github.com/dev1klas/1klas-identity/internal/usecase/internal_testkit"
	"github.com/dev1klas/1klas-identity/internal/usecase/sign_out"

	"github.com/google/uuid"
)

func TestSignOut_RevokesAndEmits(t *testing.T) {
	sessions := internal_testkit.NewFakeSessions()
	out := internal_testkit.NewFakeOutbox()
	clk := &internal_testkit.FakeClock{T: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}

	uid := uuid.New()
	sid := uuid.New()
	s := session.New(sid, tenant.DefaultID, uid, []byte("hash"), clk.T, clk.T.Add(time.Hour))
	_ = sessions.SaveTx(context.Background(), user.NewTx(struct{}{}), s)

	uc := sign_out.New(internal_testkit.FakeUoW{}, sessions, out, clk)
	if _, err := uc.Execute(context.Background(), sign_out.Input{
		TenantID:  tenant.DefaultID,
		SessionID: sid,
		UserID:    uid,
	}); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(out.Events) != 1 || out.Events[0].EventType() != outbox.TopicSessionRevoked {
		t.Fatalf("expected one revoked event, got %v", out.Events)
	}
}
