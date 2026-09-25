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
  overview: {
    eyebrow: '概览',
    title: '时间与专注',
    description: '专注时长不含分心和空闲；记录时长不含系统占位时段。',
    focusAria: '专注占已记录时间的 {value}',
  },
  metric: {
    tracked: '记录时长',
    focused: '专注时长',
    other: '其他时长',
    focusRate: '专注占比',
  },
  categories: {
    eyebrow: '构成',
    title: '分类分布',
    count: '{count} 个分类',
    distributionAria: '本周各分类时长占比',
    total: '总时长',
  },
  daily: {
    eyebrow: '节奏',
    title: '每日时间分布',
    hint: '每行一天，色块为该时段的分类',
    idleTag: '空闲',
    noActivity: '无活动',
  },
  rhythm: {
    eyebrow: '节律',
    title: '一周活跃节律',
    focus: '专注',
    idle: '空闲',
    chartAria: '各小时专注与空闲分钟数柱状图',
    empty: '本周还没有可以统计的时段',
  },
  insights: {
    title: '本周概要',
    activeDays: '活跃天数',
    daysCount: '{count} 天',
    ofSeven: '一周 7 天',
    longestFocus: '最长专注',
    peakHour: '高峰时段',
    avgFocus: '日均专注',
    perActiveDay: '按活跃天计',
    busiestDay: '最忙一天',
    none: '暂无',
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
}
