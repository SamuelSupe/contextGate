package auditexport

import (
	"context"
	"math/rand/v2"
	"time"
)

func (m *Manager) run(ctx context.Context) {
	defer close(m.done)
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.wake:
		case <-timer.C:
		}
		timer.Reset(m.cycle(ctx))
	}
}

func (m *Manager) cycle(root context.Context) time.Duration {
	m.mu.Lock()
	snapshot, client := m.state, m.sender
	if m.closed || !snapshot.Config.Enabled || snapshot.Blocked {
		m.mu.Unlock()
		return time.Minute
	}
	if next := snapshot.Status.NextAttempt; next != nil && time.Until(*next) > 0 {
		m.mu.Unlock()
		return time.Until(*next)
	}
	ctx, cancel := context.WithCancel(root)
	m.inFlight = cancel
	m.sending = true
	m.mu.Unlock()
	defer cancel()
	audits, err := m.store.AuditBatch(ctx, snapshot.Cursor)
	result := delivery{}
	count := 0
	if err != nil {
		result = delivery{error: "Local audit storage unavailable", retry: true}
	} else if len(audits) > 0 {
		request, n := auditRequest(snapshot.Config, snapshot.Instance, audits)
		count = n
		result = client.export(ctx, request)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inFlight = nil
	m.sending = false
	if m.closed || ctx.Err() != nil || m.state.Config.Revision != snapshot.Config.Revision {
		return time.Second
	}
	if err == nil && count == 0 {
		return 2 * time.Second
	}
	next := m.state
	now := time.Now().UTC()
	next.Status.LastAttempt = &now
	next.Status.LastError = result.error
	delay := time.Second
	if result.retry {
		delay = time.Second * time.Duration(1<<min(next.Failures, 5))
		delay += time.Duration(float64(delay) * rand.Float64() * 0.2)
		delay = max(delay, min(result.retryAfter, 24*time.Hour))
		retryAt := now.Add(delay)
		next.Status.NextAttempt = &retryAt
		next.Failures = min(next.Failures+1, 6)
	} else {
		next.Cursor = audits[count-1].ID
		next.Status.NextAttempt = nil
		next.Failures = 0
		rejected := int64(0)
		if result.error != "" {
			rejected = int64(count)
			next.Blocked = true
		} else if partial := result.response.GetPartialSuccess(); partial != nil {
			rejected = partial.GetRejectedLogRecords()
			if rejected < 0 || rejected > int64(count) {
				rejected = int64(count)
			}
			if rejected > 0 {
				next.Status.LastError = "Receiver partially rejected this batch; rejected records will not be retried."
			} else if partial.GetErrorMessage() != "" {
				next.Status.LastError = "Receiver returned an OTLP warning."
			}
		}
		next.Status.Rejected += rejected
		next.Status.Accepted += int64(count) - rejected
		if int64(count) > rejected {
			next.Status.LastSuccess = &now
		}
		// Rejected batches are not replayed by OTLP. Keep the original local audit;
		// pause on permanent failures so subsequent records await corrected settings.
		if count == len(audits) && count < 64 {
			delay = 2 * time.Second
		} else {
			delay = 10 * time.Millisecond
		}
	}
	if err := m.persist(ctx, next); err != nil {
		m.state.Status.LastError = "Local export checkpoint unavailable; acknowledgement was not saved."
		retryAt := now.Add(5 * time.Second)
		m.state.Status.NextAttempt = &retryAt
		return 5 * time.Second
	}
	m.state = next
	return delay
}
