package engine

import (
	"context"
	"github.com/SamuelSupe/contextGate/internal/model"
)

// Reserve all three limits together. A request waiting for its Agent or source
// must not hold global capacity needed by unrelated queries or connection opens.
func (e *Engine) admit(ctx context.Context, agent string, source model.Source) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		e.mu.Lock()
		if e.active < 32 && e.activeAgents[agent] < 4 && e.activeSources[source.ID] < source.Limits.Concurrency {
			e.active++
			e.activeAgents[agent]++
			e.activeSources[source.ID]++
			e.mu.Unlock()
			return nil
		}
		changed := e.capacityChanged
		e.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}

func (e *Engine) releaseAdmission(agent, source string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.active--
	e.activeAgents[agent]--
	e.activeSources[source]--
	if e.activeAgents[agent] == 0 {
		delete(e.activeAgents, agent)
	}
	if e.activeSources[source] == 0 {
		delete(e.activeSources, source)
	}
	close(e.capacityChanged)
	e.capacityChanged = make(chan struct{})
}
