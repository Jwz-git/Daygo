// Anonymous sample data, shaped like a real Daygo day and week: gaps between
// activities, a few distraction cards, embedded distraction records, two plans.

export const CATS = {
  focus: { name: { zh: '专注工作', en: 'Focus Work' }, color: '#6E7DF7' },
  comm: { name: { zh: '沟通', en: 'Communication' }, color: '#F3B292' },
  learn: { name: { zh: '学习', en: 'Learning' }, color: '#78CCEA' },
  research: { name: { zh: '研究', en: 'Research' }, color: '#A15DF6' },
  personal: { name: { zh: '个人', en: 'Personal' }, color: '#B8E1E2' },
  distraction: { name: { zh: '分心', en: 'Distraction' }, color: '#EB5635' },
};

const t = (h, m = 0) => h * 60 + m;

// screen: mock frame in the player (editor | browser | chat | doc | meet | video | terminal)
export const CARDS = [
  { id: 1, cat: 'comm', start: t(9, 12), end: t(9, 34), app: 'messages', screen: 'chat',
    title: { zh: '回复团队消息', en: 'Catching up on team messages' },
    summary: { zh: '在信息里回复同事关于发布时间的讨论。', en: 'Replied to the team thread about the release date.' },
    detail: { zh: '确认 1.2 推到周五发布，顺带同步了设计稿的修改点。', en: 'Agreed to ship 1.2 on Friday and passed along the design changes.' },
    apps: [['messages', { zh: '信息', en: 'Messages' }]] },
  { id: 2, cat: 'focus', start: t(9, 50), end: t(10, 42), app: 'vscode', screen: 'editor',
    title: { zh: '修复时间线卡片重叠', en: 'Fixing overlapping timeline cards' },
    summary: { zh: '把卡片布局抽成纯函数，修掉相邻短卡互相覆盖的问题。', en: 'Pulled card layout into a pure function and fixed short cards covering each other.' },
    detail: { zh: '新增 layout.ts 和 6 个边界测试；顺手删掉了旧的分栏逻辑。', en: 'Added layout.ts with six edge-case tests and removed the old lane logic.' },
    apps: [['vscode', 'VS Code'], ['terminal', { zh: '终端', en: 'Terminal' }]],
    distractions: [{ at: t(10, 18), len: 4, text: { zh: '看了一眼 YouTube 推荐', en: 'Glanced at YouTube recommendations' } }] },
  { id: 3, cat: 'focus', start: t(10, 42), end: t(11, 6), app: 'claude', screen: 'chat',
    title: { zh: '和 Claude 讨论布局算法', en: 'Talking layout with Claude' },
    summary: { zh: '比较了三种裁剪重叠卡片的办法。', en: 'Compared three ways to trim overlapping cards.' },
    detail: { zh: '最后选了「按较短卡片边缘裁剪」，和原来的参考实现一致。', en: 'Went with trimming to the shorter card’s edge, matching the reference.' },
    apps: [['claude', 'Claude']] },
  { id: 4, cat: 'distraction', start: t(11, 6), end: t(11, 24), app: 'youtube', screen: 'video',
    title: { zh: '刷 YouTube 视频', en: 'Watching YouTube' },
    summary: { zh: '连着看了几个推荐的科技视频。', en: 'Watched a few recommended tech videos in a row.' },
    detail: '', apps: [['youtube', 'YouTube']] },
  { id: 5, cat: 'research', start: t(11, 38), end: t(12, 14), app: 'safari', screen: 'browser',
    title: { zh: '调研 WebKit 毛玻璃性能', en: 'Researching WebKit blur performance' },
    summary: { zh: '查 backdrop-filter 在 WebView 里掉帧的原因。', en: 'Looked into why backdrop-filter drops frames in WebView.' },
    detail: { zh: '定位到多层模糊叠加导致重绘，记下了两种规避方案。', en: 'Traced it to stacked blur layers forcing repaints; noted two workarounds.' },
    apps: [['safari', 'webkit.org']] },
  { id: 6, cat: 'comm', start: t(13, 30), end: t(13, 52), app: 'chrome', screen: 'meet',
    title: { zh: '产品周会', en: 'Weekly product sync' },
    summary: { zh: '同步本周目标，确定周报导出挪到下个迭代。', en: 'Synced weekly goals; weekly export moves to the next sprint.' },
    detail: '', apps: [['chrome', 'Google Meet']] },
  { id: 7, cat: 'focus', start: t(13, 52), end: t(15, 6), app: 'vscode', screen: 'editor',
    title: { zh: '实现周报矩形树图', en: 'Building the weekly treemap' },
    summary: { zh: '移植 squarify 布局，加上较上周的变化徽标。', en: 'Ported the squarify layout and added week-over-week badges.' },
    detail: { zh: '树图按 800×400 设计空间布局再按百分比缩放，窄窗口时自动降级为紧凑样式。', en: 'Laid out in an 800×400 design space and scaled by percent; tiles fall back to compact when narrow.' },
    apps: [['vscode', 'VS Code'], ['github', 'github.com']],
    distractions: [{ at: t(14, 31), len: 6, text: { zh: '刷 Discord 频道', en: 'Scrolled a Discord channel' } }] },
  { id: 8, cat: 'distraction', start: t(15, 6), end: t(15, 20), app: 'discord', screen: 'chat',
    title: { zh: '闲逛 Discord', en: 'Browsing Discord' },
    summary: { zh: '在几个频道里来回看消息。', en: 'Hopped between a few channels.' }, detail: '', apps: [['discord', 'Discord']] },
  { id: 9, cat: 'focus', start: t(15, 20), end: t(15, 44), app: 'github', screen: 'browser',
    title: { zh: '代码评审：支付模块', en: 'Code review: payments' },
    summary: { zh: '留下 4 条评论后批准合并。', en: 'Left four comments, then approved.' },
    detail: { zh: '主要指出重试逻辑缺少幂等键，作者已在同一 PR 里修复。', en: 'Main point: retries lacked an idempotency key; fixed in the same PR.' },
    apps: [['github', 'github.com']] },
  { id: 10, cat: 'learn', start: t(16, 10), end: t(17, 6), app: null, screen: 'video',
    title: { zh: '网课：编译原理', en: 'Lecture: compilers' },
    summary: { zh: '看完了语法分析那一节，记了笔记。', en: 'Finished the parsing lecture and took notes.' },
    detail: { zh: '重点是 LR(1) 项目集的构造，课后题留到周末。', en: 'Focused on building LR(1) item sets; exercises left for the weekend.' },
    apps: [[null, 'course.edu'], ['notes', { zh: '备忘录', en: 'Notes' }]] },
  { id: 11, cat: 'personal', start: t(17, 6), end: t(17, 22), app: 'notes', screen: 'doc',
    title: { zh: '整理备忘录', en: 'Tidying notes' },
    summary: { zh: '把这周的待办归档，清掉过期事项。', en: 'Archived this week’s to-dos and cleared stale ones.' }, detail: '', apps: [['notes', { zh: '备忘录', en: 'Notes' }]] },
  { id: 12, cat: 'focus', start: t(19, 4), end: t(19, 48), app: 'terminal', screen: 'terminal',
    title: { zh: '跑测试并打包', en: 'Running tests and builds' },
    summary: { zh: '全量测试通过，打出 1.2 的候选包。', en: 'Full test suite green; built the 1.2 candidate.' },
    detail: { zh: '签名证书还没到，安装包暂时只能本地验证。', en: 'Signing certificate still pending, so the build is local-only for now.' },
    apps: [['terminal', { zh: '终端', en: 'Terminal' }], ['xcode', 'Xcode']] },
  { id: 13, cat: 'distraction', start: t(19, 48), end: t(20, 4), app: 'youtube', screen: 'video',
    title: { zh: '看 YouTube', en: 'Watching YouTube' },
    summary: { zh: '饭后看了两个视频。', en: 'Two videos after dinner.' }, detail: '', apps: [['youtube', 'YouTube']] },
  { id: 14, cat: 'research', start: t(20, 4), end: t(20, 46), app: 'gemini', screen: 'chat',
    title: { zh: '用 Gemini 查桑基图画法', en: 'Asking Gemini about sankey charts' },
    summary: { zh: '比较了几种桑基图的布局和配色。', en: 'Compared a few sankey layouts and colour schemes.' },
    detail: { zh: '决定沿用重心排序，让应用节点贴近给它供流的分类。', en: 'Kept barycentric ordering so apps sit near the categories feeding them.' },
    apps: [['gemini', 'Gemini']] },
  { id: 15, cat: 'focus', start: t(21, 4), end: t(21, 42), app: 'daygo', screen: 'doc',
    title: { zh: '写 Daygo 发布说明', en: 'Writing Daygo release notes' },
    summary: { zh: '列出 1.2 的新功能和已知问题。', en: 'Listed what’s new in 1.2 and the known issues.' },
    detail: '', apps: [['daygo', 'Daygo']] },
];

// The window still being analysed; it turns into this card while you watch.
export const PENDING = { id: 16, cat: 'personal', start: t(21, 42), end: t(21, 58), app: 'notes', screen: 'doc',
  title: { zh: '规划明天', en: 'Planning tomorrow' },
  summary: { zh: '排好明天的三个时间块。', en: 'Blocked out three slots for tomorrow.' }, detail: '', apps: [['notes', { zh: '备忘录', en: 'Notes' }]] };
export const NOW = t(22, 3);

export const PLANS = [
  { id: 'p1', start: t(9, 45), end: t(11, 0), cat: 'focus', status: 'done', title: { zh: '时间线重构', en: 'Timeline refactor' } },
  { id: 'p2', start: t(13, 45), end: t(15, 30), cat: 'focus', status: 'planned', title: { zh: '周报图表', en: 'Weekly charts' } },
];

export const STANDUP = {
  done: [
    { zh: '修复时间线卡片重叠，补齐 6 个边界测试', en: 'Fixed overlapping timeline cards with six edge-case tests' },
    { zh: '完成周报矩形树图和变化徽标', en: 'Shipped the weekly treemap with change badges' },
    { zh: '评审并合并支付模块 PR', en: 'Reviewed and merged the payments PR' },
  ],
  next: [
    { zh: '拿到签名证书后公证 1.2 安装包', en: 'Notarize the 1.2 build once the certificate lands' },
    { zh: '规避 WebView 里的毛玻璃掉帧', en: 'Work around blur frame drops in WebView' },
  ],
};

// Week: category minutes, apps with change vs last week (minutes, null = new).
export const WEEK = [
  { key: 'focus', apps: [['VS Code', 'vscode', 470, 132], ['Claude', 'claude', 288, 210], ['Daygo', 'daygo', 205, null], [{ zh: '终端', en: 'Terminal' }, 'terminal', 96, -24], ['GitHub', 'github', 58, 12], ['Xcode', 'xcode', 29, null]] },
  { key: 'distraction', apps: [['YouTube', 'youtube', 238, -164], ['Discord', 'discord', 97, 18], ['Finder', 'finder', 60, -9]] },
  { key: 'learn', apps: [['course.edu', null, 84, -41], ['MDN', 'safari', 48, 20]] },
  { key: 'research', apps: [['Gemini', 'gemini', 52, null], ['Safari', 'safari', 36, 8]] },
  { key: 'comm', apps: [[{ zh: '信息', en: 'Messages' }, 'messages', 25, -6], ['Slack', null, 16, 4]] },
  { key: 'personal', apps: [[{ zh: '备忘录', en: 'Notes' }, 'notes', 30, 10]] },
];

export const CHAT = {
  hints: [
    { zh: '今天我做了什么？', en: 'What did I do today?' },
    { zh: '本周时间花在哪了？', en: 'Where did my week go?' },
    { zh: '帮我写今天的站会', en: "Write today's standup" },
  ],
  answers: [
    { zh: '今天记录了 **8 小时 51 分钟**，大头在三件事上：\n\n- **实现周报矩形树图**（1 小时 14 分钟）\n- **修复时间线卡片重叠**（52 分钟）\n- **网课：编译原理**（56 分钟）\n\n分心 **48 分钟**，主要是 YouTube。',
      en: 'You recorded **8 hr 51 min** today, mostly on three things:\n\n- **Building the weekly treemap** (1 hr 14 min)\n- **Fixing overlapping timeline cards** (52 min)\n- **Lecture: compilers** (56 min)\n\n**48 min** went to distractions, mostly YouTube.' },
    { zh: '本周记录 **30 小时 32 分钟**：**专注工作**占 63%，**分心** 22%，**学习** 7%。\n\nClaude 比上周多用了 **210 分钟**，YouTube 少了 **164 分钟**。',
      en: 'This week: **30 hr 32 min**. **Focus Work** 63%, **Distraction** 22%, **Learning** 7%.\n\nClaude is up **210 min** on last week; YouTube is down **164 min**.' },
    { zh: '**完成事项**\n\n- 修复时间线卡片重叠\n- 完成周报矩形树图\n- 合并支付模块 PR\n\n**下一步**\n\n- 公证 1.2 安装包',
      en: '**Completed work**\n\n- Fixed overlapping timeline cards\n- Shipped the weekly treemap\n- Merged the payments PR\n\n**Next steps**\n\n- Notarize the 1.2 build' },
  ],
};
