package approval

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bowerhall/sheldon/internal/logger"
	"github.com/google/uuid"
)

var (
	ErrNotFound        = errors.New("approval not found")
	ErrUserMismatch    = errors.New("approving user does not match requester")
	ErrAlreadyResolved = errors.New("approval already resolved")
)

type ApprovalResult struct {
	Approved bool
	UserID   int64
}

type PendingApproval struct {
	ID          string
	ChatID      int64
	UserID      int64
	ToolName    string
	ToolArgs    string
	Description string
	CreatedAt   time.Time
	resultCh    chan ApprovalResult
	resolved    bool
}

type Manager struct {
	pending map[string]*PendingApproval
	mu      sync.RWMutex
	timeout time.Duration
}

func NewManager(timeout time.Duration) *Manager {
	return &Manager{
		pending: make(map[string]*PendingApproval),
		timeout: timeout,
	}
}

func (m *Manager) Start(chatID, userID int64, toolName, toolArgs, description string) string {
	id := uuid.New().String()[:8]

	approval := &PendingApproval{
		ID:          id,
		ChatID:      chatID,
		UserID:      userID,
		ToolName:    toolName,
		ToolArgs:    toolArgs,
		Description: description,
		CreatedAt:   time.Now(),
		resultCh:    make(chan ApprovalResult, 1),
	}

	m.mu.Lock()
	m.pending[id] = approval
	m.mu.Unlock()

	logger.Info("approval started", "id", id, "tool", toolName, "user", userID)
	return id
}

func (m *Manager) Wait(ctx context.Context, approvalID string) (bool, error) {
	m.mu.RLock()
	approval, ok := m.pending[approvalID]
	m.mu.RUnlock()

	if !ok {
		return false, ErrNotFound
	}

	defer func() {
		m.mu.Lock()
		delete(m.pending, approvalID)
		m.mu.Unlock()
	}()

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-time.After(m.timeout):
		logger.Info("approval timed out", "id", approvalID)
		return false, fmt.Errorf("approval timed out after %s", m.timeout)
	case result := <-approval.resultCh:
		return result.Approved, nil
	}
}

func (m *Manager) Request(ctx context.Context, chatID, userID int64, toolName, toolArgs, description string) (string, bool, error) {
	id := m.Start(chatID, userID, toolName, toolArgs, description)
	approved, err := m.Wait(ctx, id)
	return id, approved, err
}

func (m *Manager) Get(approvalID string) (*PendingApproval, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	approval, ok := m.pending[approvalID]
	if !ok {
		return nil, ErrNotFound
	}
	return approval, nil
}

func (m *Manager) Cancel(approvalID string) {
	m.mu.Lock()
	delete(m.pending, approvalID)
	m.mu.Unlock()
}

func (m *Manager) Resolve(approvalID string, approved bool, userID int64) error {
	m.mu.Lock()
	approval, ok := m.pending[approvalID]
	if !ok {
		m.mu.Unlock()
		return ErrNotFound
	}

	if approval.resolved {
		m.mu.Unlock()
		return ErrAlreadyResolved
	}

	if approval.UserID != userID {
		m.mu.Unlock()
		logger.Warn("approval user mismatch", "id", approvalID, "expected", approval.UserID, "got", userID)
		return ErrUserMismatch
	}

	approval.resolved = true
	m.mu.Unlock()

	result := ApprovalResult{
		Approved: approved,
		UserID:   userID,
	}

	select {
	case approval.resultCh <- result:
		logger.Info("approval resolved", "id", approvalID, "approved", approved, "user", userID)
	default:
		logger.Warn("approval channel full", "id", approvalID)
	}

	return nil
}

func (m *Manager) Timeout() time.Duration {
	return m.timeout
}
