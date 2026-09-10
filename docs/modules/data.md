# data — 数据管理与诊断

## 用户结果与范围

用户能限制录制占用、观察自动清理与故障诊断，并控制默认关闭的遥测。
后台数据库有迁移、备份与恢复机制；只读实例不会写数据或重复捕获。
负责 U9、F-S6/9 的设置与数据处理、DB-1–9、IT-12/13。
导出 / 批量删除及留存上限仍待产品决定；不自动增加入口。
崩溃上报实际分发接入由 delivery 协作，隐私名单 UI 与捕获双保护归 recording。

依据：[03](../03-data-model.md)、[05 storage](../05-interface-contract.md#562-storage)、
[07](../07-privacy-security.md)、[08 DB 测试](../08-testing-strategy.md#84-数据与迁移测试)。

## 当前状态与证据

实现进度：**部分实现**。db-core 切片已落盘：`internal/storage` 的连接、PRAGMA、迁移链、
可观测读写封装、只读降级与实例锁；`app_settings` 表由 v1 迁移创建。
settings-store 的类型化 repository、维护 / 清理、备份恢复与 GetDiagnostics 均未开始。

落盘代码：`internal/storage/{doc,errors,observe,store,open,pragma,migrate,lock_unix,lock_windows}.go`，
匿名夹具与生成器在 `internal/storage/testdata/`。

现有绑定的 canWrite / isCaptureOwner 默认值仍不证明锁已建立——锁由 `storage.Open` 持有，
尚未经绑定层接入。缺陷诊断界面未实现，因此诊断数据目前无消费者。

实现与验证状态分别记录；下方“验证记录”只登记真实运行过的命令与结果。

## 能力与跨层职责

本模块先后可独立交付 db-core、settings-store、diagnostics 和维护能力；
这些是切片，不要求一次完成 data 才解锁其他功能。

**db-core 已可被消费者接入**（`09 §9.3`）：连接、PRAGMA、迁移链、只读降级与两把实例锁
已落盘并有测试。业务表仍须由各功能模块按需追加迁移版本，data 协调合入顺序。

| 输入 | 可独立推进 | 真实接入条件 |
|---|---|---|
| 03 schema 与 05 repository 契约 | 匿名新库、旧库、损坏库与并发 fixture | db-core 独立门禁；只用隔离测试目录 |
| recording: 活跃 / 收尾分段、Media.ProbeSegment | 假分段、故障与字节均摊 fixture | 真实生命周期和 media-read 接入后验证清理 / 恢复 |
| 各模块诊断数据 | 匿名计数 / 耗时、错误码与 DTO fixture | 计数从真实行为产生，禁止敏感活动信息 |
| preferences: settings-access / ui-bridge | 磁盘上限 / 遥测配置 fixture | 持久化、事件与生成绑定；G-host |


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
4. 增加 GetDiagnostics 与匿名指标、磁盘和遥测设置、store 与 UI；
   各功能在行为产生处计数，诊断页晚接入不能成为静默丢错的理由。
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

DB-3、DB-5、DB-9 未运行：分别依赖只读 repository 方法、`timeline_cards.metadata`、
分段生命周期与 `Media`，这些表与能力尚未交付，与 db-core 无关。
IT-12 未运行：real Media 未就绪，仅阻塞真实清理验收（见下）。

后续记录驱动 / 系统、commit、匿名夹具、并发时长、回读 PRAGMA 与完整性结果。
