# task218-wafercausal 芯片晶圆缺陷因果链追踪服务

从晶圆缺陷记录回溯最可能的工艺步骤与检测证据，构造可审计的因果链。面向半导体良率工程师：导入晶圆批次、工艺步骤与缺陷/事件数据，服务按时间、晶圆位置和工艺依赖生成根因候选，工程师裁决并发布版本化因果快照。

## 业务闭环

1. 创建晶圆批次并定义工艺步骤链（光刻 → 蚀刻 → 沉积 → 清洗 …）。
2. 批量接收缺陷记录（带坐标、检测批次、时间，指纹幂等去重）与工艺事件（设备、参数、时间）。
3. 冻结批次后按坐标邻近执行空间聚类。
4. 按时间衰减与设备可信度为每个空间簇生成因果候选路径。
5. 工程师确认根因 / 排除冲突来源，标记设备数据不可信。
6. 发布不可变因果快照（版本化，旧版本自动替代）。

## 标准命令

```bash
# 启动 HTTP 服务
go run ./cmd/wafercausal --addr :8080 --db wafercausal.db

# 确定性端到端自检（Docker 判据，结束后退出）
go run ./cmd/wafercausal --smoke-test

# 构建 / 静态检查 / 测试
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test ./...
```

## API 入口（统一 /api 前缀）

- 批次：`POST/GET /api/batches`、`GET /api/batches/{id}`、`POST /api/batches/{id}/freeze`、`POST /api/batches/{id}/archive`
- 工艺步骤：`POST /api/batches/{id}/steps`、`POST /api/batches/{id}/steps/chain`、`GET /api/batches/{id}/steps`、`PUT /api/steps/{id}/parent`
- 缺陷：`POST/GET /api/batches/{id}/defects`
- 工艺事件：`POST/GET /api/batches/{id}/events`、`PUT /api/events/{id}/untrusted`
- 聚类：`POST/GET /api/batches/{id}/clusters`
- 因果候选：`POST/GET /api/batches/{id}/causal`、`GET /api/causal/{id}`
- 裁决：`POST /api/causal/{id}/confirm`、`POST /api/causal/{id}/exclude`、`GET /api/batches/{id}/rulings`
- 快照：`POST/GET /api/batches/{id}/snapshots`、`GET /api/snapshots/{id}`
- 元信息：`GET /healthz`、`GET /api/selfcheck`、`GET /api/stats`、`GET /api/audit/events`

## 持久化

SQLite（`modernc.org/sqlite` 纯 Go 驱动，CGO 无关）。实体全部落盘，服务重启后通过数据库完整恢复；缺陷记录按内容指纹幂等，封存批次只读，快照不可变。

## 状态机

- 晶圆批次：`producing → pending_analysis → confirmed → archived`
- 缺陷：`new → clustered → linked / false_positive`
- 工艺事件：`valid / missing / conflicting / excluded / untrusted`
- 空间簇：`candidate → confirmed`
- 因果候选：`candidate → confirmed_root / excluded / superseded`
- 因果快照：`pending → under_review → published → superseded`
