# 03 数据模型

> **状态：设计，已开始落盘。** 本文定义 Daygo 自有的持久化结构。
> **当前数据库（`PRAGMA user_version = 17`）有十七张表**：`app_settings`（v1）、
> cards 能力的 `analysis_batches`、`timeline_cards`、`categories`（v2，含 `System` / `Idle`
> 内置种子）、`pending_captures`、`screenshots`（v3）、`providers` 与 chat 的
> `chat_conversations`、`chat_messages`（v4）、daily 的 `journal_entries`、`day_goals`、
> `day_goal_categories`（v5）、`llm_calls`（v6）、`chat_conversations.model` 会话模型
> 覆盖列（v7）、分析流水线的 `batch_screenshots`、`observations`（v8，含
> `idx_batch_screenshots_screenshot`）；`analysis_batches.attempts`（v9）、批次软删除列（v10）、
> `providers.max_images`（v11）、首次启动分类种子（v12）、`daily_standup_entries`（v13）、
> `card_reviews`（v14）、`pending_captures.frame_index`（v15）、分段截图大小均摊（v16），
> 以及 `providers.model` → `providers.models` JSON 数组（v17，单供应商多模型）。本文其余表
> 都是目标结构，由对应功能模块随需求沿同一条迁移链逐版本追加。
> 实现与本文冲突时以代码为准，并在同一 commit 修正本文。

功能模块按需求增量落盘表与 repository，全部位于 internal/storage。
[data](modules/data.md) 负责唯一连接、迁移机制 / 编号、锁与可观测封装；功能负责业务表和查询，
[09 §9.7](09-roadmap.md#97-需求接口与测试归属) 跟踪归属。
迁移仍从 user_version=1 开始逐版本验收，不因开发模块并行而出现分叉迁移链。
app_settings repository 归 data，类型化访问归 preferences；没有第二套设置数据库。

## 3.1 磁盘布局

```text
~/Library/Application Support/Daygo/
├── daygo.sqlite (+ -wal, -shm)   业务数据库                        ★
├── daygo.sqlite.lock             写入锁（flock / LockFileEx）       ★
├── capture.lock                  捕获所有者锁，与写入锁相互独立      ★
├── recordings/staging/           Capture 原子 JPEG，提交分段后删除
├── recordings/segments/          已关闭的不可变分段
├── timelapses/<yyyy-MM-dd>/      每张卡片的时间压缩视频
├── backups/daygo-<UTC>-<seq>.db  每日数据库副本，保留 7 份           ★
└── agent.sock                    外部写入通道（推迟到 v1.1）
```

★ 已落盘。目录本身以 `0700` 创建。两把锁是独立文件而不是库内的行，这样只读实例
（没有可写描述符）也能观察争用，且写入锁与捕获所有者锁可以由不同进程持有；
候选方案与回退见 [data 实例锁](decisions/data-locking.md)。

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
连接层固定 `journal_mode=WAL`、`synchronous=NORMAL`、`busy_timeout=5000`、`foreign_keys=1`，
并在打开后**回读校验**这四项确实生效（DB-6）——只检查“没报错”发现不了被静默忽略的 PRAGMA。
`foreign_keys` 自 v4 起必须开启：`chat_messages` 的外键与级联删除依赖它，SQLite 默认
按连接关闭。

迁移链的三条纪律，违反其中任何一条都会让用户库和代码分叉：

1. **只追加，不修改。** 已发布的版本永远不能改：用户库已经停在那个版本，改动对他们不生效。
2. **每个版本配一个“旧库 → 新库”夹具**，与该版本同一个 commit 合入。证明迁移保数据的唯一
   方式，是拿上一版本写出的真实文件跑一遍（DB-2）。
3. **DDL 与 `user_version` 在同一个事务里提交**，因此崩溃后重跑是幂等的（DB-1）。

版本号比本 build 支持的更高时**拒绝打开**而不是尝试兼容；只读实例根本不跑迁移。

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
  attempts      INTEGER NOT NULL DEFAULT 0,  -- 进入失败状态的次数（v9）
  is_deleted    INTEGER NOT NULL DEFAULT 0,  -- 用户忽略的失败批次（v10，软删除）
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
CREATE INDEX idx_observations_batch ON observations (batch_id, start_ts);
CREATE INDEX idx_observations_span ON observations (start_ts, end_ts);

-- 每次真实 HTTP attempt 的脱敏元数据；不保存 endpoint、正文、图片、密钥或费用。
-- （v6 已落盘，analysis 与 chat 均通过 attempt observer 写入。）
CREATE TABLE llm_calls (
  id                 INTEGER PRIMARY KEY,
  batch_id           INTEGER REFERENCES analysis_batches(id),
  purpose            TEXT    NOT NULL,
  attempt_no         INTEGER NOT NULL,
  provider_id        TEXT    NOT NULL,
  protocol           TEXT    NOT NULL,   -- openai | openai_responses | anthropic
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
| `failed` | 失败，冷却后可重试，`attempts` 达到上限后不再入队 |
| `failed_empty` | provider 返回空结果 |
| `skipped_short` | 跨度不足最小分析时长，不送 LLM；正常终态，不进失败面板 |

**只有一个成功终态。** 不设置语义重复的第二个成功值。
`attempts` 在每次进入 `failed` / `failed_empty` 时自增；达到 `MaxBatchAttempts`（5）后
`RequeueFailed` 拒绝重新入队——确定性失败（帧文件丢失、时钟串不可解析）不应在冷却时钟上
无限重复消耗 LLM 调用。手动 `RetryBatches` 绑定会显式重置该计数。

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
  summary          TEXT    NOT NULL,      -- 一句话：应用/站点与整体活动，上限 135 字符
                                         -- （generateCards 内强制截断）
  detailed_summary TEXT,                 -- 分段时间日志：每段 "h:mm PM–h:mm PM: 描述"，
                                         -- 上限 15 段 / 2500 字符（generateCards 内强制截断）
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

分类的 `name` / `details` 种子文案是英文：它们是模型匹配用的数据，不是界面文案。界面只对
**仍保持种子原文**的内置分类显示本地化文案（`frontend/src/lib/categoryLabel.ts` 的名称表与
描述表，描述表由 `frontend/tests/categoryDefaults.test.ts` 与迁移种子对账）；用户改写过的行
按原样显示。分类管理向导的编辑框预填的也是这份本地化文案，因此"编辑并保存"等同于一次
重命名：该分类从种子文案变成用户自己的文案，历史卡片随之改写。

内置分类：`System`（系统事件，合计时排除）、`Idle`（空闲，`is_idle = 1`）。

### 3.3.4 洞察与用户输入

```sql
-- daily_standup_entries（v13 已落盘）保存已生成的日报；
-- LLM 生成与调度尚未实现，不应与存储能力混为一谈。
CREATE TABLE daily_standup_entries (
  standup_day      TEXT PRIMARY KEY,   -- 日历日 yyyy-MM-dd
  highlights_title TEXT NOT NULL,
  highlights       TEXT NOT NULL,      -- JSON 数组
  tasks_title      TEXT NOT NULL,
  tasks            TEXT NOT NULL,      -- JSON 数组
  blockers_title   TEXT NOT NULL,
  blockers_body    TEXT NOT NULL,
  generated_at     INTEGER NOT NULL
);

-- journal_entries（v5 落盘；v19 移除 summary 列）。日报站会即当日的 AI 摘要，
-- 日记不再单独保存 AI summary。
CREATE TABLE journal_entries (
  day          TEXT PRIMARY KEY,   -- 逻辑日
  intentions   TEXT,
  notes        TEXT,
  goals        TEXT,
  reflections  TEXT,
  status       TEXT NOT NULL,      -- draft | intentions_set | complete
  updated_at   INTEGER NOT NULL
);

-- day_goals / day_goal_categories（v5 已落盘）。保存时分类引用整体替换。
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

-- card_reviews（v14 已落盘）。卡片审阅流的判定：每卡一行，重判覆盖，撤销删除行。
-- day 与 minutes 在判定时从卡片快照，卡片之后被编辑也不会改变当日统计；
-- verdict 只进统计，从不改写卡片自己的分类。软删除的卡片经 join 退出统计。
CREATE TABLE card_reviews (
  card_id    INTEGER PRIMARY KEY REFERENCES timeline_cards(id) ON DELETE CASCADE,
  day        TEXT    NOT NULL,
  verdict    TEXT    NOT NULL CHECK (verdict IN ('distraction', 'neutral', 'focus')),
  minutes    INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE INDEX idx_card_reviews_day ON card_reviews (day);

-- card_ratings（v18 已落盘）。详情页对卡片 AI 摘要的拇指评分：每卡一行，重评覆盖，
-- 再次点击已激活的拇指删除行。只评价摘要文本，不改写摘要或卡片分类。
-- 独立成表而非并入 card_reviews：那里的 verdict 是 NOT NULL，只评分未判定的行无处存。
CREATE TABLE card_ratings (
  card_id    INTEGER PRIMARY KEY REFERENCES timeline_cards(id) ON DELETE CASCADE,
  rating     TEXT    NOT NULL CHECK (rating IN ('up', 'down')),
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
```

```sql
-- Chat 会话与消息（v4 已落盘）。多会话模型已决定（decisions/chat-session-model.md）；
-- tool_name / tool_arguments 随 agent 工具循环切片落盘（v6 加列）：tool_call 行两者皆填，
-- tool_result 行按 tool_name 配对、tool_arguments 为空。
CREATE TABLE chat_conversations (
  id          TEXT PRIMARY KEY,   -- UUID
  title       TEXT,
  provider_id TEXT,               -- 该会话显式选择的供应商；NULL = 尚未选择
  model       TEXT NOT NULL DEFAULT '',  -- v7：该会话的模型覆盖；'' = 跟随 provider 配置的模型
  created_at  INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL
);

CREATE TABLE chat_messages (
  id              INTEGER PRIMARY KEY,
  conversation_id TEXT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
  role            TEXT NOT NULL,  -- user | assistant | tool_call | tool_result
  content         TEXT NOT NULL,  -- tool_result 行为结果 JSON 信封
  status          TEXT,           -- assistant 消息：ok | failed | canceled
  tool_name       TEXT,           -- tool_call / tool_result 行的工具名
  tool_arguments  TEXT,           -- 仅 tool_call：紧凑 JSON 参数
  created_at      INTEGER NOT NULL
);
CREATE INDEX idx_chat_messages_conversation ON chat_messages (conversation_id, id);
```

### 3.3.5 设置与 Provider

```sql
-- 类型化访问在 internal/settings，这里只是键值存储。
CREATE TABLE app_settings (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL,     -- JSON 标量或对象
  updated_at INTEGER NOT NULL
);

-- 用户自定义 provider（v4 已落盘）。密钥不在此表。无 sort_order：展示顺序按
-- display_name，路由顺序由 providers.routing 链表达。
CREATE TABLE providers (
  id           TEXT PRIMARY KEY,   -- 生成的不透明标识
  display_name TEXT NOT NULL,
  protocol     TEXT NOT NULL,      -- openai | openai_responses | anthropic
  endpoint     TEXT NOT NULL,      -- 绝对 http(s) 基地址，不含凭据
  models       TEXT NOT NULL DEFAULT '[]',  -- v17：JSON 字符串数组，同一 endpoint/key 下的有序模型列表（至少 1 个，上限 20）
  max_images   INTEGER NOT NULL DEFAULT 0,   -- v11：单请求图片上限，0 = ai.MaxImages 默认；与模型无关，按 provider 计
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL
);
```

v17 把单列 `model` 重建为 `models`（JSON 字符串数组）：旧 `model` 折为一元数组
`["<model>"]`，空 `model` 折为 `[]`（建新表→回填→改名，见
decisions/providers-multi-model.md）。模型无独立身份，只是附在 provider 上的有序串列表，
不建子表。`max_images` 保持 per-provider（与具体模型无关）。

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
| `system.testToolsEnabled` | bool | `false` |
| `telemetry.analyticsOptIn` | bool | `false` |
| `telemetry.crashReportingOptIn` | bool | `false` |
| `providers.routing` | `{"chain": [{"providerId","model"}, …]}`（有序，`chain[0]` 为主，按对去重，上限 8；`model` 为空跟随该 provider 的首个模型） | `{"chain":[]}` |
| `llm.outputLanguage` | string（空串=跟随界面语言） | `""` |
| `llm.recognitionEnhancementEnabled` | bool | `false` |
| `chat.memory` | string（全局聊天记忆，自由文本） | `""` |
| `chat.editMode` | string（`readonly` \| `edits`） | `"readonly"` |

`providers.routing` 的链条目是**「供应商 + 模型」对** `{providerId, model}`：同一 provider
（同 endpoint、同 key）的不同模型可分别排进链、独立排序与回退，分析流水线按具体模型走
（decisions/providers-multi-model.md）。两种旧形状在**读取时**折叠，均无需 SQL 迁移：
`{"primary","secondary"}` → `[{primary,""}, {secondary,""}?]`；裸字符串数组 `["id", …]` →
`[{id,""}, …]`。空 `model` 解析为该 provider 的首个模型，与迁移后 `models=[旧model]` 行为等价。
会话级 provider 选择不在设置里：它存在 `chat_conversations.provider_id` 列
（decisions/chat-session-model）。

`chat.memory` 是用户自定义的全局聊天指令（类似 CLAUDE.md），非空时注入每个会话的系统
提示尾部；`chat.editMode` 是 chat 沙箱门禁
（[05 §5.12](05-interface-contract.md#512-chat应用内对话式-agent)），
两键均已随各自切片落盘。

`llm.outputLanguage` 与 `appearance.language` 是**两个独立设置**：前者决定模型生成的卡片
标题与摘要用什么语言，后者只影响界面文案。不得复用同一个字段。

`llm.recognitionEnhancementEnabled` 开启后，识别用途的每张图片在发送前于内存中切为
四张带交叉覆盖的分片，四片之后附上未改动的原图一并发送（见
[04 §4.3.4](04-data-flow.md#434-提示词与输出解析)），分片与原图均不额外落盘；默认关闭，
因为开启会提高 token 用量。

## 3.4 帧与分段

像素**不进入 SQLite BLOB**。`Capture` 输出的逐帧 JPEG 只在 `recordings/staging/` 短期存在；
积累到轮换边界后由 `Media` 批量构建不可变分段，成功提交后删除 staging。只有已完整发布并完成
结构化提交的帧才写入 `screenshots`，按 `(segment_path, frame_index)` 寻址。完整状态机与崩溃
窗口见[图片存储决策](decisions/recording-image-storage.md)。
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
3. 启动时必须执行一次对账：恢复已登记的 staging/pending；探测已发布但尚未结构化提交的
   building segment；完成中断的 deleting；陌生文件只隔离和诊断，绝不自动信任或发送。

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

接受的时钟串形态：契约格式 `"h:mm a"`（`"10:21 AM"`，大小写不敏感、句点可选）、无空格的
粘着形式（`"10:21AM"` / `"10:21pm"`——模型常见的偏差，拒绝它会让整批卡永久失败）、以及
裸 24 小时制 `"H:mm"`。其他形态是解析错误，调用方必须当作 skipped card 上报，不得静默丢弃。

改写窗口的重叠谓词：

```sql
WHERE ((start_ts < :to AND end_ts > :from) OR (start_ts >= :from AND start_ts < :to))
  AND is_deleted = 0
```

范围内**所有**存活卡片都在改写中吸收，`System` 回退卡（模型输出了未知分类名）也不例外：
融合把改写范围扩展到被融合卡片的 start 时，那张卡必须一并消失，否则两张卡并列占住同一时段。
失败状态由 `analysis_batches` 承载（失败面板读它），不落在卡片上。

删除整张重叠卡之前，改写所有权必须覆盖该卡完整的 `[start_ts, end_ts)`。卡片若只与改写窗口
部分重叠，而生成结果没有把所有权扩到它的完整起止，事务必须以约束错误回滚，保留原卡；不得
整张软删除后只重建交集，造成窗口外前缀或后缀无声消失。分析重处理遇到横跨批次起点的旧卡时，
必须以旧卡 start 作为生成、校验和 `ReplaceCardsInRange` 共同的 `rewriteStart`。

融合本身受确定性闸门约束（[04 §4.3.4](04-data-flow.md#434-提示词与输出解析)）：
只有当将被吸收的前卡分类全部与输出卡一致时，融合才把改写范围扩展到前卡 start；
分类不一致时前卡不属于融合对象，改写范围保持批次窗口，输出卡 start 被夹紧回窗口起点。
**横跨批次起点的那张前卡例外**：批次的证据本身就与它重叠，只替换交集会删掉它的前缀，
因此它的 start 无条件成为 `rewriteStart`。闸门算出的 `ownedFrom` 同时是校正提示、
`validateCards` 与 `ReplaceCardsInRange` 的左边界——三者不一致会让被拒的融合在三次校正后
仍以整批失败告终。

**解析失败不得静默丢弃。** `ReplaceCardsInRange` 返回 `ReplaceResult.SkippedCards`，
调用方必须消费并计入诊断指标（[05 §5.5.2](05-interface-contract.md#552-dto-目录) 的
`DiagnosticsDTO.SkippedCardsToday`）。

## 3.6 维护任务

| 任务 | 周期 | 状态 | 说明 |
|------|------|------|------|
| WAL checkpoint | 300 秒 | ★ 已实现 | `PASSIVE`：不阻塞读写，宁可 WAL 大一会儿也不要卡住一次捕获写入 |
| 数据库备份 | 启动后 1 小时，之后每 24 小时 | ★ 已实现 | `VACUUM INTO`（不是文件复制，避免撕裂的 WAL），保留最近 **7** 份（[决策](decisions/data-backup-retention.md)） |
| 录制清理 | 启动 1 小时，之后每小时 | ★ 已实现（分段粒度） | 清理以完整分段为单位（按 segment_path 聚合软删除 screenshots 行后物理删除段文件），保留未收尾 pending 段与活跃批次租用段，时间线卡片保留。见[HEVC分段落盘决策](decisions/recording-frame-segments-hevc.md)与[图片存储决策](decisions/recording-image-storage.md#7-清理流程) |
| `llm_calls` 元数据留存 | 待定 | 写入已实现（analysis 与 chat）；清理未实现 | 只含 attempt 元数据，不含正文 |

维护循环由 app 生命周期持有（`storage.Maintainer`），`ctx` 取消即退出，不存在全局单例。
只读实例照常跑循环，它的写操作被存储层拒绝——第二个实例是预期状态，不是故障。

清理规则：**从不删除 staging、building、活跃或被分析租用的分段**；先记录 `deleting` 意图，
事务外删除完整文件，再软删除其 `screenshots` 行并完成状态；对应卡片保留
（用户仍能看到那段时间做了什么，只是没有帧可看）。
