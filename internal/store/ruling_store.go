package store

import (
	"context"
	"database/sql"

	"task218-wafercausal/internal/model"
)

// RulingStore 负责证据裁决的持久化。
type RulingStore struct {
	db *sql.DB
}

// CreateRuling 插入证据裁决。
func (s *RulingStore) CreateRuling(ctx context.Context, r *model.EvidenceRuling) (*model.EvidenceRuling, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO evidence_rulings (batch_id, candidate_id, decision, reason, actor, created_at)
		 VALUES (?, ?, ?, ?, ?, ?);`,
		r.BatchID, r.CandidateID, r.Decision, r.Reason, r.Actor, r.CreatedAt)
	if err != nil {
		return nil, err
	}
	r.ID, err = res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r, nil
}

// ListRulingsByBatch 列出某批次全部证据裁决（倒序）。
func (s *RulingStore) ListRulingsByBatch(ctx context.Context, batchID int64) ([]*model.EvidenceRuling, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, batch_id, candidate_id, decision, reason, actor, created_at
		 FROM evidence_rulings WHERE batch_id = ? ORDER BY id DESC;`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.EvidenceRuling, 0)
	for rows.Next() {
		var r model.EvidenceRuling
		if err := rows.Scan(&r.ID, &r.BatchID, &r.CandidateID, &r.Decision,
			&r.Reason, &r.Actor, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &r)
	}
	return out, rows.Err()
}

// ListRulingsByCandidate 列出某候选对应的裁决。
func (s *RulingStore) ListRulingsByCandidate(ctx context.Context, candidateID int64) ([]*model.EvidenceRuling, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, batch_id, candidate_id, decision, reason, actor, created_at
		 FROM evidence_rulings WHERE candidate_id = ? ORDER BY id ASC;`, candidateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.EvidenceRuling, 0)
	for rows.Next() {
		var r model.EvidenceRuling
		if err := rows.Scan(&r.ID, &r.BatchID, &r.CandidateID, &r.Decision,
			&r.Reason, &r.Actor, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &r)
	}
	return out, rows.Err()
}
