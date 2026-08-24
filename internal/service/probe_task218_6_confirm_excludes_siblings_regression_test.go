package service

import (
	"context"
	"testing"

	"task218-wafercausal/internal/ingest"
	"task218-wafercausal/internal/model"
)

func TestTask218Bug06ConfirmExcludesSiblingCandidates(t *testing.T) {
	ctx := context.Background()
	app, st := newTestService(t)
	defer st.Close()
	b, err := app.CreateBatch(ctx, "ruling", "WFR-218-06", 300)
	if err != nil { t.Fatal(err) }
	steps, err := app.AddStepChain(ctx, b.ID, []string{"etch", "clean"})
	if err != nil { t.Fatal(err) }
	if _, err := app.ImportEvents(ctx, b.ID, []ingest.EventInput{{StepID: steps[0].ID, OccurredAt: "2026-08-24T00:00:00Z"}, {StepID: steps[1].ID, OccurredAt: "2026-08-24T00:01:00Z"}}); err != nil { t.Fatal(err) }
	if _, err := app.ImportDefects(ctx, b.ID, []ingest.DefectInput{{DetectBatch: "i", X: 1, Y: 1, DetectedAt: "2026-08-24T00:02:00Z"}}); err != nil { t.Fatal(err) }
	if _, err := app.RunClustering(ctx, b.ID, 5000); err != nil { t.Fatal(err) }
	cands, err := app.GenerateCandidates(ctx, b.ID)
	if err != nil || len(cands) != 2 { t.Fatalf("expected two candidates, got %d %v", len(cands), err) }
	if _, err := app.ConfirmCandidate(ctx, b.ID, cands[0].ID, "engineer", "confirmed"); err != nil { t.Fatal(err) }
	got, err := app.ListCandidates(ctx, b.ID)
	if err != nil { t.Fatal(err) }
	confirmed, excluded := 0, 0
	for _, c := range got { if c.Status == model.CausalStatusConfirmedRoot { confirmed++ }; if c.Status == model.CausalStatusExcluded { excluded++ } }
	if confirmed != 1 || excluded != 1 { t.Fatalf("expected one confirmed and one excluded, got %+v", got) }
}
