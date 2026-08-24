// Package model 定义芯片晶圆缺陷因果链追踪服务的领域实体、状态机与通用工具。
package model

import "time"

// ---------------------------------------------------------------------------
// 状态机常量
// ---------------------------------------------------------------------------

// 晶圆批次状态：生产中 → 待分析 → 已确认 → 封存。
const (
	BatchProducing = "producing"       // 生产中：仍可能写入缺陷与工艺事件
	BatchPending   = "pending_analysis" // 待分析：数据冻结，等待因果分析
	BatchConfirmed = "confirmed"        // 已确认：根因已裁决
	BatchArchived  = "archived"         // 封存：只读，仅能建立替代版本
)

// 缺陷状态：新发现 → 聚类 → 已关联；或误报。
const (
	DefectNew       = "new"            // 新发现
	DefectClustered = "clustered"      // 已聚类
	DefectLinked    = "linked"         // 已关联到因果候选
	DefectFalsePos  = "false_positive" // 误报
)

// 工艺事件状态：有效 / 缺失 / 冲突 / 排除；并可用不可信标记弱化其证据权重。
const (
	EventValid       = "valid"       // 有效
	EventMissing     = "missing"     // 缺失（依赖步骤无事件记录）
	EventConflicting = "conflicting" // 冲突（同步骤多设备记录不一致）
	EventExcluded    = "excluded"    // 排除
	EventUntrusted   = "untrusted"   // 设备数据不可信
)

// 空间簇状态：候选 → 确认。
const (
	ClusterCandidate = "candidate" // 候选
	ClusterConfirmed = "confirmed" // 确认
)

// 因果候选状态：候选 → 确认根因 / 排除 / 被替代。
const (
	CausalStatusCandidate     = "candidate"      // 候选
	CausalStatusConfirmedRoot = "confirmed_root" // 已确认根因
	CausalStatusExcluded      = "excluded"       // 已排除
	CausalStatusSuperseded    = "superseded"     // 被后续快照替代
)

// 因果快照状态：待计算 → 待复核 → 发布 → 替代。
const (
	SnapshotPending     = "pending"      // 待计算
	SnapshotUnderReview = "under_review" // 待复核
	SnapshotPublished   = "published"    // 发布
	SnapshotSuperseded  = "superseded"   // 替代
)

// ---------------------------------------------------------------------------
// 时间工具
// ---------------------------------------------------------------------------

// NowStr 返回当前时间的 RFC3339 字符串，用于落盘时间戳。
func NowStr() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// ---------------------------------------------------------------------------
// 实体
// ---------------------------------------------------------------------------

// WaferBatch 晶圆批次：一片晶圆经历完整工艺与检测流程的容器。
type WaferBatch struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	WaferID     string `json:"wafer_id"`     // 晶圆编号（业务唯一键）
	DiameterMM  float64 `json:"diameter_mm"` // 晶圆直径（毫米），用于坐标越界校验
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	ArchivedAt  string `json:"archived_at,omitempty"`
}

// ProcessStep 工艺步骤：晶圆在生产中经历的一个工艺环节，可带前置依赖形成链。
type ProcessStep struct {
	ID           int64  `json:"id"`
	BatchID      int64  `json:"batch_id"`
	Name         string `json:"name"`
	Sequence     int    `json:"sequence"`       // 执行顺序
	ParentStepID int64  `json:"parent_step_id"` // 前置步骤（0 表示无依赖，链路起点）
	Tool         string `json:"tool"`           // 负责设备
	CreatedAt    string `json:"created_at"`
}

// DefectRecord 缺陷记录：检测设备在某坐标发现的一个缺陷，带检测批次与时间。
type DefectRecord struct {
	ID           int64   `json:"id"`
	BatchID      int64   `json:"batch_id"`
	DetectBatch  string  `json:"detect_batch"` // 检测批次号
	X            float64 `json:"x"`            // 晶圆坐标 X（毫米）
	Y            float64 `json:"y"`            // 晶圆坐标 Y（毫米）
	RadiusUM     float64 `json:"radius_um"`    // 缺陷半径（微米）
	Severity     string  `json:"severity"`     // 严重度：minor/major/critical
	DetectedAt   string  `json:"detected_at"`  // 检测时间
	Fingerprint  string  `json:"fingerprint"`  // 幂等指纹
	Status       string  `json:"status"`
	CreatedAt    string  `json:"created_at"`
}

// ProcessEvent 工艺事件：某工艺步骤在某时刻由某设备以某参数执行的记录。
type ProcessEvent struct {
	ID          int64  `json:"id"`
	BatchID     int64  `json:"batch_id"`
	StepID      int64  `json:"step_id"`
	Tool        string `json:"tool"`
	Param       string `json:"param"`       // 工艺参数快照（JSON）
	OccurredAt  string `json:"occurred_at"` // 发生时间
	Status      string `json:"status"`
	Untrusted   bool   `json:"untrusted"` // 设备数据是否被标记不可信
	CreatedAt   string `json:"created_at"`
}

// SpatialCluster 空间簇：按坐标邻近聚合的一组缺陷。
type SpatialCluster struct {
	ID         int64   `json:"id"`
	BatchID    int64   `json:"batch_id"`
	CentroidX  float64 `json:"centroid_x"`
	CentroidY  float64 `json:"centroid_y"`
	RadiusUM   float64 `json:"radius_um"` // 簇等效半径（微米）
	DefectCnt  int     `json:"defect_count"`
	Status     string  `json:"status"`
	CreatedAt  string  `json:"created_at"`
}

// ClusterDefect 空间簇与缺陷的归属关系。
type ClusterDefect struct {
	ClusterID int64 `json:"cluster_id"`
	DefectID  int64 `json:"defect_id"`
}

// CausalCandidate 因果候选：一个空间簇对应的一个根因假设。
type CausalCandidate struct {
	ID          int64   `json:"id"`
	BatchID     int64   `json:"batch_id"`
	ClusterID   int64   `json:"cluster_id"`
	RootStepID  int64   `json:"root_step_id"` // 最可能的根因工艺步骤
	Score       float64 `json:"score"`        // 置信度 0~1
	Evidence    string  `json:"evidence"`     // 证据路径摘要
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
}

// CandidateStep 因果候选路径上的工艺步骤（按依赖顺序排列）。
type CandidateStep struct {
	CandidateID int64 `json:"candidate_id"`
	StepID      int64 `json:"step_id"`
	Order       int   `json:"order"`
}

// EvidenceRuling 证据裁决：工程师对某候选做出的确认/排除决定。
type EvidenceRuling struct {
	ID          int64  `json:"id"`
	BatchID     int64  `json:"batch_id"`
	CandidateID int64  `json:"candidate_id"`
	Decision    string `json:"decision"` // confirm / exclude
	Reason      string `json:"reason"`
	Actor       string `json:"actor"`
	CreatedAt   string `json:"created_at"`
}

// CausalSnapshot 因果快照：发布后的不可变分析结论，版本化。
type CausalSnapshot struct {
	ID            int64  `json:"id"`
	BatchID       int64  `json:"batch_id"`
	Version       int    `json:"version"` // 批次内递增版本号
	Name          string `json:"name"`
	Status        string `json:"status"`
	RootSummary   string `json:"root_summary"` // 根因结论摘要
	CreatedAt     string `json:"created_at"`
	SupersededAt  string `json:"superseded_at,omitempty"`
}

// SnapshotCandidate 快照与因果候选的冻结关联。
type SnapshotCandidate struct {
	SnapshotID  int64  `json:"snapshot_id"`
	CandidateID int64  `json:"candidate_id"`
	Decision    string `json:"decision"` // 该候选在快照中的最终状态
}
