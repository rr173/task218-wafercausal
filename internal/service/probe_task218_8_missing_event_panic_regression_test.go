package service

import (
	"context"
	"testing"

	"task218-wafercausal/internal/ingest"
)

func TestTask218Bug08MissingProcessEventReturnsWeakCandidate(t *testing.T) {
	ctx := context.Background()
	app, st := newTestService(t)
	defer st.Close()
	b, err := app.CreateBatch(ctx, "missing-event", "WFR-218-08", 300)
	if err != nil { t.Fatal(err) }
	if _, err := app.AddStepChain(ctx, b.ID, []string{"etch"}); err != nil { t.Fatal(err) }
	if _, err := app.ImportDefects(ctx, b.ID, []ingest.DefectInput{{DetectBatch: "i", X: 1, Y: 1, DetectedAt: "2026-08-24T00:00:00Z"}}); err != nil { t.Fatal(err) }
	if _, err := app.RunClustering(ctx, b.ID, 5000); err != nil { t.Fatal(err) }
	cands, err := app.GenerateCandidates(ctx, b.ID)
	if err != nil { t.Fatal(err) }
	if len(cands) != 1 || cands[0].Score != 0.05 { t.Fatalf("expected weak missing-event candidate, got %+v", cands) }
}
