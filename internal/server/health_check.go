package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/SamuelSupe/contextGate/internal/adapter"
	"github.com/SamuelSupe/contextGate/internal/engine"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/semantic"
)

func healthReferences(snapshot semantic.Snapshot) []semantic.Reference {
	refs := map[semantic.Reference]bool{}
	add := func(ref semantic.Reference) {
		if ref.Object != "" {
			ref.Field = ""
			refs[ref] = true
		}
	}
	for _, entry := range snapshot.Entries {
		if entry.Reference != nil {
			add(*entry.Reference)
		}
		for _, ref := range entry.Related {
			add(ref)
		}
	}
	if snapshot.Ontology != nil {
		for _, entity := range snapshot.Ontology.Entities {
			for _, ref := range entity.Objects {
				add(ref)
			}
		}
		for _, property := range snapshot.Ontology.Properties {
			if property.Reference != nil {
				add(*property.Reference)
			}
		}
		for _, relation := range snapshot.Ontology.Relations {
			for _, pair := range relation.Fields {
				add(pair.From)
				add(pair.To)
			}
		}
	}
	out := []semantic.Reference{}
	for ref := range refs {
		out = append(out, ref)
	}
	slices.SortFunc(out, func(a, b semantic.Reference) int {
		if a.Namespace < b.Namespace || a.Namespace == b.Namespace && a.Object < b.Object {
			return -1
		}
		return 1
	})
	return out
}

func (s *Server) checkSourceHealth(ctx context.Context, id string) (model.SourceHealth, error) {
	if !s.healthMu.TryLock() {
		return model.SourceHealth{}, model.Fail("check_busy", "Another health check is running; try again shortly")
	}
	defer s.healthMu.Unlock()
	src, st, err := s.semanticState(id)
	if err != nil {
		return model.SourceHealth{}, err
	}
	if !src.Enabled {
		return model.SourceHealth{}, model.Fail("source_disabled", "Enable the data source before checking it")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(min(src.Limits.TimeoutSeconds, 30))*time.Second)
	defer cancel()
	v := model.SourceHealth{SourceID: id, ConnectionRevision: src.ConnectionRevision, PublishedVersion: st.PublishedVersion, CheckedAt: time.Now().UTC(), Structure: []model.StructureHealth{}, Baseline: map[string]string{}, Current: map[string]string{}, BaselineColumns: map[string][]model.Column{}, CurrentColumns: map[string][]model.Column{}}
	previous, previousErr := s.Store.SourceHealth(id)
	if previousErr != nil && !errors.Is(previousErr, sql.ErrNoRows) {
		return v, previousErr
	}
	if previousErr == nil && previous.ConnectionRevision == src.ConnectionRevision {
		v.Baseline = previous.Baseline
		if previous.BaselineColumns != nil {
			v.BaselineColumns = previous.BaselineColumns
		}
	}
	refs := healthReferences(st.Published)
	v.CoverageLimited = len(refs) > 20
	watched := map[string]bool{}
	for _, ref := range refs[:min(len(refs), 20)] {
		watched[strconv.Quote(ref.Namespace)+":"+strconv.Quote(ref.Object)] = true
	}
	for key := range v.Baseline {
		if !watched[key] {
			delete(v.Baseline, key)
			delete(v.BaselineColumns, key)
		}
	}
	probe, probeErr := s.Engine.Probe(ctx, src)
	probe.CheckedAt = v.CheckedAt
	if probeErr != nil {
		probe = model.Probe{CheckedAt: v.CheckedAt, PermissionStatus: "unverified", Evidence: []string{}, Error: engine.PublicError(probeErr)}
	}
	v.Probe = probe
	if probe.Connected {
		for _, ref := range refs[:min(len(refs), 20)] {
			check := model.StructureHealth{Namespace: ref.Namespace, Object: ref.Object, Status: "unverified"}
			// These adapters inspect data contents for fields/types. Background checks
			// never sample documents, keys or nodes to infer a schema.
			switch src.Kind {
			case "neo4j", "redis", "valkey", "influxdb":
				v.Structure = append(v.Structure, check)
				continue
			}
			result, err := s.Engine.Execute(ctx, model.Principal{Admin: true, Preview: true}, "describe", model.Query{SourceID: id, Namespace: ref.Namespace, Object: ref.Object})
			if err != nil {
				check.Status = model.ErrorCode(err)
			} else if result.Truncated || result.NextCursor != "" {
				check.Status = "incomplete"
			} else {
				columns := []model.Column{}
				for _, row := range result.Data {
					raw, _ := json.Marshal(row)
					var object model.Object
					if json.Unmarshal(raw, &object) != nil {
						continue
					}
					if adapter.ForSource(src).Tool == "query_sql" || adapter.ForSource(src).Tool == "query_cql" {
						columns = append(columns, model.Column{Name: object.Name, Type: object.Type})
					} else {
						columns = append(columns, object.Columns...)
					}
				}
				if len(columns) > 0 {
					slices.SortFunc(columns, func(a, b model.Column) int {
						if a.Name < b.Name || a.Name == b.Name && a.Type < b.Type {
							return -1
						}
						if a == b {
							return 0
						}
						return 1
					})
					columns = slices.Compact(columns)
					raw, _ := json.Marshal(columns)
					digest := sha256.Sum256(raw)
					key := strconv.Quote(ref.Namespace) + ":" + strconv.Quote(ref.Object)
					v.Current[key] = hex.EncodeToString(digest[:])
					if len(columns) <= 200 && len(raw) <= 32<<10 {
						v.CurrentColumns[key] = columns
					} else {
						check.DetailLimited = true
					}
					check.Status = "unchanged"
					if baseline, known := v.Baseline[key]; !known {
						v.Baseline[key] = v.Current[key]
						v.BaselineColumns[key] = v.CurrentColumns[key]
						check.Status = "baseline"
					} else if baseline != v.Current[key] {
						check.Status = "changed"
						if old, known := v.BaselineColumns[key]; known && old != nil && v.CurrentColumns[key] != nil {
							check.Changes, check.DetailLimited = structureDiff(old, columns)
						} else {
							check.DetailLimited = true
						}
					}
				} else {
					check.Status = "unverified"
					if adapter.ForSource(src).Tool == "query_sql" || adapter.ForSource(src).Tool == "query_cql" {
						check.Status = "object_missing"
					}
				}
			}
			v.Structure = append(v.Structure, check)
		}
	}
	s.Store.Mutations.Lock()
	defer s.Store.Mutations.Unlock()
	fresh, latest, err := s.semanticState(id)
	if err != nil {
		return v, err
	}
	if fresh.ConnectionRevision != src.ConnectionRevision || fresh.ExecutionRevision() != src.ExecutionRevision() || latest.PublishedVersion != st.PublishedVersion {
		return v, model.Fail("conflict", "Source or publication changed during health check; run it again")
	}
	if probe.Connected && probe.ServerVersion != "" && fresh.ObservedVersion != probe.ServerVersion {
		if fresh.ObservedVersion != "" {
			fresh.ConnectionRevision++
			fresh.Revision++
			fresh.QueryRevision = fresh.Revision
			s.Engine.InvalidateSource(id)
		}
		fresh.ObservedVersion = probe.ServerVersion
	}
	fresh.Probe = &v.Probe
	if err = s.Store.SaveSource(fresh); err != nil {
		return v, err
	}
	v.ConnectionRevision = fresh.ConnectionRevision
	if err = s.Store.SaveSourceHealth(v); err != nil {
		return v, err
	}
	return v, nil
}

func (s *Server) runHealth(ctx context.Context) {
	defer close(s.healthDone)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		cfg, err := s.Store.HealthConfig()
		if err != nil || !cfg.Enabled {
			continue
		}
		sources, err := s.Store.Sources()
		if err != nil {
			continue
		}
		for _, src := range sources {
			if ctx.Err() != nil {
				return
			}
			latest, err := s.Store.HealthConfig()
			if err != nil || !latest.Enabled || latest.Revision != cfg.Revision {
				break
			}
			if !src.Enabled {
				continue
			}
			prior, err := s.Store.SourceHealth(src.ID)
			if err == nil && time.Since(prior.CheckedAt) < time.Duration(cfg.IntervalMinutes)*time.Minute {
				continue
			}
			s.checkSourceHealth(ctx, src.ID)
		}
	}
}

func structureDiff(before, after []model.Column) ([]model.StructureChange, bool) {
	old, next := map[string]string{}, map[string]string{}
	names := []string{}
	for _, col := range before {
		old[col.Name] = col.Type
		names = append(names, col.Name)
	}
	for _, col := range after {
		next[col.Name] = col.Type
		if _, exists := old[col.Name]; !exists {
			names = append(names, col.Name)
		}
	}
	slices.Sort(names)
	changes := []model.StructureChange{}
	limited := false
	for _, name := range names {
		a, was := old[name]
		b, is := next[name]
		if was && is && a == b {
			continue
		}
		if len(changes) == 10 || len(name) > 256 || len(a) > 128 || len(b) > 128 {
			limited = true
			continue
		}
		change := "type_changed"
		if !was {
			change = "added"
		}
		if !is {
			change = "removed"
		}
		changes = append(changes, model.StructureChange{Column: name, Change: change, Before: a, After: b})
	}
	return changes, limited
}
