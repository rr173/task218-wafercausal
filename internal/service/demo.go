package service

import (
	"context"
	"fmt"
	"time"

	"task218-wafercausal/internal/ingest"
	"task218-wafercausal/internal/model"
)

// RunDemo 执行确定性端到端演示：从建批次到发布因果快照的完整业务闭环。
// 任何一步断言失败都会返回错误，供 --smoke-test 判据使用。
func (s *Service) RunDemo(ctx context.Context) error {
	// 固定时间基准，保证每次运行结果确定。
	base := time.Date(2026, 8, 23, 8, 0, 0, 0, time.UTC)
	at := func(d time.Duration) string { return base.Add(d).Format(time.RFC3339) }

	// 1. 创建晶圆批次。
	batch, err := s.CreateBatch(ctx, "demo-wafer", "WFR-0001", 300)
	if err != nil {
		return fmt.Errorf("create batch: %w", err)
	}

	// 2. 建立工艺步骤链：光刻 → 蚀刻 → 沉积 → 清洗。
	steps, err := s.AddStepChain(ctx, batch.ID, []string{"litho", "etch", "deposition", "clean"})
	if err != nil {
		return fmt.Errorf("add step chain: %w", err)
	}
	if len(steps) != 4 {
		return fmt.Errorf("expected 4 steps, got %d", len(steps))
	}
	lithoID, etchID, depID := steps[0].ID, steps[1].ID, steps[2].ID

	// 3. 导入工艺事件（蚀刻设备出现一条不可信记录）。
	events, err := s.ImportEvents(ctx, batch.ID, []ingest.EventInput{
		{StepID: lithoID, Tool: "scanner-1", Param: `{"dose": 42}`, OccurredAt: at(0)},
		{StepID: etchID, Tool: "etcher-1", Param: `{"depth": 120}`, OccurredAt: at(10 * time.Minute)},
		{StepID: etchID, Tool: "etcher-1", Param: `{"depth": 999}`, OccurredAt: at(12 * time.Minute)},
		{StepID: depID, Tool: "cvd-1", Param: `{"film": "oxide"}`, OccurredAt: at(20 * time.Minute)},
	})
	if err != nil {
		return fmt.Errorf("import events: %w", err)
	}
	if events.Accepted != 4 {
		return fmt.Errorf("expected 4 events accepted, got %d", events.Accepted)
	}
	// 标记第二条蚀刻事件为不可信。
	if err := s.MarkEventUntrusted(ctx, events.IDs[2], true); err != nil {
		return fmt.Errorf("mark untrusted: %w", err)
	}

	// 4. 导入缺陷记录（同一位置连续缺陷 + 一条重复项验证幂等）。
	defInputs := []ingest.DefectInput{
		{DetectBatch: "inspect-1", X: 10, Y: 10, RadiusUM: 2, Severity: "major", DetectedAt: at(30 * time.Minute)},
		{DetectBatch: "inspect-1", X: 10.1, Y: 10.2, RadiusUM: 3, Severity: "critical", DetectedAt: at(31 * time.Minute)},
		{DetectBatch: "inspect-1", X: 10.2, Y: 9.9, RadiusUM: 2, Severity: "major", DetectedAt: at(32 * time.Minute)},
		{DetectBatch: "inspect-1", X: -80, Y: -80, RadiusUM: 1, Severity: "minor", DetectedAt: at(33 * time.Minute)},
		// 与第一条完全相同的记录 → 应被幂等跳过。
		{DetectBatch: "inspect-1", X: 10, Y: 10, RadiusUM: 2, Severity: "major", DetectedAt: at(30 * time.Minute)},
	}
	defRes, err := s.ImportDefects(ctx, batch.ID, defInputs)
	if err != nil {
		return fmt.Errorf("import defects: %w", err)
	}
	if defRes.Accepted != 4 || defRes.Skipped != 1 {
		return fmt.Errorf("expected 4 accepted + 1 skipped, got %d/%d", defRes.Accepted, defRes.Skipped)
	}

	// 5. 冻结批次并执行空间聚类。
	if _, err := s.FreezeBatch(ctx, batch.ID); err != nil {
		return fmt.Errorf("freeze batch: %w", err)
	}
	clusters, err := s.RunClustering(ctx, batch.ID, 5000)
	if err != nil {
		return fmt.Errorf("run clustering: %w", err)
	}
	if len(clusters) < 2 {
		return fmt.Errorf("expected >=2 clusters, got %d", len(clusters))
	}

	// 6. 生成因果候选。
	cands, err := s.GenerateCandidates(ctx, batch.ID)
	if err != nil {
		return fmt.Errorf("generate candidates: %w", err)
	}
	if len(cands) == 0 {
		return fmt.Errorf("expected candidates, got 0")
	}

	// 7. 证据裁决：每个空间簇确认一个根因（确认会自动排除同簇其余候选）。
	confirmedClusters := make(map[int64]bool)
	for _, c := range cands {
		if c.Status != model.CausalStatusCandidate {
			continue
		}
		if confirmedClusters[c.ClusterID] {
			continue
		}
		if _, err := s.ConfirmCandidate(ctx, batch.ID, c.ID, "engineer", "matches depth signature"); err != nil {
			return fmt.Errorf("confirm candidate: %w", err)
		}
		confirmedClusters[c.ClusterID] = true
	}
	if len(confirmedClusters) == 0 {
		return fmt.Errorf("no candidate confirmed")
	}

	// 8. 发布因果快照。
	snap, err := s.PublishSnapshot(ctx, batch.ID, "demo-snapshot")
	if err != nil {
		return fmt.Errorf("publish snapshot: %w", err)
	}
	if snap.Version != 1 {
		return fmt.Errorf("expected snapshot version 1, got %d", snap.Version)
	}

	// 9. 自检统计。
	stats, err := s.SelfCheck(ctx)
	if err != nil {
		return fmt.Errorf("self check: %w", err)
	}
	if stats["integrity_check"] != 1 {
		return fmt.Errorf("integrity check failed: %+v", stats)
	}
	return nil
}
