基于 Go 实现的芯片晶圆缺陷因果链追踪服务，一款纯后端半导体分析服务，处理缺陷记录回溯、工艺证据关联与版本化因果快照发布。

# task218-wafercausal 评测说明

## 启动与自检

```bash
# 构建（本地）
CGO_ENABLED=0 GOTOOLCHAIN=local go build -o wafercausal ./cmd/wafercausal

# 端到端自检（退出码 0 为通过）
./wafercausal --smoke-test
```

`--smoke-test` 从空库开始执行确定性端到端演示：建批次 → 建工艺链 → 导入缺陷（含幂等重复）与工艺事件（含不可信记录）→ 冻结 → 空间聚类 → 生成因果候选 → 确认根因 → 发布快照 → 关闭并重开同一数据库验证持久化恢复，全部断言通过后以 0 退出。

## HTTP 服务

```bash
./wafercausal --addr :8080 --db wafercausal.db
```

- 健康检查：`GET /healthz`
- 自检：`GET /api/selfcheck`
- 统计：`GET /api/stats`

## Docker 双架构

镜像同时支持 `linux/amd64` 与 `linux/arm64`：

```bash
docker build --platform linux/amd64 -t wafercausal:amd64 .
docker run wafercausal:amd64    # 默认 CMD 为 --smoke-test
```

## 组件版本

| 组件 | 版本 |
| --- | --- |
| Go | 1.26.3 |
| SQLite | 3.46.1 |
