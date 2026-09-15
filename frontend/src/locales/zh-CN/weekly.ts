export default {
  title: '周报',
  developmentFixture: '示例数据 · 仅开发环境',
  navigation: {
    label: '周导航',
    current: '本周',
    backendRequired: '周数据绑定接入后可用',
  },
  meta: {
    tracked: '已跟踪 {count} 分钟',
  },
  intro: {
    eyebrow: '每周复盘',
    title: '这一周，时间花在了哪里',
    description: '把跟踪时长、专注节奏、每日分布和分类构成放在一起看，先呈现事实，不补写尚未产生的洞察。',
  },
  overview: {
    eyebrow: '概览',
    title: '时间与专注',
    description: '专注时长排除空闲分类；跟踪时长排除系统占位。数值以后端周聚合为准。',
    focusAria: '专注占已跟踪时间的 {value}',
  },
  metric: {
    tracked: '跟踪时长',
    focused: '专注时长',
    other: '其他时长',
    focusRate: '专注占比',
  },
  categories: {
    eyebrow: '构成',
    title: '分类分布',
    count: '{count} 个分类',
    distributionAria: '本周各分类时长占比',
    total: '总跟踪',
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
    empty: '本周暂无可汇总的时段数据',
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
      title: '正在读取每周数据',
      description: '周聚合返回后会显示跟踪、专注与分类构成。',
    },
    unavailable: {
      title: '每周数据能力尚不可用',
      description: '生产页面不会注入示例统计。接入 GetWeeklyDashboard 后，这里会显示真实周数据。',
    },
    failure: {
      title: '每周数据加载失败',
      description: '现有统计没有被覆盖。可以重试读取这一周。',
    },
    empty: {
      title: '这一周还没有可汇总的活动',
      description: '产生并处理时间线卡片后，每周统计会自动出现在这里。',
    },
  },
  scopeNote: '图表基于周聚合的按日明细与时段数据；应用级关系图与桑基图需要单独的数据契约。',
}
