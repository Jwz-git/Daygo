// Two-language copy. App strings are taken from Daygo's own locales
// (frontend/src/locales/{zh-CN,en}) so the replica reads exactly like the app.

const STR = {
  zh: {
    'meta.title': 'Daygo — 你的一天，AI 都记得',
    'nav.timeline': '时间线', 'nav.daily': '日报', 'nav.weekly': '周报', 'nav.privacy': '隐私', 'nav.download': '下载',
    'hero.pill': '正在记录',
    'hero.pill2': 'Dayflow 的跨平台改版',
    'origin.k': '19:30 · 起源', 'origin.h': 'Dayflow 的<span class="grad">跨平台改版。</span>',
    'origin.p': '原版 Dayflow 是用 Swift 写的 macOS 应用。Daygo 沿用它的设计，用 Go + Vue 重写，带到了 Windows。',
    'origin.orig': '原版', 'origin.remake': '复刻改版', 'origin.link': 'Dayflow 源码 ↗', 'origin.arrow': '重写 · 跨平台',
    'pv.1': '本地优先', 'pv.1d': '数据保存在本机，截图仅发送给你配置的 AI', 'pv.2': '自带模型', 'pv.2d': 'OpenAI · Anthropic · 本地模型',
    'pv.3': '屏蔽应用', 'pv.3d': '敏感应用只记一帧占位', 'pv.4': '密钥进钥匙串', 'pv.4d': '界面只写不读',
    'foot.credit': '基于 <a href="https://dayflow.so" target="_blank" rel="noopener">Dayflow</a>（<a href="https://github.com/JerryZLiu/Dayflow" target="_blank" rel="noopener">MIT</a> · Jerry Liu）的跨平台复刻改版',
    'hero.l1': '一天结束，', 'hero.l2': 'Daygo 都记得。',
    'hero.en': 'Your day, remembered.',
    'hero.sub': '后台安静截屏，用你自己的 AI，<br class="br-sm" />整理成时间线、日报与周报。',
    'hero.mac': '下载 macOS 版', 'hero.note': 'macOS 14+ · Apple Silicon　/　Windows 11',
    'cap.timeline.k': '09:00 · 时间线', 'cap.timeline': '每一刻，<span class="grad">自动成卡。</span>',
    'win.hint': '点一张卡片看看 ↗',
    'daily.k': '11:30 · 日报', 'daily.h': '今天做了什么，<span class="grad">一眼看清。</span>', 'daily.p': '投入、切换和站会，自动整理好。',
    'weekly.k': '14:00 · 周报', 'weekly.h': '时间去哪了，<span class="grad">一目了然。</span>', 'weekly.p': '悬停任意分类或应用，看清它的流向。',
    'chat.k': '16:00 · 对话 <em>预览</em>', 'chat.h': '直接问，<span class="grad">你的一天。</span>',
    'how.k': '18:40 · 原理', 'how.h': '三步，<span class="grad">零操作。</span>', 'how.p': '打开一次，然后忘了它。',
    'how.s1': '截屏', 'how.s1d': '每 10 秒一帧，不录视频', 'how.s2': '理解', 'how.s2d': '交给你选的模型', 'how.s3': '成卡', 'how.s3d': '标题、摘要、分类',
    'privacy.k': '20:30 · 隐私', 'privacy.h': '你的屏幕，<span class="grad">只属于你。</span>',
    'badge.1': '无服务器', 'badge.2': '无账号', 'badge.3': '密钥进钥匙串', 'badge.4': 'MIT 开源',
    'cta.h': '今天，辛苦了。', 'cta.en': 'See you tomorrow.', 'cta.win': 'Windows 版',
    'cta.note': 'macOS 14+ · Apple Silicon　/　Windows 11 · x64　/　MIT 开源',

    // app
    'rail.timeline': '时间线', 'rail.daily': '日报', 'rail.weekly': '周报', 'rail.chat': '对话', 'rail.settings': '设置', 'rail.rec': '正在记录',
    'common.today': '今天', 'common.day': '天', 'common.week': '周', 'common.all': '全部', 'common.tz': '中国标准时间', 'common.close': '关闭',
    'tl.generating': '正在生成下一张卡片', 'tl.recordingNow': '正在记录',
    'tl.review': '审阅卡片', 'tl.copy': '复制时间线', 'tl.copied': '已复制',
    'ov.eyebrow': '今日概览', 'ov.title': '时间分布', 'ov.total': '总计',
    'plan.title': '今日计划', 'plan.done': '已完成', 'plan.upcoming': '未开始',
    'insp.close': '关闭活动详情', 'insp.summary': '摘要', 'insp.detailed': '详细摘要', 'insp.apps': '应用与站点', 'insp.distractions': '分心片段',
    'insp.verdict': '你的判定', 'v.distraction': '分心', 'v.neutral': '中性', 'v.focus': '专注', 'v.undo': '撤销',
    'review.hint': '逐张回顾时间线上的卡片，标记它们是专注还是分心。', 'review.done': '全部看完了！', 'review.doneBody': '你的回顾已更新。',
    'player.play': '播放',
    'wf.title': '工作流概览', 'wf.desc': '按分类查看一天里的投入与切换。', 'wf.note': '每格 15 分钟 · 右上角红点表示该活动含分心记录',
    'wf.distractions': '分心', 'wf.total': '当日合计',
    'su.title': '今天，9月30日的站会', 'su.desc': '按完成事项、下一步和限制整理，可直接复制。', 'su.done': '完成事项', 'su.next': '下一步',
    'su.copy': '复制日报', 'su.regen': '重新生成', 'su.copied': '已复制',
    'wk.treemap': '各分类最常用的应用', 'wk.sankey': '分类与应用的时间流向', 'wk.week': '9月28日 – 10月4日', 'wk.foot': '基于你每天的活动卡片。', 'wk.other': '其他',
    'chat.new': '新对话', 'chat.provider': 'AI 服务', 'chat.providerVal': '我的 Provider', 'chat.model': '模型', 'chat.modelVal': '默认',
    'chat.hello': '你好，有什么可以帮你的？', 'chat.helloSub': '可以问我时间线、日报、周报里的任何事', 'chat.placeholder': '输入消息…（Enter 发送）',
    'chat.read.day': '已读取 今天的时间线', 'chat.read.week': '已读取 本周周报',
    'pv.title': '录屏隐私', 'pv.hint': '这些应用在前台时，Daygo 不会保存画面，时间线上只标记这段时间有活动。',
    'pv.search': '搜索已安装的应用', 'pv.installed': '已安装的应用', 'pv.shown': '已显示 {n} 个', 'pv.blocked': '已屏蔽的应用', 'pv.blockedN': '已屏蔽 {n} 个', 'pv.clear': '清除', 'pv.empty': '没有匹配的应用。', 'pv.none': '还没有屏蔽任何应用。',
  },
  en: {
    'meta.title': 'Daygo — Your day, remembered',
    'nav.timeline': 'Timeline', 'nav.daily': 'Daily', 'nav.weekly': 'Weekly', 'nav.privacy': 'Privacy', 'nav.download': 'Download',
    'hero.pill': 'Recording',
    'hero.pill2': 'A cross-platform remake of Dayflow',
    'origin.k': '19:30 · Origin', 'origin.h': 'Dayflow, <span class="grad">rebuilt for every desktop.</span>',
    'origin.p': 'Dayflow is a macOS app written in Swift. Daygo keeps its design, rewrites it in Go + Vue, and brings it to Windows.',
    'origin.orig': 'Original', 'origin.remake': 'Remake', 'origin.link': 'Dayflow on GitHub ↗', 'origin.arrow': 'Rewritten · cross-platform',
    'pv.1': 'Local first', 'pv.1d': 'Data stays local; screenshots go only to the AI you configure', 'pv.2': 'Your own model', 'pv.2d': 'OpenAI · Anthropic · local models',
    'pv.3': 'Blocked apps', 'pv.3d': 'Sensitive apps leave only a placeholder', 'pv.4': 'Keys in Keychain', 'pv.4d': 'Write-only from the interface',
    'foot.credit': 'A cross-platform remake of <a href="https://dayflow.so" target="_blank" rel="noopener">Dayflow</a> (<a href="https://github.com/JerryZLiu/Dayflow" target="_blank" rel="noopener">MIT</a> · Jerry Liu)',
    'hero.l1': 'At the end of the day,', 'hero.l2': 'Daygo remembers.',
    'hero.en': 'Every moment, kept.',
    'hero.sub': 'Quiet screenshots in the background, organized by your own AI<br class="br-sm" /> into a timeline, daily recaps and weekly reviews.',
    'hero.mac': 'Download for macOS', 'hero.note': 'macOS 14+ · Apple Silicon　/　Windows 11',
    'cap.timeline.k': '09:00 · Timeline', 'cap.timeline': 'Every moment, <span class="grad">a card.</span>',
    'win.hint': 'Click a card ↗',
    'daily.k': '11:30 · Daily', 'daily.h': 'What you did today, <span class="grad">at a glance.</span>', 'daily.p': 'Focus, switches and your standup, already written.',
    'weekly.k': '14:00 · Weekly', 'weekly.h': 'Where the week <span class="grad">actually went.</span>', 'weekly.p': 'Hover any category or app to follow its time.',
    'chat.k': '16:00 · Chat <em>Preview</em>', 'chat.h': 'Just ask <span class="grad">about your day.</span>',
    'how.k': '18:40 · How', 'how.h': 'Three steps, <span class="grad">zero effort.</span>', 'how.p': 'Open it once, then forget it.',
    'how.s1': 'Capture', 'how.s1d': 'One frame every 10s, no video', 'how.s2': 'Understand', 'how.s2d': 'Read by the model you pick', 'how.s3': 'Card', 'how.s3d': 'Title, summary, category',
    'privacy.k': '20:30 · Privacy', 'privacy.h': 'Your screen <span class="grad">stays yours.</span>',
    'badge.1': 'No server', 'badge.2': 'No account', 'badge.3': 'Keys in Keychain', 'badge.4': 'MIT licensed',
    'cta.h': 'Good work today.', 'cta.en': 'See you tomorrow.', 'cta.win': 'Windows',
    'cta.note': 'macOS 14+ · Apple Silicon　/　Windows 11 · x64　/　MIT licensed',

    'rail.timeline': 'Timeline', 'rail.daily': 'Daily', 'rail.weekly': 'Weekly', 'rail.chat': 'Chat', 'rail.settings': 'Settings', 'rail.rec': 'Recording',
    'common.today': 'Today', 'common.day': 'Day', 'common.week': 'Week', 'common.all': 'All', 'common.tz': 'China Standard Time', 'common.close': 'Close',
    'tl.generating': 'Generating the next card', 'tl.recordingNow': 'Recording now',
    'tl.review': 'Review cards', 'tl.copy': 'Copy timeline', 'tl.copied': 'Copied',
    'ov.eyebrow': 'Today', 'ov.title': 'Time breakdown', 'ov.total': 'Total',
    'plan.title': "Today's plan", 'plan.done': 'Done', 'plan.upcoming': 'Upcoming',
    'insp.close': 'Close activity detail', 'insp.summary': 'Summary', 'insp.detailed': 'Detailed summary', 'insp.apps': 'Apps and sites', 'insp.distractions': 'Distractions',
    'insp.verdict': 'Your verdict', 'v.distraction': 'Distraction', 'v.neutral': 'Neutral', 'v.focus': 'Focus', 'v.undo': 'Undo',
    'review.hint': 'Go through your timeline cards one by one and mark each as focus or distraction.', 'review.done': 'All caught up!', 'review.doneBody': 'Your review is up to date.',
    'player.play': 'Play',
    'wf.title': 'Workflow overview', 'wf.desc': 'See where the day went and when the context changed.', 'wf.note': '15 minutes per cell · a red corner dot marks an activity with a distraction record',
    'wf.distractions': 'Distractions', 'wf.total': 'Day total',
    'su.title': 'Standup for today, Sep 30', 'su.desc': 'A copy-ready view of completed work and next steps.', 'su.done': 'Completed work', 'su.next': 'Next steps',
    'su.copy': 'Copy recap', 'su.regen': 'Regenerate', 'su.copied': 'Copied',
    'wk.treemap': 'Most used per category', 'wk.sankey': 'Time between categories and apps', 'wk.week': 'Sep 28 – Oct 4', 'wk.foot': 'Based on your daily activity cards.', 'wk.other': 'Other',
    'chat.new': 'New chat', 'chat.provider': 'AI service', 'chat.providerVal': 'My provider', 'chat.model': 'Model', 'chat.modelVal': 'Default',
    'chat.hello': 'Hi, how can I help?', 'chat.helloSub': 'Ask me anything about your timeline, daily or weekly recap', 'chat.placeholder': 'Message… (Enter to send)',
    'chat.read.day': "Read today's timeline", 'chat.read.week': "Read this week's review",
    'pv.title': 'Screen privacy', 'pv.hint': 'While these apps are in front, Daygo doesn’t save what’s on screen — the timeline only notes that you were active.',
    'pv.search': 'Search installed apps', 'pv.installed': 'Installed apps', 'pv.shown': '{n} shown', 'pv.blocked': 'Blocked apps', 'pv.blockedN': '{n} blocked', 'pv.clear': 'Clear', 'pv.empty': 'No matching apps.', 'pv.none': 'No apps blocked yet.',
  },
};

const listeners = [];
// English unless the visitor picked a language with the nav switch.
let lang = (() => {
  try {
    const saved = localStorage.getItem('daygo-lang');
    if (saved === 'zh' || saved === 'en') return saved;
  } catch { /* storage unavailable */ }
  return 'en';
})();

export const getLang = () => lang;
export function setLang(next) {
  if (next === lang) return;
  lang = next;
  try { localStorage.setItem('daygo-lang', next); } catch { /* storage unavailable */ }
  listeners.forEach((fn) => fn(next));
}
export const onLang = (fn) => listeners.push(fn);

/** Pick the current language from a { zh, en } pair; plain values pass through. */
export const L = (v) => (v && typeof v === 'object' && 'zh' in v ? v[lang] : v);

export function t(key, vars) {
  let s = STR[lang][key] ?? STR.zh[key] ?? key;
  if (vars) for (const k of Object.keys(vars)) s = s.replace(`{${k}}`, vars[k]);
  return s;
}

/** Durations the way the app writes them: "1 小时 50 分钟" / "1 hr 50 min". */
export function dur(min) {
  const m = Math.round(min), h = Math.floor(m / 60), r = m % 60;
  if (lang === 'zh') return h ? (r ? `${h} 小时 ${r} 分钟` : `${h} 小时`) : `${r} 分钟`;
  return h ? (r ? `${h} hr ${r} min` : `${h} hr`) : `${r} min`;
}
/** Treemap's compact units, English in both languages as in the app. */
export function durCompact(min) {
  const m = Math.round(min), h = Math.floor(m / 60), r = m % 60;
  return h ? (r ? `${h}hr ${r}m` : `${h}hr`) : `${r}m`;
}
export const fmt12 = (m) => {
  const h = Math.floor(m / 60) % 24, mm = Math.floor(m % 60);
  return `${((h + 11) % 12) + 1}:${String(mm).padStart(2, '0')} ${h < 12 ? 'AM' : 'PM'}`;
};
export const fmt24 = (m) => `${Math.floor(m / 60)}:${String(Math.floor(m % 60)).padStart(2, '0')}`;
/** Hour labels on the track (Intl hour+minute in the app's locale). */
export const hourLabel = (h) => (lang === 'zh' ? `${String(h).padStart(2, '0')}:00` : fmt12(h * 60));
/** Hour ticks on the daily workflow axis. */
export const hourTick = (h) => (lang === 'zh' ? `${h}时` : `${((h + 11) % 12) + 1} ${h % 24 < 12 ? 'AM' : 'PM'}`);
