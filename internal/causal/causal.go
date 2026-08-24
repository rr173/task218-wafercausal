// Package causal 实现晶圆缺陷的因果候选生成：按时间、空间与工艺依赖
// 为每个空间簇构造最可能的根因工艺步骤及其证据路径。
package causal

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"task218-wafercausal/internal/model"
	"task218-wafercausal/internal/store"
)

// 时间衰减常数：事件与缺陷检测的时间差每过一个 tau 得分衰减 e 倍。
const timeTau = 30 * time.Minute

// CandidateSpec 一个待落盘的因果候选规格。
type CandidateSpec struct {
	ClusterID  int64
	RootStepID int64
	Score      float64
	Evidence   string
	StepIDs    []int64
}

// Generator 因果候选生成器。
type Generator struct {
	repos *store.Repositories
}

// NewGenerator 构造生成器。
func NewGenerator(repos *store.Repositories) *Generator {
	return &Generator{repos: repos}
}

// Generate 为某批次的所有空间簇生成因果候选（每簇至多 2 个根因假设）。
func (g *Generator) Generate(ctx context.Context, batchID int64) ([]CandidateSpec, error) {
	clusters, err := g.repos.Clusters.ListClustersByBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if len(clusters) == 0 {
		return nil, fmt.Errorf("%w: no clusters for batch %d", model.ErrInvalid, batchID)
	}
	steps, err := g.repos.Steps.ListStepsByBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("%w: no process steps for batch %d", model.ErrInvalid, batchID)
	}
	events, err := g.repos.Events.ListEventsByBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	// 建立 stepID → 事件列表 索引。
	eventsByStep := make(map[int64][]*model.ProcessEvent)
	for _, e := range events {
		eventsByStep[e.StepID] = append(eventsByStep[e.StepID], e)
	}
	var out []CandidateSpec
	for _, cl := range clusters {
		defects, err := g.repos.Defects.ListDefectsByCluster(ctx, cl.ID)
		if err != nil {
			return nil, err
		}
		specs := g.generateForCluster(cl, defects, steps, eventsByStep)
		out = append(out, specs...)
	}
	return out, nil
}

// generateForCluster 为单个簇生成至多两个根因候选。
func (g *Generator) generateForCluster(
	cl *model.SpatialCluster,
	defects []*model.DefectRecord,
	steps []*model.ProcessStep,
	eventsByStep map[int64][]*model.ProcessEvent,
) []CandidateSpec {
	tRef := referenceTime(defects)
	type scored struct {
		step  *model.ProcessStep
		score float64
	}
	var scoredSteps []scored
	for _, st := range steps {
		evs := eventsByStep[st.ID]
		s := scoreStep(st, evs, tRef)
		scoredSteps = append(scoredSteps, scored{step: st, score: s})
	}
	sort.Slice(scoredSteps, func(i, j int) bool {
		return scoredSteps[i].score > scoredSteps[j].score
	})
	var specs []CandidateSpec
	for i := 0; i < len(scoredSteps) && i < 2; i++ {
		root := scoredSteps[i].step
		path := downstreamPath(root, steps)
		ev := evidenceText(root, scoredSteps[i].score, eventsByStep[root.ID], tRef)
		specs = append(specs, CandidateSpec{
			ClusterID:  cl.ID,
			RootStepID: root.ID,
			Score:      round2(scoredSteps[i].score),
			Evidence:   ev,
			StepIDs:    path,
		})
	}
	return specs
}

// referenceTime 返回簇内缺陷的参考时间：取最早检测时间。
func referenceTime(defects []*model.DefectRecord) time.Time {
	var t time.Time
	first := true
	for _, d := range defects {
		parsed, err := time.Parse(time.RFC3339, d.DetectedAt)
		if err != nil {
			continue
		}
		if first || parsed.Before(t) {
			t = parsed
			first = false
		}
	}
	if first {
		return time.Now().UTC()
	}
	return t
}

// scoreStep 计算某工艺步骤相对缺陷参考时间的根因得分。
// 得分 = 时间接近度 × 设备可信度；无事件记录的步骤给极低的缺失分。
func scoreStep(st *model.ProcessStep, evs []*model.ProcessEvent, tRef time.Time) float64 {
	if len(evs) == 0 {
		return 0.05 // 缺失事件：证据弱
	}
	best := 0.0
	trustFactor := 1.0
	for _, e := range evs {
		if e.Untrusted {
			trustFactor = math.Min(trustFactor, 0.3)
		}
		parsed, err := time.Parse(time.RFC3339, e.OccurredAt)
		if err != nil {
			continue
		}
		dt := tRef.Sub(parsed)
		if dt < 0 {
			// 事件晚于缺陷检测：作为根因的可能性低，但仍保留微弱得分。
			dt = 0
		}
		s := math.Exp(-dt.Seconds() / timeTau.Seconds())
		if s > best {
			best = s
		}
	}
	return best * trustFactor
}

// downstreamPath 返回从 root 步骤沿依赖顺序（seq 递增）到链尾的步骤 ID。
func downstreamPath(root *model.ProcessStep, steps []*model.ProcessStep) []int64 {
	var ids []int64
	for _, st := range steps {
		if st.Sequence >= root.Sequence {
			ids = append(ids, st.ID)
		}
	}
	return ids
}

// evidenceText 生成证据路径摘要字符串。
func evidenceText(root *model.ProcessStep, score float64, evs []*model.ProcessEvent, tRef time.Time) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("root=%s(seq=%d)", root.Name, root.Sequence))
	if len(evs) == 0 {
		sb.WriteString("; no process event recorded (missing-evidence)")
	} else {
		sb.WriteString(fmt.Sprintf("; events=%d", len(evs)))
		trusted := 0
		for _, e := range evs {
			if !e.Untrusted {
				trusted++
			}
		}
		if trusted < len(evs) {
			sb.WriteString(fmt.Sprintf("; untrusted=%d", len(evs)-trusted))
		}
	}
	sb.WriteString(fmt.Sprintf("; score=%.3f; ref=%s", score, tRef.Format(time.RFC3339)))
	return sb.String()
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
