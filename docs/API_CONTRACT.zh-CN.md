# Sculpt API 契约

[English](API_CONTRACT.md) · [简体中文](API_CONTRACT.zh-CN.md)

Go 框架的关键不变量（与递归 harnessing 方法对齐）：

1. **预算恒等** — 每种 mode 每任务运行 `R = 4k` 次 rollout 与 `P = 3k` 次 improver session。
2. **Propose–Probe–Compose** — 候选在同一任务上探测；选择对标准答案盲视（context 过滤 leak glob）。
3. **Adoption** — 选中的 harness 物化到 `harnesses/<label>/`，并记入 `reflection/*/adoption.json`。
4. **Library** — 成功经验按 domain/mode 存储，并前缀注入同领域后续任务。
5. **Resume** — 已完成 summary 视为缓存命中，除非采纳产物损坏或重新请求了 modes。
6. **Env** — 部署旋钮使用 `SCULPT_*`（例如 `SCULPT_PHASE_WIDTH`、`SCULPT_IMPROVER_MODEL`、`SCULPT_RUNS_DIR`）。
