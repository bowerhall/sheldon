package approval

import (
	"context"
	"testing"
	"time"
)

func TestApprovalFlow(t *testing.T) {
	mgr := NewManager(100 * time.Millisecond)

	chatID := int64(123)
	userID := int64(456)

	id := mgr.Start(chatID, userID, "deploy_app", `{"name":"test"}`, "Deploy test")
	if id == "" {
		t.Fatal("expected non-empty approval ID")
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		if err := mgr.Resolve(id, true, userID); err != nil {
			t.Errorf("resolve failed: %v", err)
		}
	}()

	approved, err := mgr.Wait(context.Background(), id)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if !approved {
		t.Error("expected approved=true")
	}
}

func TestApprovalDeny(t *testing.T) {
	mgr := NewManager(100 * time.Millisecond)

	id := mgr.Start(123, 456, "remove_app", `{}`, "Remove app")

	go func() {
		time.Sleep(10 * time.Millisecond)
		mgr.Resolve(id, false, 456)
	}()

	approved, err := mgr.Wait(context.Background(), id)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if approved {
		t.Error("expected approved=false")
	}
}

func TestApprovalTimeout(t *testing.T) {
	mgr := NewManager(20 * time.Millisecond)

	id := mgr.Start(123, 456, "deploy_app", `{}`, "Deploy")

	_, err := mgr.Wait(context.Background(), id)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestApprovalUserMismatch(t *testing.T) {
	mgr := NewManager(100 * time.Millisecond)

	id := mgr.Start(123, 456, "deploy_app", `{}`, "Deploy")

	err := mgr.Resolve(id, true, 999)
	if err != ErrUserMismatch {
		t.Errorf("expected ErrUserMismatch, got %v", err)
	}
}

func TestApprovalCancel(t *testing.T) {
	mgr := NewManager(100 * time.Millisecond)

	id := mgr.Start(123, 456, "deploy_app", `{}`, "Deploy")
	mgr.Cancel(id)

	_, err := mgr.Get(id)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound after cancel, got %v", err)
	}
}

func TestApprovalContextCancel(t *testing.T) {
	mgr := NewManager(5 * time.Second)

	id := mgr.Start(123, 456, "deploy_app", `{}`, "Deploy")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := mgr.Wait(ctx, id)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
