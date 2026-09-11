export default {
  title: '每日',
  developmentFixture: '样例数据 · 仅开发',
  navigation: {
    label: '每日复盘日期导航',
    backendRequired: '日期与时间线绑定完整接入后可切换日期',
    futureUnavailable: '不能前往当前逻辑日之后',
  },
  meta: {
    tracked: '已跟踪 {count} 分钟',
  },
  overview: {
    eyebrow: '每日复盘',
    title: '先看清今天，再决定明天',
    description: '活动按真实时间归入工作流；下方日报只呈现后端已经保存的内容，不会用前端推断补写。',
  },
  workflow: {
    title: '工作流概览',
    description: '按分类查看一天里的投入与切换。',
    slotNote: '每格 15 分钟 · 右上角红点表示该活动含分心记录',
    empty: '这一天还没有可汇总的活动。',
    total: '当日合计',
  },
  stats: {
    title: '当日统计',
    contextSwitched: '上下文切换',
    interrupted: '分心记录',
    focusedFor: '有效投入',
    distractedFor: '分心时长',
    transitioning: '未跟踪间隔',
    times: '{count} 次',
  },
  standup: {
    title: '日报草稿',
    description: '按完成事项、下一步和限制整理，可直接复制。',
    highlights: '完成事项',
    tasks: '下一步',
    blockers: '当前限制',
    noBlockers: '没有记录限制。',
    copy: '复制日报',
    copied: '已复制',
    copyFailed: '复制失败',
    generatedAt: '生成于 {date}',
    generateUnavailable: '日报生成绑定尚未交付',
    unavailableTitle: '日报能力尚未接入',
    unavailableDescription: '工作流仍可查看；生成、编辑和自动保存会在真实绑定可用后启用。',
    failureTitle: '日报读取失败',
    failureDescription: '活动概览没有受到影响，可以稍后重新读取日报。',
  },
  state: {
    loading: {
      title: '正在读取每日概览',
      description: '活动与日报会分别读取，尚未返回的内容不会用占位数据补齐。',
    },
    unavailable: {
      title: '每日数据能力尚不可用',
      description: '生产页面不会注入示例活动。接入日期与时间线绑定后，这里会显示真实数据。',
    },
    failure: {
      title: '每日概览读取失败',
      description: '现有内容没有被改写。请重试，或稍后再回来。',
    },
    empty: {
      title: '这一天还没有可复盘的内容',
      description: '活动完成分析或日报保存后，会出现在这里。',
    },
    populated: {
      title: '每日概览已就绪',
      description: '查看工作流与日报。',
    },
  },
  duration: {
    minutes: '{count} 分钟',
    hours: '{count} 小时',
    hoursMinutes: '{hours} 小时 {minutes} 分钟',
  },
}
