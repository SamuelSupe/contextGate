package engine

import (
	"context"
	"encoding/json"

	"github.com/SamuelSupe/contextGate/internal/adapter"
	"github.com/SamuelSupe/contextGate/internal/model"
	"github.com/SamuelSupe/contextGate/internal/semantic"
)

func (e *Engine) DraftTemplateValidation(src model.Source, st semantic.State, en semantic.Entry) TemplateValidation {
	v := e.TemplateValidation(src, en)
	if en.Template == nil || !en.Template.Enabled {
		return v
	}
	if st.TrialAfter != nil && (v.CheckedAt == nil || v.CheckedAt.Before(*st.TrialAfter)) {
		v.Valid, v.Status = false, "trial_required"
	}
	ev, err := e.Store.SemanticEvidence(src.ID, "trial-result:"+semantic.EvidenceDefinition(*en.Template))
	if err == nil && ev.Connection == e.connectionProof(src) {
		var report semantic.TrialReport
		if json.Unmarshal([]byte(ev.DetailsJSON), &report) == nil {
			v.Report = &report
			if !report.Passed && (v.CheckedAt == nil || !ev.CheckedAt.Before(*v.CheckedAt)) {
				v.Valid, v.Status = false, "regression_failed"
			}
		}
	}
	return v
}

func (e *Engine) runTemplateCases(ctx context.Context, src model.Source, en semantic.Entry) (semantic.TrialReport, error) {
	report := semantic.TrialReport{Passed: true, Cases: []semantic.CaseResult{}}
	cases := append([]semantic.RegressionCase{{Name: "Example", ParametersJSON: en.Template.ExampleJSON}}, en.Template.Tests...)
	var firstError error
	for i, c := range cases {
		check := semantic.CaseResult{Name: c.Name}
		query, err := semantic.BindCase(*en.Template, c)
		if err == nil {
			err = adapter.CheckTemplateTargets(src, query)
		}
		if err == nil {
			query.SourceID = src.ID
			var result *model.Result
			result, err = e.execute(ctx, model.AdministratorPrincipal(ctx), en.Template.Tool, query, nil, en.ID)
			if err == nil {
				check.Rows, check.ElapsedMS = result.RowCount, result.ElapsedMS
				if i > 0 {
					check.ErrorCode = semantic.CheckRegression(c, result)
				}
			}
		}
		if err != nil {
			check.ErrorCode = model.ErrorCode(err)
			if firstError == nil {
				firstError = err
			}
		}
		check.Passed = check.ErrorCode == ""
		if !check.Passed {
			report.Passed = false
		}
		report.Cases = append(report.Cases, check)
		if ctx.Err() != nil {
			break
		}
	}
	if !report.Passed && firstError == nil {
		firstError = model.Fail("regression_failed", "Template regression checks failed; review the saved trial report")
	}
	return report, firstError
}
