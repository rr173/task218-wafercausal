package service

import (
	"context"
	"testing"

	"task218-wafercausal/internal/ingest"
)

func TestTask218Bug05CandidatePathIncludesRootAndDownstreamSteps(t *testing.T) {
	ctx := context.Background()
	app, st := newTestService(t)
	defer st.Close()
	b, err := app.CreateBatch(ctx, "path", "WFR-218-05", 300)
	if err != nil { t.Fatal(err) }
	steps, err := app.AddStepChain(ctx, b.ID, []string{"litho", "etch", "clean"})
	if err != nil { t.Fatal(err) }
	if _, err := app.ImportEvents(ctx, b.ID, []ingest.EventInput{{StepID: steps[1].ID, OccurredAt: "2026-08-24T00:00:00Z"}}); err != nil { t.Fatal(err) }
	if _, err := app.ImportDefects(ctx, b.ID, []ingest.DefectInput{{DetectBatch: "i", X: 1, Y: 1, DetectedAt: "2026-08-24T00:05:00Z"}}); err != nil { t.Fatal(err) }
	if _, err := app.RunClustering(ctx, b.ID, 5000); err != nil { t.Fatal(err) }
	cands, err := app.GenerateCandidates(ctx, b.ID)
	if err != nil { t.Fatal(err) }
	if len(cands) == 0 { t.Fatal("expected candidate") }
	ids, err := app.Repos().Causals.ListCandidateSteps(ctx, cands[0].ID)
	if err != nil { t.Fatal(err) }
	if len(ids) != 2 || ids[0] != steps[1].ID || ids[1] != steps[2].ID { t.Fatalf("unexpected causal path %v", ids) }
}
