package auditexport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/secure"
	"github.com/SamuelSupe/contextGate/internal/store"
)

const stateKey = "audit_otlp_v1"

type Status struct {
	State       string     `json:"state"`
	Pending     int64      `json:"pending"`
	Accepted    int64      `json:"accepted"`
	Rejected    int64      `json:"rejected"`
	LastAttempt *time.Time `json:"last_attempt,omitempty"`
	LastSuccess *time.Time `json:"last_success,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
	NextAttempt *time.Time `json:"next_attempt,omitempty"`
}

type View struct {
	Config            Config `json:"config"`
	HeadersConfigured bool   `json:"headers_configured"`
	Status            Status `json:"status"`
}

type checkpoint struct {
	Config   Config `json:"config"`
	Instance string `json:"instance"`
	Cursor   int64  `json:"cursor"`
	Status   Status `json:"status"`
	Blocked  bool   `json:"blocked"`
	Failures int    `json:"failures"`
}

type Manager struct {
	store    *store.Store
	mu       sync.Mutex
	state    checkpoint
	sender   *sender
	cancel   context.CancelFunc
	inFlight context.CancelFunc
	sending  bool
	closed   bool
	done     chan struct{}
	wake     chan struct{}
}

func New(st *store.Store) (*Manager, error) {
	m := &Manager{store: st, done: make(chan struct{}), wake: make(chan struct{}, 1)}
	value, err := st.Get(stateKey)
	if err == nil {
		b, err := st.Vault.Open(value, stateKey)
		if err != nil {
			return nil, errors.New("Could not decrypt audit export configuration.")
		}
		if json.Unmarshal(b, &m.state) != nil {
			return nil, errors.New("Invalid stored audit export configuration.")
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	m.state.Config, err = normalize(m.state.Config)
	if err != nil {
		return nil, errors.New("Invalid stored audit export configuration.")
	}
	if m.state.Instance == "" {
		m.state.Instance = secure.Random(16)
	}
	if m.state.Config.Enabled {
		m.sender, err = newSender(m.state.Config)
		if err != nil {
			return nil, errors.New("Could not initialize audit log exporter.")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	go m.run(ctx)
	return m, nil
}

func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	m.cancel()
	if m.inFlight != nil {
		m.inFlight()
	}
	m.mu.Unlock()
	<-m.done
	m.mu.Lock()
	if m.sender != nil {
		m.sender.close()
	}
	m.mu.Unlock()
}

func (m *Manager) View(ctx context.Context) (View, error) {
	m.mu.Lock()
	state, sending := m.state, m.sending
	m.mu.Unlock()
	v := View{Config: state.Config, HeadersConfigured: len(state.Config.Headers) > 0, Status: state.Status}
	v.Config.Headers = nil
	switch {
	case !state.Config.Enabled:
		v.Status.State = "disabled"
	case state.Blocked:
		v.Status.State = "blocked"
	case sending:
		v.Status.State = "exporting"
	case state.Status.NextAttempt != nil:
		v.Status.State = "retrying"
	case state.Status.LastError != "":
		v.Status.State = "warning"
	default:
		v.Status.State = "ready"
	}
	if state.Config.Enabled {
		err := m.store.DB.QueryRowContext(ctx, "SELECT count(*) FROM audit WHERE id>$1", state.Cursor).Scan(&v.Status.Pending)
		if err != nil {
			return View{}, err
		}
	}
	return v, nil
}

func (m *Manager) candidate(update Update) (Config, error) {
	if update.Revision != m.state.Config.Revision {
		return Config{}, model.Fail("conflict", "Audit export settings changed. Reload them before editing.")
	}
	if update.ClearHeaders && len(update.Headers) > 0 {
		return Config{}, model.Fail("invalid_input", "Choose new headers or clear stored headers, not both.")
	}
	if update.Headers == nil && !update.ClearHeaders {
		update.Headers = m.state.Config.Headers
	}
	if update.ClearHeaders {
		update.Headers = nil
	}
	config, err := normalize(update.Config)
	if err != nil {
		return Config{}, model.Fail("invalid_input", err.Error())
	}
	return config, nil
}

func (m *Manager) Update(ctx context.Context, update Update) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("Audit exporter is shutting down.")
	}
	config, err := m.candidate(update)
	if err != nil {
		return err
	}
	var client *sender
	if config.Enabled {
		client, err = newSender(config)
		if err != nil {
			return model.Fail("invalid_input", "Could not initialize the OTLP transport.")
		}
	}
	committed := false
	defer func() {
		if !committed && client != nil {
			client.close()
		}
	}()
	next := m.state
	config.Revision++
	next.Config = config
	next.Blocked, next.Failures = false, 0
	next.Status.LastError = ""
	next.Status.NextAttempt = nil
	next.Status.LastAttempt = nil
	next.Status.LastSuccess = nil
	tx, err := m.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Enable starts at the current tail, so old and disabled-period audits are
	// never disclosed later. Updating an enabled destination preserves its backlog.
	if !m.state.Config.Enabled || !config.Enabled {
		if err = tx.QueryRowContext(ctx, "SELECT coalesce(max(id),0) FROM audit").Scan(&next.Cursor); err != nil {
			return err
		}
	}
	b, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO kv(key,value) VALUES($1,$2) ON CONFLICT(key) DO UPDATE SET value=excluded.value", stateKey, m.store.Vault.Seal(b, stateKey)); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if m.inFlight != nil {
		m.inFlight()
	}
	if m.sender != nil {
		m.sender.close()
	}
	m.state, m.sender = next, client
	committed = true
	select {
	case m.wake <- struct{}{}:
	default:
	}
	return nil
}

func (m *Manager) persist(ctx context.Context, next checkpoint) error {
	b, err := json.Marshal(next)
	if err != nil {
		return err
	}
	_, err = m.store.DB.ExecContext(ctx, "INSERT INTO kv(key,value) VALUES($1,$2) ON CONFLICT(key) DO UPDATE SET value=excluded.value", stateKey, m.store.Vault.Seal(b, stateKey))
	return err
}

type TestResult struct {
	Accepted  bool      `json:"accepted"`
	Message   string    `json:"message"`
	CheckedAt time.Time `json:"checked_at"`
}

func (m *Manager) Test(ctx context.Context, update Update) (TestResult, error) {
	m.mu.Lock()
	update.Enabled = true
	config, err := m.candidate(update)
	instance := m.state.Instance
	m.mu.Unlock()
	if err != nil {
		return TestResult{}, err
	}
	client, err := newSender(config)
	if err != nil {
		return TestResult{}, model.Fail("invalid_input", "Could not initialize the OTLP transport.")
	}
	defer client.close()
	result := client.export(ctx, testRequest(config, instance))
	test := TestResult{CheckedAt: time.Now().UTC()}
	if result.error != "" {
		test.Message = result.error
		return test, nil
	}
	if partial := result.response.GetPartialSuccess(); partial != nil && partial.GetRejectedLogRecords() != 0 {
		test.Message = "Receiver rejected the test log record."
		return test, nil
	}
	test.Accepted = true
	test.Message = "Receiver accepted the test log record."
	if result.response.GetPartialSuccess().GetErrorMessage() != "" {
		test.Message = "Receiver accepted the test record with an OTLP warning."
	}
	return test, nil
}
