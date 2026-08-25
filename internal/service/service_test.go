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

// TestSnapshotSupersession 验证同一批次发布新因果快照后，
// 旧版本被标记为已替代（superseded），而新版本保持 published。
func TestSnapshotSupersession(t *testing.T) {
	ctx := context.Background()
	app, st := newTestService(t)
	defer st.Close()

	batch, err := app.CreateBatch(ctx, "sup-wafer", "WFR-SUP", 300)
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	steps, err := app.AddStepChain(ctx, batch.ID, []string{"litho", "etch"})
	if err != nil {
		t.Fatalf("add step chain: %v", err)
	}
	if _, err := app.ImportEvents(ctx, batch.ID, []ingest.EventInput{
		{StepID: steps[0].ID, Tool: "scanner-1", Param: "{}", OccurredAt: "2026-08-23T08:00:00Z"},
		{StepID: steps[1].ID, Tool: "etcher-1", Param: "{}", OccurredAt: "2026-08-23T08:10:00Z"},
	}); err != nil {
		t.Fatalf("import events: %v", err)
	}
	if _, err := app.ImportDefects(ctx, batch.ID, []ingest.DefectInput{
		{DetectBatch: "i1", X: 10, Y: 10, RadiusUM: 2, Severity: "major", DetectedAt: "2026-08-23T08:30:00Z"},
	}); err != nil {
		t.Fatalf("import defects: %v", err)
	}
	if _, err := app.FreezeBatch(ctx, batch.ID); err != nil {
		t.Fatalf("freeze batch: %v", err)
	}
	if _, err := app.RunClustering(ctx, batch.ID, 5000); err != nil {
		t.Fatalf("run clustering: %v", err)
	}
	if _, err := app.GenerateCandidates(ctx, batch.ID); err != nil {
		t.Fatalf("generate candidates: %v", err)
	}
	cands, err := app.ListCandidates(ctx, batch.ID)
	if err != nil || len(cands) == 0 {
		t.Fatalf("expected candidates, got %d %v", len(cands), err)
	}
	if _, err := app.ConfirmCandidate(ctx, batch.ID, cands[0].ID, "engineer", "root cause"); err != nil {
		t.Fatalf("confirm candidate: %v", err)
	}

	// 发布首版快照：应保持 published，无旧版本可替代。
	snap1, err := app.PublishSnapshot(ctx, batch.ID, "v1")
	if err != nil {
		t.Fatalf("publish v1: %v", err)
	}
	if snap1.Version != 1 || snap1.Status != model.SnapshotPublished {
		t.Fatalf("v1 expected version=1 published, got version=%d status=%s", snap1.Version, snap1.Status)
	}

	// 发布次版快照：旧版本应被标记为已替代，新版本保持 published。
	snap2, err := app.PublishSnapshot(ctx, batch.ID, "v2")
	if err != nil {
		t.Fatalf("publish v2: %v", err)
	}
	if snap2.Version != 2 || snap2.Status != model.SnapshotPublished {
		t.Fatalf("v2 expected version=2 published, got version=%d status=%s", snap2.Version, snap2.Status)
	}
	got1, err := app.GetSnapshot(ctx, snap1.ID)
	if err != nil {
		t.Fatalf("get v1: %v", err)
	}
	if got1.Status != model.SnapshotSuperseded || got1.SupersededAt == "" {
		t.Fatalf("v1 expected superseded with timestamp, got status=%s superseded_at=%q", got1.Status, got1.SupersededAt)
	}
	got2, err := app.GetSnapshot(ctx, snap2.ID)
	if err != nil {
		t.Fatalf("get v2: %v", err)
	}
	if got2.Status != model.SnapshotPublished || got2.SupersededAt != "" {
		t.Fatalf("v2 expected published with no supersede timestamp, got status=%s superseded_at=%q", got2.Status, got2.SupersededAt)
	}
}
