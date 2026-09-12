# data — 数据管理与诊断

## 用户结果与范围

用户能限制录制占用、观察自动清理与故障诊断，并控制默认关闭的遥测。
后台数据库有迁移、备份与恢复机制；只读实例不会写数据或重复捕获。
负责 U9、F-S6/9 的设置与数据处理、DB-1–9、IT-12/13。
导出 / 批量删除及留存上限仍待产品决定；不自动增加入口。
崩溃上报实际分发接入由 delivery 协作，隐私名单 UI 与捕获双保护归 recording。

依据：[03](../03-data-model.md)、[05 storage](../05-interface-contract.md#562-storage)、
[图片存储流水线](../decisions/recording-image-storage.md)、
[07](../07-privacy-security.md)、[08 DB 测试](../08-testing-strategy.md#84-数据与迁移测试)。

## 当前状态与证据

实现进度：**部分实现**。切片 1、2、4 已落盘，切片 3 的 checkpoint / 备份 / 损坏恢复已完成、
**清理未开始**，切片 5 长期观察未开始。

已交付能力：

- **db-core**：`internal/storage` 的连接、PRAGMA、迁移链、可观测读写封装、只读降级与实例锁
  （POSIX `flock` / Windows `LockFileEx`）；
  `app_settings` 表由 v1 迁移创建。
- **settings-store**：`app_settings` 的类型化 repository（`SettingsRepo`），含往返、单事务批量写、
  错误路径与 `Watch` 变更通知（ctx 结束关闭）。规范化与夹取按 `05 §5.6.3` 留给
  `internal/settings`，本层不重复实现。
- **绑定接入**：`internal/app` 持有真实 `*storage.Store`；`canWrite` / `isCaptureOwner`
  来自实例锁而非构造参数，`GetCapabilities` 只在真正打开数据库时报告 `storage` 能力。
- **diagnostics**：`Store.Stats` 与 `GetDiagnostics`；无数据源的字段经 `DTO.Unavailable`
  说明原因，不返回会读作"没有活动"的裸零。storage → apperr 的错误映射集中在
  `mapStorageError`（`05 §5.6.1` 要求单点）。
- **维护**：`Checkpoint`（WAL，300 秒）、`Backup`（`VACUUM INTO`，每日，保留 7 份）、
  `Backup` 轮换、`IntegrityCheck`，以及由 app 生命周期持有的 `Maintainer` goroutine
  （ctx 取消即退出，无全局单例）。
- **损坏恢复（DB-7）**：`Open` 在写入实例上遇到归类为损坏的连接失败时，自动还原 `backups/`
  中最新一份并重试一次；无备份、环境故障、只读实例三种情形都不碰文件。原库改名为
  `.replaced`（重名时追加序号，不覆盖），恢复来源经 `RecoveredFrom()` 与
  `DiagnosticsDTO.RecoveredFromBackup` 暴露。流程见
  [decisions/data-corruption-recovery.md](../decisions/data-corruption-recovery.md)。

录制清理仍未开始；原因见下。

**跨模块边界已由各模块自行补齐**：诊断原先依赖 recording 的 `screenshots` 与 timeline 的
`analysis_batches`，两张表当时都不存在，data 按「禁止一次性建设未使用的全部目标表」选择了
不代建、如实报告为不可用。现在 recording 与 timeline 已各自把表提交进同一条迁移链
（v2 建 `timeline_cards` / `categories` / `analysis_batches`，v3 建 `pending_captures` /
`screenshots`），因此：`recordingsBytes`、`lastCaptureAtTs`、`pendingBatches`、`failedBatches`
改为查询真实数据源；`lastCaptureAtTs` 用指针表达「尚无已提交帧」，不把缺失压成零。

诊断的「来源不存在」分支仍然保留，但已无法由正常迁移链触达，因此其测试改为显式删除表来构造
（`internal/storage/db_gate_test.go`）。**录制清理仍被阻塞**，两个前置都归 recording，
详见「能力与跨层职责」。

已接入前端的低风险切片：设置页“存储与诊断”通过 `GetSettings` / `GetDiagnostics` 显示数据库状态、原生服务状态、捕获所有者和真实可用性；录制占用上限可持久化写入 `app_settings`。页面在数据源不可用时显示“尚未接入”，不把零误报为没有录制数据。**上限本身还没有消费者**：修改它只写库，不会删除任何文件——清理逻辑尚未实现（见下）。

落盘代码：`internal/storage/{doc,errors,observe,store,open,pragma,migrate,recover,settings,cards,categories,captures,diagnostics,maintenance,maintain,lock_unix,lock_windows}.go`，匿名夹具与生成器在 `internal/storage/testdata/`；前端接入位于 `frontend/src/views/Settings/StorageSection.vue` 与 `frontend/src/api/diagnostics.ts`。

诊断现在已有真实设置页消费者；不可用来源会在 UI 中显式显示，生产构建不会加载开发夹具。

实现与验证状态分别记录；下方“验证记录”只登记真实运行过的命令与结果。


## 能力与跨层职责

本模块先后可独立交付 db-core、settings-store、diagnostics 和维护能力；
这些是切片，不要求一次完成 data 才解锁其他功能。

**已可供消费者接入**（`09 §9.3`）：db-core、settings-store、diagnostics。
维护的 checkpoint、备份与损坏恢复已可用；**录制清理未实现，且有两个前置都归 recording**：

1. **`recording_segments` 表不存在**。清理按分段而非单帧工作，需要枚举 `closed` 分段、
   排除 building 段与被分析租用的段（见 [图片存储决策 §7](../decisions/recording-image-storage.md#7-清理流程)）。
   该表**由 recording 的迁移夹具定义**，决策明确要求「不要在没有恢复测试时先冻结 schema」，
   因此 data 不代它建表。
2. **`Media` 无任何实现**（连 fake 都没有）。删除整个分段文件、探测分段帧数都经它。

`pending_captures` 与 `screenshots` 已由 recording 的 v3 迁移创建，不再是阻塞项。

| 输入 | 可独立推进 | 真实接入条件 |
|---|---|---|
| 03 schema 与 05 repository 契约 | 匿名新库、旧库、损坏库与并发 fixture | db-core 独立门禁；只用隔离测试目录 |
| recording: 活跃 / 收尾分段、Media.ProbeSegment | 假分段、故障与字节均摊 fixture | 真实生命周期和 media-read 接入后验证清理 / 恢复 |
| 各模块诊断数据 | 匿名计数 / 耗时、错误码与 DTO fixture | 计数从真实行为产生，禁止敏感活动信息 |
| preferences: settings-access / ui-bridge | 磁盘上限 / 遥测配置 fixture | settings-store 已就绪；待 preferences 接入类型化访问与生成绑定 |



internal/storage 是唯一 SQL、连接、schema 和迁移 owner；data 维护统一读写可观测封装、
锁及迁移编号。其他功能提交自身表与 repository 到此包，按实际需求新增版本，
禁止一次性建设未使用的全部目标表。data 交付 app_settings repository；
preferences 在其上做类型化访问，不新开数据库。
Go 负责清理决策、备份与诊断；像素读取经 Media，适配层不写 SQLite。
维护 goroutine 由 app 生命周期管理，退出可取消，不引入全局数据库单例。

## 实验与失败条件

| 实验 | 输入与操作 | 预期结果 | 失败条件 / 证据 |
|---|---|---|---|
| DB-1/2/3/4/6 | 空库、旧版本与边界库；迁移两次、往返、回读 PRAGMA | 迁移幂等、逐版本保留数据、完整性 ok、固定 PRAGMA | 修改真实数据、遗漏版本夹具或 schema 漂移失败 |
| DB-7 | 截断库、只读目录、磁盘满；尝试打开 / 恢复 | 区分损坏与环境故障；后两者只告警不删文件 | 将所有失败当损坏、删除原文件失败 |
| DB-8 / IT-13 | 一个 writer 与只读实例并发 1 小时；直接尝试只读连接写入 | 唯一 writer / capture owner、连接层拒写、无忙锁风暴 / 损坏 | 调用层自律代替只读、两个捕获者失败 |
| DB-9 / IT-12 | 小上限、已关闭与活跃分段、解码失败、恢复重放 | 以整段清理，从不删活跃段，file_size 总和正确 | 单帧删除、活跃段丢失或错误字节占用失败 |
| 诊断与备份 | 模拟慢查询、争用、解析跳过与崩溃；检查持久化 / 上报载荷 | 匿名计数可查询，默认不上传，密钥与活动内容不泄漏 | payload 泄漏或错误静默失败 |

## 实现切片与集成

1. 先落隔离 DB / 锁夹具，再实现纯 Go 连接、PRAGMA、迁移与可观测封装、只读降级；
   db-core 门禁通过即解锁功能持久化，不等 data UI 或录制完成。
2. 实现 settings-store 的往返 / 事务契约，提供给 preferences；
   其他模块的业务表逐项走同一迁移链，data 协调迁移合入顺序。
3. 在匿名分段 fixture 上实现 checkpoint、备份恢复与清理；真实 media-read 就绪后跑 IT-12。
   retention 决策落盘后再启用相应策略，禁止删除活跃分段。
   **checkpoint、备份、轮换、损坏恢复已完成**；清理未开始，前置见「能力与跨层职责」。
4. 增加 GetDiagnostics 与匿名指标、磁盘和遥测设置、store 与 UI；当前已接入诊断 UI 和磁盘上限设置，
   遥测写入仍待实际 telemetry 消费者，**磁盘上限也还没有消费者**（清理未实现）。
5. 累计 14 天磁盘 / 内存观察，与录制 / 更新共同验证关停和恢复；长期状态单列。

## 验收、阻塞与回退

完成要求：所交付 schema 的 DB-1–9、IT-12/13、真实维护 / 诊断用户闭环通过，
opt-in 和隐私载荷符合 07；已完成 db-core 可提前被接入。
纯 Go WAL 实验失败时记录 G-core 阻塞并重新决策，不自动引入 cgo。
real Media 未就绪仅阻塞真实清理验收，不阻塞连接、迁移和设置 repository。

待决：备份保留份数由 data 工程在维护实现前决定；导出 / 批量删除和留存上限由 data 产品
在相关入口 / 策略实现前决定。默认值沿用公共规范，不在此预选。
回退：变更前用匿名夹具验证备份恢复；实际 schema 升级不能仅靠 git revert 降级。
故障先停止写入、保留原库与备份，按已验证恢复步骤处理；不得拿用户库测试破坏性迁移。

## 验证记录

| 日期 / commit / 环境 | 命令或人工步骤 / 输入 | 期望与实际结果 | 限制 / 下一步 |
|---|---|---|---|
| 2026-09-11 / 见本次提交 / macOS arm64 · go1.26.3 · `CGO_ENABLED=0` | `go test ./internal/storage/`、`-race`、`go build ./...`、`go vet ./...`、`gofmt -l .` | 全部通过；DB-1/2/4/6/7/8(smoke)/IT-13 在已实现范围通过 | 非 Linux 实机；`internal/app` 需 `frontend/dist` 才能编译 |
| 2026-09-11 / 同上 | `go test -tags long -run TestConcurrentReaderWriterOneHour -timeout 25s` | 25 秒后被超时中断，无死锁、无 busy 报错、无损坏 | **仅为逻辑验证，不是 DB-8 通过**；1 小时全量未运行 |
| 2026-09-11 / 同上 | `go test -count=1 -race ./internal/app/` | 通过；绑定层所有权来自真实锁、诊断映射与维护路径均有断言 | 未在真实 Wails 宿主中运行；`GetDiagnostics` 无 UI |
| 2026-09-11 / 同上 | `go test -count=1 ./internal/storage/`（settings / 维护 / 诊断用例） | 通过；settings 往返与重启读回、单事务原子性、Watch 交付与关闭、备份可读且轮换、恢复保留原库、并发备份互不碰撞 | 未接真实用户设置；清理未接线 |
| 2026-09-11 / 当前工作树 / Windows 11 amd64 · go1.25.4 | `go test -count=1 ./internal/storage ./internal/settings`；子进程持锁、正常退出与强制终止夹具 | 通过；`LockFileEx` 对第二实例返回 `ErrLockBusy`，正常关闭和进程终止后均可重取；`Open` 只读降级、捕获所有者互斥与 `Close` 释放通过 | 仅短时 smoke；DB-8 一小时并发与录制清理未运行 |
| 2026-09-12 / 见本次提交 / macOS arm64 · go1.26.3 · `CGO_ENABLED=0` | `go test -count=1 ./internal/storage/ ./internal/app/`、`-race` | 通过；**DB-7 在已实现范围通过**：截断的库触发还原、还原后备份中的值回读一致、备份之后写入的值按预期消失、损坏原库以 `.replaced` 保留、无备份时报错且文件大小不变、只读实例不恢复、连续两次恢复各留一份副本、健康打开不报恢复 | 未在真实 Wails 宿主中触发过恢复；诊断界面尚未渲染 `recoveredFromBackup` |

**测试发现的一个真实缺陷**：并发调用 `Store.Backup` 时，先前基于秒级时间的文件名会让两次
备份取到同名，`VACUUM INTO` 拒绝覆盖导致双双失败。现改为在互斥区内使用单调序号命名，
并发备份用例覆盖此路径。

DB-3 已运行：所有只读 repository 方法在空库、v0/v1 迁移夹具与代表性数据上均返回非错误
（`internal/storage/db_gate_test.go`）。
DB-5 已运行：`timeline_cards.metadata` 可解码，`appSites` / `distractions` 往返一致。
**DB-9 与 IT-12 未运行**：两者都依赖分段生命周期与 `Media`，而 `Media.ProbeSegment` 尚无实现
（连 fake 都没有），因此无法判断分段边界与活跃段。这是 data 目前唯一的硬前置依赖。

后续记录驱动 / 系统、commit、匿名夹具、并发时长、回读 PRAGMA 与完整性结果。
