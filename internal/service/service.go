// Package service 编排芯片晶圆缺陷因果链追踪服务的完整业务闭环。
package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"task218-wafercausal/internal/causal"
	"task218-wafercausal/internal/cluster"
	"task218-wafercausal/internal/evidence"
	"task218-wafercausal/internal/ingest"
	"task218-wafercausal/internal/model"
	"task218-wafercausal/internal/store"
)

// Service 聚合全部业务能力。
type Service struct {
	repos *store.Repositories
	mu    sync.Mutex
}

// New 构造服务。
func New(repos *store.Repositories) *Service {
	return &Service{repos: repos}
}

// Repos 暴露仓储（供自检与扩展使用）。
func (s *Service) Repos() *store.Repositories { return s.repos }

// ---------------------------------------------------------------------------
// 晶圆批次
// ---------------------------------------------------------------------------

// CreateBatch 创建晶圆批次（初始状态 producing）。
func (s *Service) CreateBatch(ctx context.Context, name, waferID string, diameterMM float64) (*model.WaferBatch, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: batch name is required", model.ErrInvalid)
	}
	if strings.TrimSpace(waferID) == "" {
		return nil, fmt.Errorf("%w: wafer_id is required", model.ErrInvalid)
	}
	if diameterMM <= 0 {
		diameterMM = 300
	}
	now := model.NowStr()
	b, err := s.repos.Batches.CreateBatch(ctx, &model.WaferBatch{
		Name:       name,
		WaferID:    waferID,
		DiameterMM: diameterMM,
		Status:     model.BatchProducing,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		return nil, err
	}
	_ = s.repos.AuditEvent(ctx, "system", "batch.create", fmt.Sprintf("batch:%d", b.ID), waferID)
	return b, nil
}

// GetBatch 查询晶圆批次。
func (s *Service) GetBatch(ctx context.Context, id int64) (*model.WaferBatch, error) {
	return s.repos.Batches.GetBatch(ctx, id)
}

// ListBatches 列出全部晶圆批次。
func (s *Service) ListBatches(ctx context.Context) ([]*model.WaferBatch, error) {
	return s.repos.Batches.ListBatches(ctx)
}

// FreezeBatch 冻结批次进入待分析状态（数据冻结，准备因果分析）。
func (s *Service) FreezeBatch(ctx context.Context, id int64) (*model.WaferBatch, error) {
	b, err := s.repos.Batches.GetBatch(ctx, id)
	if err != nil {
		return nil, err
	}
	if b.Status != model.BatchProducing {
		return nil, fmt.Errorf("%w: batch is %s, cannot freeze", model.ErrStateMachine, b.Status)
	}
	if err := s.repos.Batches.UpdateStatus(ctx, id, model.BatchPending, model.NowStr()); err != nil {
		return nil, err
	}
	b.Status = model.BatchPending
	_ = s.repos.AuditEvent(ctx, "system", "batch.freeze", fmt.Sprintf("batch:%d", id), "")
	return b, nil
}

// ArchiveBatch 封存批次（只读，后续仅能建立替代版本）。
func (s *Service) ArchiveBatch(ctx context.Context, id int64) (*model.WaferBatch, error) {
	b, err := s.repos.Batches.GetBatch(ctx, id)
	if err != nil {
		return nil, err
	}
	if b.Status == model.BatchArchived {
		return nil, fmt.Errorf("%w: batch already archived", model.ErrStateMachine)
	}
	now := model.NowStr()
	if err := s.repos.Batches.Archive(ctx, id, now, now); err != nil {
		return nil, err
	}
	b.Status = model.BatchArchived
	b.ArchivedAt = now
	_ = s.repos.AuditEvent(ctx, "system", "batch.archive", fmt.Sprintf("batch:%d", id), "")
	return b, nil
}

// ---------------------------------------------------------------------------
// 工艺步骤
// ---------------------------------------------------------------------------

// AddStep 新增单个工艺步骤。
func (s *Service) AddStep(ctx context.Context, batchID int64, name, tool string, seq int) (*model.ProcessStep, error) {
	if err := s.ensureWritable(ctx, batchID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("%w: step name is required", model.ErrInvalid)
	}
	st, err := s.repos.Steps.CreateStep(ctx, &model.ProcessStep{
		BatchID:   batchID,
		Name:      name,
		Sequence:  seq,
		Tool:      tool,
		CreatedAt: model.NowStr(),
	})
	if err != nil {
		return nil, err
	}
	_ = s.repos.AuditEvent(ctx, "system", "step.create", fmt.Sprintf("step:%d", st.ID), name)
	return st, nil
}

// AddStepChain 新增一条顺序工艺链：按名字顺序建立 seq 与前置依赖。
func (s *Service) AddStepChain(ctx context.Context, batchID int64, names []string) ([]*model.ProcessStep, error) {
	if err := s.ensureWritable(ctx, batchID); err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("%w: empty step chain", model.ErrInvalid)
	}
	var out []*model.ProcessStep
	var parentID int64
	for i, name := range names {
		st, err := s.repos.Steps.CreateStep(ctx, &model.ProcessStep{
			BatchID:      batchID,
			Name:         strings.TrimSpace(name),
			Sequence:     i + 1,
			ParentStepID: parentID,
			CreatedAt:    model.NowStr(),
		})
		if err != nil {
			return nil, err
		}
		parentID = st.ID
		out = append(out, st)
	}
	_ = s.repos.AuditEvent(ctx, "system", "step.chain", fmt.Sprintf("batch:%d", batchID),
		fmt.Sprintf("steps=%d", len(out)))
	return out, nil
}

// ListSteps 列出某批次的工艺步骤。
func (s *Service) ListSteps(ctx context.Context, batchID int64) ([]*model.ProcessStep, error) {
	return s.repos.Steps.ListStepsByBatch(ctx, batchID)
}

// SetStepParent 设置工艺步骤的前置依赖。
func (s *Service) SetStepParent(ctx context.Context, stepID, parentStepID int64) error {
	st, err := s.repos.Steps.GetStep(ctx, stepID)
	if err != nil {
		return err
	}
	if err := s.ensureWritable(ctx, st.BatchID); err != nil {
		return err
	}
	if parentStepID == stepID {
		return fmt.Errorf("%w: self-dependency", model.ErrInvalid)
	}
	return s.repos.Steps.SetParent(ctx, stepID, parentStepID)
}

// ---------------------------------------------------------------------------
// 缺陷接收
// ---------------------------------------------------------------------------

// ImportDefects 批量接收缺陷记录（幂等去重）。
func (s *Service) ImportDefects(ctx context.Context, batchID int64, inputs []ingest.DefectInput) (*ingest.ImportResult, error) {
	b, err := s.repos.Batches.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(b); err != nil {
		return nil, err
	}
	im := ingest.NewImporter(s.repos)
	res, err := im.ImportDefects(ctx, b, inputs)
	if err != nil {
		return nil, err
	}
	_ = s.repos.AuditEvent(ctx, "system", "defect.import", fmt.Sprintf("batch:%d", batchID),
		fmt.Sprintf("accepted=%d skipped=%d", res.Accepted, res.Skipped))
	return res, nil
}

// ListDefects 列出某批次的缺陷记录。
func (s *Service) ListDefects(ctx context.Context, batchID int64) ([]*model.DefectRecord, error) {
	return s.repos.Defects.ListDefectsByBatch(ctx, batchID)
}

// ---------------------------------------------------------------------------
// 工艺事件
// ---------------------------------------------------------------------------

// ImportEvents 批量接收工艺事件。
func (s *Service) ImportEvents(ctx context.Context, batchID int64, inputs []ingest.EventInput) (*ingest.ImportResult, error) {
	b, err := s.repos.Batches.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(b); err != nil {
		return nil, err
	}
	im := ingest.NewImporter(s.repos)
	res, err := im.ImportEvents(ctx, b, inputs)
	if err != nil {
		return nil, err
	}
	_ = s.repos.AuditEvent(ctx, "system", "event.import", fmt.Sprintf("batch:%d", batchID),
		fmt.Sprintf("accepted=%d", res.Accepted))
	return res, nil
}

// ListEvents 列出某批次的工艺事件。
func (s *Service) ListEvents(ctx context.Context, batchID int64) ([]*model.ProcessEvent, error) {
	return s.repos.Events.ListEventsByBatch(ctx, batchID)
}

// MarkEventUntrusted 标记工艺事件为设备数据不可信（或解除）。
func (s *Service) MarkEventUntrusted(ctx context.Context, eventID int64, untrusted bool) error {
	e, err := s.repos.Events.GetEvent(ctx, eventID)
	if err != nil {
		return err
	}
	b, err := s.repos.Batches.GetBatch(ctx, e.BatchID)
	if err != nil {
		return err
	}
	if err := ensureWritable(b); err != nil {
		return err
	}
	if err := s.repos.Events.MarkUntrusted(ctx, eventID, untrusted); err != nil {
		return err
	}
	_ = s.repos.AuditEvent(ctx, "system", "event.untrusted", fmt.Sprintf("event:%d", eventID),
		fmt.Sprintf("%t", untrusted))
	return nil
}

// ---------------------------------------------------------------------------
// 空间聚类
// ---------------------------------------------------------------------------

// RunClustering 执行空间聚类：按距离聚合缺陷，重建空间簇。
func (s *Service) RunClustering(ctx context.Context, batchID int64, thresholdUM float64) ([]*model.SpatialCluster, error) {
	b, err := s.repos.Batches.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(b); err != nil {
		return nil, err
	}
	if thresholdUM <= 0 {
		thresholdUM = 5000 // 默认 5mm
	}
	defects, err := s.repos.Defects.ListDefectsByBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if len(defects) == 0 {
		return nil, fmt.Errorf("%w: no defects to cluster", model.ErrInvalid)
	}
	results := cluster.Cluster(defects, thresholdUM)
	// 重建：清空旧簇后写入新簇。
	if _, err := s.repos.Clusters.DeleteClustersByBatch(ctx, batchID); err != nil {
		return nil, err
	}
	now := model.NowStr()
	var out []*model.SpatialCluster
	for _, r := range results {
		r.Cluster.BatchID = batchID
		r.Cluster.CreatedAt = now
		created, err := s.repos.Clusters.CreateCluster(ctx, r.Cluster)
		if err != nil {
			return nil, err
		}
		members := r.DefectIDs
		for _, did := range members {
			if err := s.repos.Clusters.AddClusterDefect(ctx, created.ID, did); err != nil {
				return nil, err
			}
			_ = s.repos.Defects.UpdateStatus(ctx, did, model.DefectClustered)
		}
		out = append(out, created)
	}
	_ = s.repos.AuditEvent(ctx, "system", "cluster.run", fmt.Sprintf("batch:%d", batchID),
		fmt.Sprintf("clusters=%d", len(out)))
	return out, nil
}

// ListClusters 列出某批次的空间簇。
func (s *Service) ListClusters(ctx context.Context, batchID int64) ([]*model.SpatialCluster, error) {
	return s.repos.Clusters.ListClustersByBatch(ctx, batchID)
}

// ---------------------------------------------------------------------------
// 因果候选
// ---------------------------------------------------------------------------

// GenerateCandidates 为全部空间簇生成因果候选。
func (s *Service) GenerateCandidates(ctx context.Context, batchID int64) ([]*model.CausalCandidate, error) {
	b, err := s.repos.Batches.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if err := ensureWritable(b); err != nil {
		return nil, err
	}
	gen := causal.NewGenerator(s.repos)
	specs, err := gen.Generate(ctx, batchID)
	if err != nil {
		return nil, err
	}
	now := model.NowStr()
	var out []*model.CausalCandidate
	for _, sp := range specs {
		cand, err := s.repos.Causals.CreateCandidate(ctx, &model.CausalCandidate{
			BatchID:    batchID,
			ClusterID:  sp.ClusterID,
			RootStepID: sp.RootStepID,
			Score:      sp.Score,
			Evidence:   sp.Evidence,
			Status:     model.CausalStatusCandidate,
			CreatedAt:  now,
		})
		if err != nil {
			return nil, err
		}
		for order, stepID := range sp.StepIDs {
			if err := s.repos.Causals.AddCandidateStep(ctx, cand.ID, stepID, order); err != nil {
				return nil, err
			}
		}
		_ = s.repos.Clusters.UpdateStatus(ctx, sp.ClusterID, model.ClusterConfirmed)
		out = append(out, cand)
	}
	_ = s.repos.AuditEvent(ctx, "system", "causal.generate", fmt.Sprintf("batch:%d", batchID),
		fmt.Sprintf("candidates=%d", len(out)))
	return out, nil
}

// ListCandidates 列出某批次的因果候选。
func (s *Service) ListCandidates(ctx context.Context, batchID int64) ([]*model.CausalCandidate, error) {
	return s.repos.Causals.ListCandidatesByBatch(ctx, batchID)
}

// GetCandidate 查询单个因果候选。
func (s *Service) GetCandidate(ctx context.Context, id int64) (*model.CausalCandidate, error) {
	return s.repos.Causals.GetCandidate(ctx, id)
}

// ---------------------------------------------------------------------------
// 证据裁决
// ---------------------------------------------------------------------------

// ConfirmCandidate 确认根因候选。
func (s *Service) ConfirmCandidate(ctx context.Context, batchID, candidateID int64, actor, reason string) (*model.EvidenceRuling, error) {
	ruler := evidence.NewRuler(s.repos)
	return ruler.Confirm(ctx, batchID, candidateID, actor, reason)
}

// ExcludeCandidate 排除冲突候选。
func (s *Service) ExcludeCandidate(ctx context.Context, batchID, candidateID int64, actor, reason string) (*model.EvidenceRuling, error) {
	ruler := evidence.NewRuler(s.repos)
	return ruler.Exclude(ctx, batchID, candidateID, actor, reason)
}

// ListRulings 列出某批次的证据裁决。
func (s *Service) ListRulings(ctx context.Context, batchID int64) ([]*model.EvidenceRuling, error) {
	return s.repos.Rulings.ListRulingsByBatch(ctx, batchID)
}

// ---------------------------------------------------------------------------
// 因果快照
// ---------------------------------------------------------------------------

// PublishSnapshot 发布因果分析快照（版本化、不可变）。
func (s *Service) PublishSnapshot(ctx context.Context, batchID int64, name string) (*model.CausalSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := s.repos.Batches.GetBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if b.Status == model.BatchArchived {
		return nil, fmt.Errorf("%w: cannot snapshot archived batch", model.ErrArchived)
	}
	cands, err := s.repos.Causals.ListCandidatesByBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	roots := make([]string, 0)
	confirmed := make([]*model.CausalCandidate, 0)
	for _, c := range cands {
		if c.Status == model.CausalStatusConfirmedRoot {
			roots = append(roots, fmt.Sprintf("step:%d", c.RootStepID))
			confirmed = append(confirmed, c)
		}
	}
	summary := fmt.Sprintf("confirmed_roots=%d", len(confirmed))
	if len(roots) > 0 {
		summary += " [" + strings.Join(roots, ",") + "]"
	}
	if strings.TrimSpace(name) == "" {
		name = fmt.Sprintf("snapshot-%d", len(cands))
	}
	ver, err := s.repos.Snapshots.NextVersion(ctx, batchID)
	if err != nil {
		return nil, err
	}
	now := model.NowStr()
	snap, err := s.repos.Snapshots.CreateSnapshot(ctx, &model.CausalSnapshot{
		BatchID:     batchID,
		Version:     ver,
		Name:        name,
		Status:      model.SnapshotPublished,
		RootSummary: summary,
		CreatedAt:   now,
	})
	if err != nil {
		return nil, err
	}
	for _, c := range cands {
		if err := s.repos.Snapshots.AddSnapshotCandidate(ctx, snap.ID, c.ID, c.Status); err != nil {
			return nil, err
		}
	}
	// 早于当前版本的已发布快照标记为替代。
	_, _ = s.repos.Snapshots.Supersede(ctx, batchID, ver, now)
	_ = s.repos.AuditEvent(ctx, "system", "snapshot.publish", fmt.Sprintf("batch:%d", batchID),
		fmt.Sprintf("version=%d", ver))
	return snap, nil
}

// GetSnapshot 查询因果快照。
func (s *Service) GetSnapshot(ctx context.Context, id int64) (*model.CausalSnapshot, error) {
	return s.repos.Snapshots.GetSnapshot(ctx, id)
}

// ListSnapshots 列出某批次的因果快照。
func (s *Service) ListSnapshots(ctx context.Context, batchID int64) ([]*model.CausalSnapshot, error) {
	return s.repos.Snapshots.ListSnapshotsByBatch(ctx, batchID)
}

// ---------------------------------------------------------------------------
// 自检
// ---------------------------------------------------------------------------

// SelfCheck 返回数据库统计与完整性检查结果。
func (s *Service) SelfCheck(ctx context.Context) (map[string]int64, error) {
	return s.repos.Stats(ctx)
}

// ensureWritable 校验批次未被封存，允许写入。
func (s *Service) ensureWritable(ctx context.Context, batchID int64) error {
	b, err := s.repos.Batches.GetBatch(ctx, batchID)
	if err != nil {
		return err
	}
	return ensureWritable(b)
}

// ensureWritable 校验批次对象未被封存。
func ensureWritable(b *model.WaferBatch) error {
	if b.Status == model.BatchArchived {
		return model.ErrArchived
	}
	return nil
}
