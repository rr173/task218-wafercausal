package service

import (
	"context"
	"strings"
	"testing"

	"task218-wafercausal/internal/ingest"
)

func TestTask218Bug03UntrustedEventLowersCausalWeight(t *testing.T) {
	ctx := context.Background()
	app, st := newTestService(t)
	defer st.Close()
	b, err := app.CreateBatch(ctx, "causal", "WFR-218-03", 300)
	if err != nil { t.Fatal(err) }
	steps, err := app.AddStepChain(ctx, b.ID, []string{"etch"})
	if err != nil { t.Fatal(err) }
	events, err := app.ImportEvents(ctx, b.ID, []ingest.EventInput{{StepID: steps[0].ID, OccurredAt: "2026-08-24T00:00:00Z"}})
	if err != nil { t.Fatal(err) }
	if err := app.MarkEventUntrusted(ctx, events.IDs[0], true); err != nil { t.Fatal(err) }
	if _, err := app.ImportDefects(ctx, b.ID, []ingest.DefectInput{{DetectBatch: "i", X: 1, Y: 1, DetectedAt: "2026-08-24T00:10:00Z"}}); err != nil { t.Fatal(err) }
	if _, err := app.RunClustering(ctx, b.ID, 5000); err != nil { t.Fatal(err) }
	cands, err := app.GenerateCandidates(ctx, b.ID)
	if err != nil { t.Fatal(err) }
	if len(cands) != 1 || cands[0].Score >= 0.5 || !strings.Contains(cands[0].Evidence, "untrusted=1") {
		t.Fatalf("untrusted evidence was not discounted: %+v", cands)
	}
}
