# 评测与自进化计分

[English](EVAL.md) · [简体中文](EVAL.zh-CN.md)

Sculpt 对齐上游实验管线：

1. **Refine** — `sculpt-demo` / `sculpt` 运行 Propose–Probe–Compose，写出任务 `summary.json` 与采纳的 harness。
2. **Judge refined** — `sculpt-judge` 将选中 run 写入 `judged.json`。
3. **Judge bases** — `sculpt-judge --bases` 将参考执行写入 `base_judged.json`。
4. **Report** — `sculpt-report` 打印 refined 指标；若存在 bases，则输出 **base → refined Δ**（自进化增益，百分点）。

未评分的 judge 失败不计入分母（不会被强制记为 0）。

## 离线 demo 契约

确定性 ledger demo 使用精确匹配评分（`n_passed` / `n_criteria`）：

| 阶段 | 期望 |
|------|------|
| Base harness（仅正数求和） | 0% |
| 一轮 PPC 之后 | 100% |
| Δ | **+100 pt** |

`go test ./demo -run TestSelfEvolution` 覆盖采纳与 library；
`TestSelfEvolutionScored` 覆盖 judge → bases → report 计分。

## 论文线上峰值（参考）

方法论文报告的峰值（定性目标，不是 CI 门槛）：

| Bench | 相对 base 的峰值增益 |
|-------|----------------------|
| WorkBuddy (`wb`) | +11.5 pt |
| JobBench (`jb`) | +4.5 pt |
| LAB (`lab`) | +11.8 pp 部分通过率 |

常量位于 `eval.PaperGains`；当 bench 名匹配时会出现在 report 的 delta 区块。
