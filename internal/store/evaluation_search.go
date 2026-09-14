package store

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SamuelSupe/contextGate/internal/model"
)

type EvaluationFilter struct {
	Search, Agent string
	From, Until   time.Time
}

type EvaluationSummary struct {
	Total           int `json:"total"`
	CompletedPairs  int `json:"completed_pairs"`
	ReviewedPairs   int `json:"reviewed_pairs"`
	ChangedPairs    int `json:"changed_pairs"`
	BaselineCorrect int `json:"baseline_correct"`
	GuidedCorrect   int `json:"guided_correct"`
}

type EvaluationAgent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type EvaluationPage struct {
	Items   []json.RawMessage `json:"items"`
	Next    string            `json:"next_cursor"`
	Summary EvaluationSummary `json:"summary"`
	Agents  []EvaluationAgent `json:"agents"`
}

// Search decrypted records one at a time, without adding plaintext indexes of
// business questions. Per-source retention caps and the caller's deadline bound
// this scan; summaries cover all matches, independently of the displayed page.
func (s *Store) SearchEvaluations(ctx context.Context, source string, questions bool, before int64, filter EvaluationFilter) (EvaluationPage, error) {
	page := EvaluationPage{Items: []json.RawMessage{}, Agents: []EvaluationAgent{}}
	table := "evaluations"
	if questions {
		table = "evaluation_questions"
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT seq,id,value FROM "+table+" WHERE source_id=$1 ORDER BY seq DESC", source)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	agents := map[string]string{}
	var last int64
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return page, err
		}
		var seq int64
		var id, sealed string
		if err := rows.Scan(&seq, &id, &sealed); err != nil {
			return page, err
		}
		b, err := s.Vault.Open(sealed, table+":"+source+":"+id)
		if err != nil {
			return page, err
		}
		var v model.Evaluation
		if err := json.Unmarshal(b, &v); err != nil {
			return page, err
		}
		if v.AgentID != "" {
			if _, exists := agents[v.AgentID]; !exists {
				agents[v.AgentID] = v.AgentName
			}
		}
		if filter.Agent != "" && v.AgentID != filter.Agent ||
			!filter.From.IsZero() && v.Created.Before(filter.From) ||
			!filter.Until.IsZero() && !v.Created.Before(filter.Until) ||
			!strings.Contains(strings.ToLower(v.Name+"\n"+v.Question), strings.ToLower(filter.Search)) {
			continue
		}
		page.Summary.Total++
		baseline, guided := v.Runs["baseline"], v.Runs["guided"]
		if baseline != nil && guided != nil && baseline.State == "completed" && guided.State == "completed" {
			page.Summary.CompletedPairs++
			if baseline.ConfigurationChanged || guided.ConfigurationChanged || baseline.Configuration != guided.Configuration {
				page.Summary.ChangedPairs++
			} else if baseline.Stats != nil && guided.Stats != nil && baseline.Stats.Queries > 0 && guided.Stats.Queries > 0 && reviewedVerdict(baseline.Verdict) && reviewedVerdict(guided.Verdict) {
				page.Summary.ReviewedPairs++
				if baseline.Verdict == "correct" {
					page.Summary.BaselineCorrect++
				}
				if guided.Verdict == "correct" {
					page.Summary.GuidedCorrect++
				}
			}
		}
		if before != 0 && seq >= before {
			continue
		}
		if len(page.Items) < 20 {
			page.Items = append(page.Items, b)
			last = seq
		} else if page.Next == "" {
			page.Next = strconv.FormatInt(last, 10)
		}
	}
	for id, name := range agents {
		page.Agents = append(page.Agents, EvaluationAgent{id, name})
	}
	sort.Slice(page.Agents, func(i, j int) bool {
		if page.Agents[i].Name == page.Agents[j].Name {
			return page.Agents[i].ID < page.Agents[j].ID
		}
		return page.Agents[i].Name < page.Agents[j].Name
	})
	return page, rows.Err()
}

func reviewedVerdict(v string) bool { return v == "correct" || v == "partial" || v == "incorrect" }
