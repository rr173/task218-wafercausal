// Package ingest 负责缺陷记录与工艺事件的接收、校验与幂等去重。
package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"

	"task218-wafercausal/internal/model"
	"task218-wafercausal/internal/store"
)

// DefectInput 一次缺陷接收请求的原始输入。
type DefectInput struct {
	DetectBatch string  `json:"detect_batch"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	RadiusUM    float64 `json:"radius_um"`
	Severity    string  `json:"severity"`
	DetectedAt  string  `json:"detected_at"`
}

// EventInput 一次工艺事件接收请求的原始输入。
type EventInput struct {
	StepID     int64  `json:"step_id"`
	Tool       string `json:"tool"`
	Param      string `json:"param"`
	OccurredAt string `json:"occurred_at"`
}

// ImportResult 批量接收的结果统计。
type ImportResult struct {
	Accepted int     `json:"accepted"`
	Skipped  int     `json:"skipped"` // 幂等跳过
	IDs      []int64 `json:"ids"`
}

// Importer 负责校验与幂等写入。
type Importer struct {
	repos *store.Repositories
}

// NewImporter 构造导入器。
func NewImporter(repos *store.Repositories) *Importer {
	return &Importer{repos: repos}
}

// Fingerprint 计算缺陷记录的内容指纹，用于同批次幂等去重。
// 指纹覆盖晶圆、检测批次、坐标、时间与半径，同一条检测记录重复提交得到相同指纹。
func Fingerprint(waferID, detectBatch string, x, y, radiusUM float64, detectedAt string) string {
	raw := fmt.Sprintf("%s|%s|%.3f|%.3f|%.2f|%s",
		waferID, detectBatch, x, y, radiusUM, detectedAt)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ImportDefects 批量接收缺陷记录。逐条校验并幂等写入，重复项跳过。
// 已封存批次只读：拒绝写入且不产生任何新缺陷。
func (im *Importer) ImportDefects(ctx context.Context, batch *model.WaferBatch, inputs []DefectInput) (*ImportResult, error) {
	if batch.Status == model.BatchArchived {
		return nil, model.ErrArchived
	}
	if len(inputs) == 0 {
		return nil, fmt.Errorf("%w: empty defect batch", model.ErrInvalid)
	}
	r := batch.DiameterMM / 2.0
	res := &ImportResult{IDs: make([]int64, 0, len(inputs))}
	now := model.NowStr()
	for _, in := range inputs {
		if err := validateDefect(batch.DiameterMM, r, in); err != nil {
			return res, err
		}
		fp := Fingerprint(batch.WaferID, in.DetectBatch, in.X, in.Y, in.RadiusUM, in.DetectedAt)
		d := &model.DefectRecord{
			BatchID:     batch.ID,
			DetectBatch: in.DetectBatch,
			X:           in.X,
			Y:           in.Y,
			RadiusUM:    in.RadiusUM,
			Severity:    normalizeSeverity(in.Severity),
			DetectedAt:  in.DetectedAt,
			Fingerprint: fp,
			Status:      model.DefectNew,
			CreatedAt:   now,
		}
		created, err := im.repos.Defects.InsertDefect(ctx, d)
		if err == model.ErrDuplicate {
			res.Skipped++
			continue
		}
		if err != nil {
			return res, err
		}
		res.Accepted++
		res.IDs = append(res.IDs, created.ID)
	}
	return res, nil
}

// ImportEvents 批量接收工艺事件。逐条校验并写入。
func (im *Importer) ImportEvents(ctx context.Context, batch *model.WaferBatch, inputs []EventInput) (*ImportResult, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("%w: empty event batch", model.ErrInvalid)
	}
	res := &ImportResult{IDs: make([]int64, 0, len(inputs))}
	now := model.NowStr()
	for _, in := range inputs {
		if err := validateEvent(batch, in, im.repos); err != nil {
			return res, err
		}
		e := &model.ProcessEvent{
			BatchID:    batch.ID,
			StepID:     in.StepID,
			Tool:       in.Tool,
			Param:      normalizeParam(in.Param),
			OccurredAt: in.OccurredAt,
			Status:     model.EventValid,
			Untrusted:  false,
			CreatedAt:  now,
		}
		created, err := im.repos.Events.InsertEvent(ctx, e)
		if err != nil {
			return res, err
		}
		res.Accepted++
		res.IDs = append(res.IDs, created.ID)
	}
	return res, nil
}

// validateDefect 校验缺陷记录：坐标必须落在晶圆圆内、时间非空、半径非负。
func validateDefect(diameter, r float64, in DefectInput) error {
	if strings.TrimSpace(in.DetectBatch) == "" {
		return fmt.Errorf("%w: detect_batch is required", model.ErrInvalid)
	}
	if strings.TrimSpace(in.DetectedAt) == "" {
		return fmt.Errorf("%w: detected_at is required", model.ErrInvalid)
	}
	if in.RadiusUM < 0 {
		return fmt.Errorf("%w: radius_um must be non-negative", model.ErrInvalid)
	}
	dist := math.Hypot(in.X, in.Y)
	if dist > r {
		return fmt.Errorf("%w: coordinate (%.2f, %.2f) out of wafer radius %.2f",
			model.ErrInvalid, in.X, in.Y, r)
	}
	_ = diameter
	return nil
}

// validateEvent 校验工艺事件：步骤必须属于该批次。
func validateEvent(batch *model.WaferBatch, in EventInput, repos *store.Repositories) error {
	if strings.TrimSpace(in.OccurredAt) == "" {
		return fmt.Errorf("%w: occurred_at is required", model.ErrInvalid)
	}
	if in.StepID <= 0 {
		return fmt.Errorf("%w: step_id is required", model.ErrInvalid)
	}
	step, err := repos.Steps.GetStep(context.Background(), in.StepID)
	if err != nil {
		return err
	}
	if step.BatchID != batch.ID {
		return fmt.Errorf("%w: step %d not in batch %d", model.ErrInvalid, in.StepID, batch.ID)
	}
	return nil
}

// normalizeSeverity 归一化严重度，空值回退 minor。
func normalizeSeverity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "major":
		return "major"
	case "critical":
		return "critical"
	default:
		return "minor"
	}
}

// normalizeParam 归一化工艺参数，空值回退空对象。
func normalizeParam(p string) string {
	if strings.TrimSpace(p) == "" {
		return "{}"
	}
	return p
}
