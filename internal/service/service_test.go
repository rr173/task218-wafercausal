package service

import (
	"context"
	"errors"
	"testing"

	"task218-wafercausal/internal/ingest"
	"task218-wafercausal/internal/model"
	"task218-wafercausal/internal/store"
)

func newTestService(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	st, err := store.Open("")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return New(store.NewRepositories(st)), st
}

func TestRunDemo(t *testing.T) {
	ctx := context.Background()
	app, st := newTestService(t)
	defer st.Close()
	if err := app.RunDemo(ctx); err != nil {
		t.Fatalf("RunDemo: %v", err)
	}
}

func TestRestartRecovery(t *testing.T) {
	ctx := context.Background()
	app, st := newTestService(t)
	if err := app.RunDemo(ctx); err != nil {
		t.Fatalf("RunDemo: %v", err)
	}
	path := st.Path()
	if err := st.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// 重开同一数据库验证持久化。
	st2, err := store.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st2.Close()
	app2 := New(store.NewRepositories(st2))
	batches, err := app2.ListBatches(ctx)
	if err != nil || len(batches) == 0 {
		t.Fatalf("list batches after reopen: %d %v", len(batches), err)
	}
	snaps, err := app2.ListSnapshots(ctx, batches[0].ID)
	if err != nil || len(snaps) == 0 {
		t.Fatalf("list snapshots after reopen: %d %v", len(snaps), err)
	}
	if snaps[0].Version != 1 {
		t.Fatalf("expected snapshot version 1, got %d", snaps[0].Version)
	}
}

func TestArchiveRejectsWrites(t *testing.T) {
	ctx := context.Background()
	app, st := newTestService(t)
	defer st.Close()

	batch, err := app.CreateBatch(ctx, "b", "WFR-A", 300)
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	if _, err := app.ArchiveBatch(ctx, batch.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if _, err := app.ImportDefects(ctx, batch.ID, []ingest.DefectInput{
		{DetectBatch: "i1", X: 1, Y: 1, DetectedAt: "2026-08-23T08:00:00Z"},
	}); err != model.ErrArchived {
		t.Fatalf("expected ErrArchived after archive, got %v", err)
	}
}

func TestFreezeStateMachine(t *testing.T) {
	ctx := context.Background()
	app, st := newTestService(t)
	defer st.Close()

	batch, err := app.CreateBatch(ctx, "b", "WFR-B", 300)
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	if _, err := app.FreezeBatch(ctx, batch.ID); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	// 已冻结不能再次冻结。
	if _, err := app.FreezeBatch(ctx, batch.ID); !errors.Is(err, model.ErrStateMachine) {
		t.Fatalf("expected ErrStateMachine on second freeze, got %v", err)
	}
}

// TestSnapshotFreezesConfirmedRoot 验证发布因果快照时冻结每个候选当时的裁决结果：
// 已确认的根因不得在快照关联中退化为普通 candidate，排除项亦应原样固化。
// 这条回归覆盖「快照证据不可变」语义曾被破坏的情形。
func TestSnapshotFreezesConfirmedRoot(t *testing.T) {
	ctx := context.Background()
	app, st := newTestService(t)
	defer st.Close()

	if err := app.RunDemo(ctx); err != nil {
		t.Fatalf("RunDemo: %v", err)
	}
	batches, err := app.ListBatches(ctx)
	if err != nil || len(batches) == 0 {
		t.Fatalf("list batches: %d %v", len(batches), err)
	}
	batchID := batches[0].ID

	// 发布前的候选状态：应有 confirmed_root 与 excluded（同簇确认后自动排除）。
	cands, err := app.ListCandidates(ctx, batchID)
	if err != nil {
		t.Fatalf("list candidates: %v", err)
	}
	want := map[int64]string{}
	hasConfirmed, hasExcluded := false, false
	for _, c := range cands {
		want[c.ID] = c.Status
		if c.Status == model.CausalStatusConfirmedRoot {
			hasConfirmed = true
		}
		if c.Status == model.CausalStatusExcluded {
			hasExcluded = true
		}
	}
	if !hasConfirmed || !hasExcluded {
		t.Fatalf("demo preconditions: need confirmed_root and excluded candidates, got %+v", want)
	}

	snaps, err := app.ListSnapshots(ctx, batchID)
	if err != nil || len(snaps) == 0 {
		t.Fatalf("list snapshots: %d %v", len(snaps), err)
	}
	frozen, err := app.Repos().Snapshots.ListSnapshotCandidates(ctx, snaps[0].ID)
	if err != nil {
		t.Fatalf("list snapshot candidates: %v", err)
	}
	if len(frozen) != len(cands) {
		t.Fatalf("expected %d frozen candidates, got %d", len(cands), len(frozen))
	}
	for _, sc := range frozen {
		expected, ok := want[sc.CandidateID]
		if !ok {
			t.Fatalf("frozen candidate %d not in batch candidates", sc.CandidateID)
		}
		if sc.Decision != expected {
			t.Fatalf("frozen decision mismatch for candidate %d: want %s, got %s "+
				"(confirmed root must not degrade to candidate)",
				sc.CandidateID, expected, sc.Decision)
		}
		if expected == model.CausalStatusConfirmedRoot && sc.Decision != model.CausalStatusConfirmedRoot {
			t.Fatalf("confirmed root degraded to %s in snapshot", sc.Decision)
		}
	}
}
