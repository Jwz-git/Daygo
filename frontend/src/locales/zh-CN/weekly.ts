export default {
  title: '周报',
  developmentFixture: '示例数据 · 仅开发环境',
  navigation: {
    label: '周导航',
    current: '本周',
    backendRequired: '暂时无法切换周',
  },
  meta: {
    tracked: '已记录 {count} 分钟',
  },
  intro: {
    eyebrow: '每周复盘',
    title: '这一周，时间花在了哪里',
    description: '看看这一周的投入时长、专注节奏、每天的分布和各类活动的占比。',
  },
  state: {
    loading: {
      title: '正在加载本周数据',
      description: '稍等片刻。',
    },
    unavailable: {
      title: '周报暂时无法显示',
      description: '请稍后再试，或重启 Daygo。',
    },
    failure: {
      title: '本周数据加载失败',
      description: '已有数据不受影响，可以重试。',
    },
    empty: {
      title: '这一周还没有记录',
      description: 'Daygo 记录并整理出活动后，周报会出现在这里。',
    },
  },
  scopeNote: '统计基于每天的活动卡片。',
  charts: {
    distribution: {
      title: '本周分布',
      total: '总计',
      aria: '本周各分类时长占比',
    },
    context: {
      title: '上下文切换与分心对比',
      shifts: '上下文切换',
      distractions: '分心',
      distribution: '时段分布（10:00–18:00）',
      comparison: '每日对比',
      insight: '{day}被打断最多：{shifts} 次切换，{distracted} 次分心。',
      insightNone: '本周没有发现明显的切换或分心。',
      aria: '每天的上下文切换与分心次数',
    },
    workflow: {
      title: '本周工作流',
      total: '本周合计',
      aria: '每天各时段的主要分类',
    },
    heatmap: {
      title: '专注与分心热力图',
      focused: '专注工作',
      distracted: '分心',
      aria: '每天各时段的专注与分心程度',
    },
    treemap: {
      title: '各分类最常用的应用',
      empty: '本周还没有应用数据',
      aria: '各分类中使用时间最长的应用',
      // Dayflow's compact units ("6hr 4m", "+ 152m"), kept in English on purpose.
      hoursMinutes: '{hours}hr {minutes}m',
      hours: '{hours}hr',
      minutes: '{minutes}m',
    },
    sankey: {
      title: '分类与应用的时间流向',
      aria: '本周时间从分类流向应用',
    },
    tooltip: {
      shifts: '{count} 次上下文切换',
      distractions: '{count} 次分心',
      noRecord: '无记录',
      share: '占{name}的 {value}',
      change: '较上周 {value}',
      newThisWeek: '上周未使用',
      pinHint: '点击可固定高亮，再次点击取消',
    },
    otherApp: '其他',
    otherCategory: '其他',
  },
}
