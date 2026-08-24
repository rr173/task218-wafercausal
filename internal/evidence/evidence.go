// Package evidence 实现因果候选的证据裁决：确认根因、排除冲突来源，
// 并保证同一空间簇内的裁决相互一致（确认一个根因后其余候选自动排除）。
package evidence

import (
	"context"
	"fmt"

	"task218-wafercausal/internal/model"
	"task218-wafercausal/internal/store"
)

// Ruler 证据裁决器。
type Ruler struct {
	repos *store.Repositories
}

// NewRuler 构造裁决器。
func NewRuler(repos *store.Repositories) *Ruler {
	return &Ruler{repos: repos}
}

// Confirm 确认某候选为根因：候选转 confirmed_root，同簇其余候选转 excluded。
func (r *Ruler) Confirm(ctx context.Context, batchID, candidateID int64, actor, reason string) (*model.EvidenceRuling, error) {
	cand, err := r.repos.Causals.GetCandidate(ctx, candidateID)
	if err != nil {
		return nil, err
	}
	if cand.BatchID != batchID {
		return nil, fmt.Errorf("%w: candidate not in batch", model.ErrInvalid)
	}
	if cand.Status != model.CausalStatusCandidate {
		return nil, fmt.Errorf("%w: candidate already %s", model.ErrStateMachine, cand.Status)
	}
	if err := r.repos.Causals.UpdateCandidateStatus(ctx, candidateID, model.CausalStatusConfirmedRoot); err != nil {
		return nil, err
	}
	// 同簇其余候选排除，保留冲突来源。
	siblings, err := r.repos.Causals.ListCandidatesByCluster(ctx, cand.ClusterID)
	if err != nil {
		return nil, err
	}
	for _, sib := range siblings {
		if sib.ID == candidateID || sib.Status != model.CausalStatusCandidate {
			continue
		}
		if err := r.repos.Causals.UpdateCandidateStatus(ctx, sib.ID, model.CausalStatusExcluded); err != nil {
			return nil, err
		}
	}
	rule := &model.EvidenceRuling{
		BatchID:     batchID,
		CandidateID: candidateID,
		Decision:    "confirm",
		Reason:      reason,
		Actor:       actor,
		CreatedAt:   model.NowStr(),
	}
	return r.saveRuling(ctx, rule)
}

// Exclude 排除某候选（冲突来源显式保留）。
func (r *Ruler) Exclude(ctx context.Context, batchID, candidateID int64, actor, reason string) (*model.EvidenceRuling, error) {
	cand, err := r.repos.Causals.GetCandidate(ctx, candidateID)
	if err != nil {
		return nil, err
	}
	if cand.BatchID != batchID {
		return nil, fmt.Errorf("%w: candidate not in batch", model.ErrInvalid)
	}
	if cand.Status != model.CausalStatusCandidate {
		return nil, fmt.Errorf("%w: candidate already %s", model.ErrStateMachine, cand.Status)
	}
	if err := r.repos.Causals.UpdateCandidateStatus(ctx, candidateID, model.CausalStatusExcluded); err != nil {
		return nil, err
	}
	rule := &model.EvidenceRuling{
		BatchID:     batchID,
		CandidateID: candidateID,
		Decision:    "exclude",
		Reason:      reason,
		Actor:       actor,
		CreatedAt:   model.NowStr(),
	}
	return r.saveRuling(ctx, rule)
}

// saveRuling 落盘裁决并写审计。
func (r *Ruler) saveRuling(ctx context.Context, rule *model.EvidenceRuling) (*model.EvidenceRuling, error) {
	saved, err := r.repos.Rulings.CreateRuling(ctx, rule)
	if err != nil {
		return nil, err
	}
	_ = r.repos.AuditEvent(ctx, rule.Actor, "ruling."+rule.Decision,
		fmt.Sprintf("candidate:%d", rule.CandidateID), rule.Reason)
	return saved, nil
}
