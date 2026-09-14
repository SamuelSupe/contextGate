package store

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/SamuelSupe/contextGate/internal/model"
)

func (s *Store) EvaluationQuestion(source, id string) (v model.EvaluationQuestion, err error) {
	err = s.readEvaluation("evaluation_questions", source, id, &v)
	return
}

func (s *Store) Evaluation(source, id string) (v model.Evaluation, err error) {
	err = s.readEvaluation("evaluations", source, id, &v)
	return
}

func (s *Store) readEvaluation(table, source, id string, out any) error {
	var sealed string
	if err := s.DB.QueryRow("SELECT value FROM "+table+" WHERE source_id=$1 AND id=$2", source, id).Scan(&sealed); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Fail("not_found", "Saved question or evaluation not found")
		}
		return err
	}
	b, err := s.Vault.Open(sealed, table+":"+source+":"+id)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func (s *Store) SaveEvaluationQuestion(source string, v model.EvaluationQuestion, expected int64) error {
	return s.writeEvaluation("evaluation_questions", source, v.ID, expected, v, 1000)
}

func (s *Store) SaveEvaluation(v model.Evaluation, expected int64) error {
	return s.writeEvaluation("evaluations", v.SourceID, v.ID, expected, v, 10000)
}

// Table names are private constants supplied by the typed methods above.
// The revision compare-and-swap also protects edits in multiple admin tabs.
func (s *Store) writeEvaluation(table, source, id string, expected int64, v any, limit int) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	sealed := s.Vault.Seal(b, table+":"+source+":"+id)
	var res sql.Result
	if expected == 0 {
		res, err = s.DB.Exec("INSERT INTO "+table+"(source_id,id,revision,value) SELECT $1,$2,1,$3 WHERE (SELECT count(*) FROM "+table+" WHERE source_id=$4)<$5", source, id, sealed, source, limit)
	} else {
		res, err = s.DB.Exec("UPDATE "+table+" SET revision=$1,value=$2 WHERE source_id=$3 AND id=$4 AND revision=$5", expected+1, sealed, source, id, expected)
	}
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n != 1 {
		if expected == 0 {
			return model.Fail("limit_exceeded", "Saved question or history limit reached; export and delete old entries")
		}
		return model.Fail("conflict", "This record changed in another tab; reload before saving")
	}
	return err
}

func (s *Store) DeleteEvaluation(source, id string, questions bool, expected int64) error {
	table := "evaluations"
	if questions {
		table = "evaluation_questions"
	}
	res, err := s.DB.Exec("DELETE FROM "+table+" WHERE source_id=$1 AND id=$2 AND revision=$3", source, id, expected)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n != 1 {
		return model.Fail("conflict", "This record changed; reload before deleting")
	}
	return err
}
