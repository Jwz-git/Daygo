# 03 数据模型

> **状态：设计。** 本文定义 Daygo 自有的持久化结构。schema 尚未落盘；落盘时以本文为准，
> 实现与本文冲突时以代码为准并在同一 commit 修正本文。

功能模块按需求增量落盘表与 repository，全部位于 internal/storage。
[data](modules/data.md) 负责唯一连接、迁移机制 / 编号、锁与可观测封装；功能负责业务表和查询，
[09 §9.7](09-roadmap.md#97-需求接口与测试归属) 跟踪归属。
迁移仍从 user_version=1 开始逐版本验收，不因开发模块并行而出现分叉迁移链。
app_settings repository 归 data，类型化访问归 preferences；没有第二套设置数据库。

## 3.1 磁盘布局

```text
~/Library/Application Support/Daygo/
├── daygo.sqlite (+ -wal, -shm)   业务数据库
├── recordings/                   分段文件，yyyyMMdd_HHmmssSSS.<ext>
├── timelapses/<yyyy-MM-dd>/      每张卡片的时间压缩视频
├── backups/                      每日数据库副本
└── agent.sock                    外部写入通道（推迟到 v1.1）
```

数据库是唯一的结构化事实来源。**设置也在数据库里**（`app_settings` 表），不使用
`UserDefaults` / plist——这样设置能与受其影响的数据在同一事务内变更，也不必处理偏好设置
守护进程的缓存与覆盖时序。

密钥**不在**这里：provider 密钥存系统钥匙串，service 为
`io.github.jwz-git.daygo.apikeys.<provider>`，见 [07 §7.3](07-privacy-security.md#73-密钥)。

## 3.2 凌晨 4 点逻辑日

**一天从本地时间凌晨 4 点开始，而不是午夜。** 凌晨 2:00 的活动属于时间线上的*前一天*。
这是产品决策：深夜工作在用户心智里属于前一天。

两个日期概念并存，不可混用：

| 概念 | 边界 | 字段名 | 用途 |
|------|------|--------|------|
| 逻辑日 `day` | 凌晨 4 点 | `day` | 时间线、日记、目标、卡片归属 |
| 日历日 `standupDay` | 午夜 | `standupDay` | 每日站会摘要 |

三条硬规则：

1. **逻辑日只能由 `internal/timeutil` 生成。** 禁止 `time.Truncate(24*time.Hour)` 或任何
   简单午夜计算。前端**不得**用 `Date` 自行推算逻辑日——午夜到凌晨 4 点之间自算必然错一天。
2. 实现必须覆盖 DST 切换日、半小时时区（如 `Asia/Kolkata`）和 45 分钟时区
   （如 `Asia/Kathmandu`）。这些是夹具测试的必测项。
3. 逻辑日窗口是**左闭右开** `[dayStartTs, dayEndTs)`。

## 3.3 表结构

`PRAGMA user_version` 从 `1` 起，配套版本化迁移链（`internal/storage/migrate.go`）。
连接层固定 `journal_mode=WAL`、`synchronous=NORMAL`、`busy_timeout=5000`。

### 3.3.1 捕获与分析

```sql
-- 帧索引。一行 = 一帧。像素本身在分段文件里。
CREATE TABLE screenshots (
  id                      INTEGER PRIMARY KEY,
  segment_path            TEXT    NOT NULL,   -- 相对 recordings/ 的路径
  frame_index             INTEGER NOT NULL,   -- 分段内序号，从 0 开始
  captured_at             INTEGER NOT NULL,   -- Unix 秒
  idle_seconds_at_capture INTEGER,            -- NULL 表示采样不可用
  width                   INTEGER NOT NULL,
  height                  INTEGER NOT NULL,
  redacted                INTEGER NOT NULL DEFAULT 0, -- 1 = 脱敏占位帧
  file_size               INTEGER,            -- 分段总字节均摊到每帧，见 §3.4
  is_deleted              INTEGER NOT NULL DEFAULT 0,
  UNIQUE (segment_path, frame_index)
);
CREATE INDEX idx_screenshots_captured_at ON screenshots (captured_at);

-- 批次状态机。
CREATE TABLE analysis_batches (
  id            INTEGER PRIMARY KEY,
  start_ts      INTEGER NOT NULL,
  end_ts        INTEGER NOT NULL,
  status        TEXT    NOT NULL,   -- 见下方枚举
  failure_kind  TEXT,               -- 失败时的面向用户分类
  failure_note  TEXT,               -- 已脱敏
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL
);
CREATE INDEX idx_batches_status ON analysis_batches (status);

CREATE TABLE batch_screenshots (
  batch_id      INTEGER NOT NULL REFERENCES analysis_batches(id) ON DELETE CASCADE,
  screenshot_id INTEGER NOT NULL REFERENCES screenshots(id),
  PRIMARY KEY (batch_id, screenshot_id)
);

-- 每批次的 LLM 转录输出。
CREATE TABLE observations (
  id            INTEGER PRIMARY KEY,
  batch_id      INTEGER NOT NULL REFERENCES analysis_batches(id) ON DELETE CASCADE,
  start_ts      INTEGER NOT NULL,
  end_ts        INTEGER NOT NULL,
  observation   TEXT    NOT NULL,
  metadata      TEXT,               -- JSON
  created_at    INTEGER NOT NULL
);

-- 每次真实 HTTP attempt 的脱敏元数据；不保存 endpoint、正文、图片、密钥或费用。
CREATE TABLE llm_calls (
  id                 INTEGER PRIMARY KEY,
  batch_id           INTEGER REFERENCES analysis_batches(id),
  purpose            TEXT    NOT NULL,
  attempt_no         INTEGER NOT NULL,
  provider_id        TEXT    NOT NULL,
  protocol           TEXT    NOT NULL,   -- openai | anthropic
  requested_model    TEXT    NOT NULL,
  actual_model       TEXT,
  started_at         INTEGER NOT NULL,
  finished_at        INTEGER NOT NULL,
  latency_ms         INTEGER NOT NULL,
  outcome            TEXT    NOT NULL,
  error_kind         TEXT,
  http_status        INTEGER,
  input_tokens       INTEGER,
  output_tokens      INTEGER,
  cache_read_tokens  INTEGER,
  cache_write_tokens INTEGER
);
CREATE INDEX idx_llm_calls_batch ON llm_calls (batch_id, purpose, attempt_no);
```

`analysis_batches.status` 是封闭枚举：

| 值 | 含义 |
|----|------|
| `pending` | 已创建，等待处理 |
| `processing` | 正在处理（进程异常退出后由下次启动重新拾取） |
| `succeeded` | 成功终态 |
| `failed` | 失败，可重试 |
| `failed_empty` | provider 返回空结果 |
| `skipped_short` | 跨度不足最小分析时长，不送 LLM |

**只有一个成功终态。** 不设置语义重复的第二个成功值。

### 3.3.2 时间线

```sql
CREATE TABLE timeline_cards (
  id               INTEGER PRIMARY KEY,
  batch_id         INTEGER REFERENCES analysis_batches(id),
  day              TEXT    NOT NULL,   -- 逻辑日 yyyy-MM-dd，凌晨 4 点边界
  start            TEXT    NOT NULL,   -- 时钟串，LLM 原始输出，如 "10:21 AM"
  end              TEXT    NOT NULL,
  start_ts         INTEGER NOT NULL,   -- 由时钟串派生，见 §3.5
  end_ts           INTEGER NOT NULL,
  category         TEXT    NOT NULL,   -- 分类名称字符串，见 §3.3.3
  subcategory      TEXT,
  title            TEXT    NOT NULL,
  summary          TEXT    NOT NULL,
  detailed_summary TEXT,
  video_summary_path TEXT,             -- timelapse 相对路径
  metadata         TEXT,               -- JSON：appSites、distractions、idle 诊断等
  is_deleted       INTEGER NOT NULL DEFAULT 0,
  created_at       INTEGER NOT NULL,
  updated_at       INTEGER NOT NULL
);
CREATE INDEX idx_cards_day     ON timeline_cards (day, start_ts);
CREATE INDEX idx_cards_span    ON timeline_cards (start_ts, end_ts);
```

**时间戳存两次是有意的**：`start`/`end` 是 LLM 输出的本地化时钟串，`start_ts`/`end_ts` 是
由它派生的 Unix 秒。两者必须一起返回给前端，且**任何一方都不得由前端重算**（§3.5）。

### 3.3.3 分类

分类是一等实体，不是设置里的一个数组。

```sql
CREATE TABLE categories (
  id          TEXT    PRIMARY KEY,   -- UUID
  name        TEXT    NOT NULL UNIQUE,
  color_hex   TEXT    NOT NULL,      -- "#RRGGBB"
  details     TEXT    NOT NULL DEFAULT '',  -- 提供给 LLM 的分类说明
  sort_order  INTEGER NOT NULL,
  is_system   INTEGER NOT NULL DEFAULT 0,   -- 内置，不可删除
  is_idle     INTEGER NOT NULL DEFAULT 0,   -- 该分类计入空闲而非跟踪时长
  created_at  INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL
);
```

`timeline_cards.category` 存的是**名称字符串**而不是外键，因为它同时是给 LLM 的输出约束
（模型被要求从名称列表里选一个）。代价是重命名分类必须同步改写已有卡片——这是一次事务，
在 `SaveCategories` 内完成，不能留给前端。

内置分类：`System`（系统事件，合计时排除）、`Idle`（空闲，`is_idle = 1`）。

### 3.3.4 洞察与用户输入

```sql
CREATE TABLE daily_standup_entries (
  standup_day  TEXT PRIMARY KEY,   -- 日历日 yyyy-MM-dd
  payload      TEXT NOT NULL,      -- JSON：highlights / tasks / blockers
  generated_at INTEGER
);

CREATE TABLE journal_entries (
  day          TEXT PRIMARY KEY,   -- 逻辑日
  intentions   TEXT,
  notes        TEXT,
  goals        TEXT,
  reflections  TEXT,
  summary      TEXT,               -- AI 生成，用户只读
  status       TEXT NOT NULL,      -- draft | intentions_set | complete
  updated_at   INTEGER NOT NULL
);

CREATE TABLE day_goals (
  day                       TEXT PRIMARY KEY,
  focus_target_minutes      INTEGER NOT NULL,
  distraction_limit_minutes INTEGER NOT NULL,
  is_skipped                INTEGER NOT NULL DEFAULT 0,
  updated_at                INTEGER NOT NULL
);

CREATE TABLE day_goal_categories (
  day         TEXT NOT NULL REFERENCES day_goals(day) ON DELETE CASCADE,
  category_id TEXT NOT NULL REFERENCES categories(id),
  role        TEXT NOT NULL,   -- focus | distraction
  sort_order  INTEGER NOT NULL,
  PRIMARY KEY (day, category_id, role)
);

CREATE TABLE timeline_review_ratings (
  id        INTEGER PRIMARY KEY,
  start_ts  INTEGER NOT NULL,
  end_ts    INTEGER NOT NULL,
  rating    INTEGER NOT NULL,
  note      TEXT,
  created_at INTEGER NOT NULL
);
```

Chat 相关表（`chat_conversations`、`chat_messages`）随 Chat 功能一起落盘，v1 不建。

### 3.3.5 设置与 Provider

```sql
-- 类型化访问在 internal/settings，这里只是键值存储。
CREATE TABLE app_settings (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL,     -- JSON 标量或对象
  updated_at INTEGER NOT NULL
);

-- 用户自定义 provider。密钥不在此表。
CREATE TABLE providers (
  id           TEXT PRIMARY KEY,   -- 生成的不透明标识
  display_name TEXT NOT NULL,
  protocol     TEXT NOT NULL,      -- openai | anthropic
  endpoint     TEXT NOT NULL,      -- 绝对 http(s) 基地址，不含凭据
  model        TEXT NOT NULL,
  sort_order   INTEGER NOT NULL,
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL
);
```

`app_settings` 的键空间与 `SettingsDTO` 的分组一一对应
（[05 §5.5.2](05-interface-contract.md#552-dto-目录)）：

| 键 | 类型 | 默认 |
|----|------|------|
| `capture.intervalSeconds` | int | `10` |
| `capture.heightPixels` | int | `1080` |
| `privacy.blockedApplicationIds` | string[] | `[]` |
| `storage.recordingsLimitBytes` | int64 | `0`（不限） |
| `notifications.journalReminderEnabled` | bool | `false` |
| `notifications.journalReminderTime` | string `HH:mm` | `"18:00"` |
| `appearance.theme` | string | `"system"` |
| `appearance.language` | string（BCP 47，空串=跟随系统） | `"zh-CN"` |
| `system.launchAtLogin` | bool | `false` |
| `system.showDockIcon` | bool | `true` |
| `system.agentEditsEnabled` | bool | `false` |
| `telemetry.analyticsOptIn` | bool | `false` |
| `telemetry.crashReportingOptIn` | bool | `false` |
| `providers.routing` | `{primary, secondary}` | 空 |
| `llm.outputLanguage` | string（空串=跟随界面语言） | `""` |

`llm.outputLanguage` 与 `appearance.language` 是**两个独立设置**：前者决定模型生成的卡片
标题与摘要用什么语言，后者只影响界面文案。不得复用同一个字段。

## 3.4 帧与分段

像素不逐帧存图片，而是写入**分段文件**，按 `(segment_path, frame_index)` 寻址。
容器与编码格式**待定设计**，但必须满足：

| # | 要求 | 原因 |
|---|------|------|
| 1 | 支持按帧序号随机访问 | 帧条 UI 要跳着取帧 |
| 2 | 第 N 帧的呈现时间为第 N 秒（1 fps 恒定） | 让"帧序号 ↔ 时间"是纯算术，不依赖容器时间轴解析 |
| 3 | 分段在尺寸变化、达到帧数上限或时长上限时轮换 | 单个分段不会无限增长 |
| 4 | 未收尾的分段可被检测并安全丢弃或修复 | 崩溃、断电、强制退出都会留下半个文件 |
| 5 | 清理以**整个分段**为单位 | 无法单独删除流中的一帧 |

三条不变量：

1. **绝不删除活跃分段。**
2. `screenshots.file_size` 是分段总大小**均摊**到该分段每一行的值。要得到实际磁盘占用，
   必须逐行求和；把某一行的值当成单帧大小是错的。
3. 启动时必须执行一次对账，处理三种故障：行在文件缺失（软删除行）、文件在但不可读
   （删文件 + 软删除行）、文件可读但 `file_size` 为 NULL（回填）。

## 3.5 时钟串派生

这是最容易造成**无声数据错误**的地方。

LLM 以本地化时钟串输出时间（`"10:21 AM"`）。派生 Unix 时间戳的规则：

```text
anchor = from + (to - from) / 2                 // 改写窗口的中点

resolveClock(h, m):
    candidates = [anchor 当日 - 1, anchor 当日, anchor 当日 + 1] 各自的 h:m
    return 使 |candidate - anchor| 最小的 candidate

startTs = resolveClock(startHour, startMinute)
endTs   = resolveClock(endHour,   endMinute)
if endTs < startTs: endTs += 24h                // 跨午夜
day     = 由 startTs 按凌晨 4 点边界得出
```

四个各自独立的要点，缺一不可：

1. **在前后共三天中选最近的候选。** 临近午夜时把 `"11:50 PM"` 直接解析到 anchor 当日
   是错的；±1 天候选修正这一点。
2. **`end < start` 表示跨午夜**，加一天。
3. **`day` 用凌晨 4 点边界算**，不是日历日期。
4. 全过程依赖宿主时区，必须在 DST 切换和非整点偏移时区中验证——这是基于属性的测试的
   首要候选（[08 §8.3](08-testing-strategy.md#83-行为测试)）。

改写窗口的重叠谓词：

```sql
WHERE ((start_ts < :to AND end_ts > :from) OR (start_ts >= :from AND start_ts < :to))
  AND is_deleted = 0
  AND (category != 'System' OR batch_id = :batchId)
```

最后一行保留**其它批次**写入的 `System` 卡片：相邻批次改写自己的范围时，失败标记仍然可见。

**解析失败不得静默丢弃。** `ReplaceCardsInRange` 返回 `ReplaceResult.SkippedCards`，
调用方必须消费并计入诊断指标（[05 §5.5.2](05-interface-contract.md#552-dto-目录) 的
`DiagnosticsDTO.SkippedCardsToday`）。

## 3.6 维护任务

| 任务 | 周期 | 说明 |
|------|------|------|
| WAL checkpoint | 300 秒 | |
| 数据库备份 | 启动后 1 小时，之后每 24 小时 | 保留最近 N 份，N 待定 |
| 录制清理 | 启动后 1 小时，之后每小时 | 超出 `storage.recordingsLimitBytes` 时按分段从旧到新删除 |
| `llm_calls` 元数据留存 | 待定 | 只含 attempt 元数据，不含正文 |

清理规则：**从不删除活跃分段**；删除分段的同时软删除其 `screenshots` 行；对应卡片保留
（用户仍能看到那段时间做了什么，只是没有帧可看）。
