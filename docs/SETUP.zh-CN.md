# 安装与配置

[English](SETUP.md) · [简体中文](SETUP.zh-CN.md)

如何在本地跑通 Sculpt：先离线，再按需接线上基准。

## 环境要求

| 需求 | 离线 demo | 线上基准 |
|------|-----------|----------|
| Go 1.22+ | 必需 | 必需 |
| `PATH` 上的 Python 3（`python` / `python3` / `py`） | 必需 | 必需 |
| OpenCode（`oc`）或 Codex（`cx`） | — | `--live` / Execute 需要 |
| 数据集 / runner checkout | — | 按 bench |
| `.env` 中的 API 凭证 | — | 模型调用需要 |

切勿提交 `.env` 或 API Key。

## 离线安装

```bash
git clone https://github.com/linuslin-cto/Sculpt.git
cd Sculpt
go test ./...
go run ./cmd/sculpt-check --bench demo --output runs/check-demo
go run ./cmd/sculpt-demo --output runs/demo --k 1
```

确定性 demo **不需要** 云登录。

## 环境变量

在仓库根目录创建本地 `.env`（已被 gitignore）。Shell 中已导出的变量优先于文件。

| 变量 | 作用 |
|------|------|
| `SCULPT_PHASE_WIDTH` | Propose/rollout 并发（默认 `2`） |
| `SCULPT_TRANSPORT_ATTEMPTS` | 传输重试（默认 `2`） |
| `SCULPT_IMPROVER_MODEL` | Improver / OpenCode proposer 模型 |
| `SCULPT_CLAUDE_MODEL` | Claude CLI proposer 模型 |
| `SCULPT_SOLVER_MODEL` | Live execute 的 solver 模型 |
| `SCULPT_SOLVER_MODEL_CX` | Codex 专用 solver 覆盖 |
| `SCULPT_RUNS_DIR` | 覆盖 runs 根目录 |
| `SCULPT_DEPENDENCIES` | 依赖 / 第三方根目录 |
| `SCULPT_BUN` | Bun 二进制路径（如需） |
| `SCULPT_CODEX_DIST` | Codex 发行目录 |
| `SCULPT_JB_RUNNER_DIR` | JobBench runner checkout |
| `SCULPT_JB_OC_RUNNER` / `SCULPT_JB_CX_RUNNER` | JobBench OpenCode / Codex 入口 |
| `SCULPT_JUDGE_MODEL` / `SCULPT_JUDGE_BASE_URL` / `SCULPT_JUDGE_API_KEY` | Live judge（JobBench） |

## Substrate

| 参数 | 名称 | 说明 |
|------|------|------|
| `--substrate oc` | OpenCode | 默认 |
| `--substrate cx` | Codex | 已安装并配置 Codex 时使用 |

带二进制的预检：

```bash
go run ./cmd/sculpt-check --bench demo --live --output runs/check-demo
go run ./cmd/sculpt-check --bench jb --substrate oc --output runs/check-jb
go run ./cmd/sculpt-check --bench lab --substrate cx
go run ./cmd/sculpt-check --bench wb --substrate oc
```

## 基准

| 别名 | 侧重 | 常见额外依赖 |
|------|------|--------------|
| `demo` | 离线 ledger / PPC | 仅需 Python |
| `jb` | JobBench | runner 目录 + judge 环境 |
| `lab` | LAB | 适配器所需数据集 / 工具链 |
| `wb` | WorkBuddy-Bench | 数据集 / Docker 或 runner |

适配器已接线；完整 live Execute 仍需可用的 substrate checkout 与凭证。在此之前，请优先使用 `sculpt --dry-run` 与离线 `sculpt-demo`。

```bash
go run ./cmd/sculpt --bench demo --output runs/demo --k 1 --seed 7 --dry-run
```

## 验证

```bash
go test ./...
go test ./demo -run TestSelfEvolution
go run ./cmd/sculpt-demo --output runs/demo --k 1
go run ./cmd/sculpt-judge --bench demo --output runs/demo --k 1
go run ./cmd/sculpt-judge --bench demo --output runs/demo --k 1 --bases
go run ./cmd/sculpt-report --bench demo --output runs/demo --k 1
```

离线期望 Δ：**base 0% → refined 100%（+100 pt）**。详见 [EVAL.zh-CN.md](EVAL.zh-CN.md)。

## 相关文档

- [API_CONTRACT.zh-CN.md](API_CONTRACT.zh-CN.md) — 运行时不变量
- [EVAL.zh-CN.md](EVAL.zh-CN.md) — judge / report 管线
- [i18n/README.md](i18n/README.md) — 文档多语言
