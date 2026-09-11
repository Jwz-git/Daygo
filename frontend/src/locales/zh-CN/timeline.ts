export default {
  title: '时间线',
  developmentFixture: '样例数据 · 仅开发',
  navigation: {
    label: '逻辑日导航',
    backendRequired: '时间线数据绑定交付后可切换日期',
    futureUnavailable: '不能前往当前逻辑日之后',
  },
  meta: {
    tracked: '已跟踪 {count} 分钟',
  },
  filter: {
    label: '按分类筛选',
    all: '全部',
    manage: '管理分类',
    manageUnavailable: '分类管理绑定尚未交付',
  },
  track: {
    ariaLabel: '按小时排列的活动时间线',
  },
  processing: '正在分析这一时段…',
  failure: {
    title: '分析未完成',
  },
  state: {
    loading: {
      eyebrow: '正在加载',
      title: '读取时间线',
      description: '正在读取当天的活动。',
    },
    unavailable: {
      eyebrow: '暂不可用',
      title: '时间线数据能力尚不可用',
      description: '时间线数据尚未接入。生产页面不会使用示例活动填充。',
    },
    empty: {
      eyebrow: '今天',
      title: '这一天还没有活动',
      description: '捕获并完成分析后，活动会按真实发生时间出现在这里。',
    },
    processing: {
      eyebrow: '分析中',
      title: '正在整理活动片段',
      description: '进行中的时间范围会显示为骨架，完成后自动重新读取。',
    },
    failure: {
      eyebrow: '需要处理',
      title: '时间线暂时无法完整加载',
      description: '失败时段会保留在轨道中，不会被静默表现为空白。',
    },
    populated: {
      eyebrow: '时间线',
      title: '活动已就绪',
      description: '选择一张活动卡片查看详情。',
    },
  },
  overview: {
    eyebrow: '今日概览',
    title: '时间分布',
    tracked: '已跟踪',
    idle: '空闲',
    noCategories: '有活动后，这里会显示分类用时。',
  },
  inspector: {
    title: '活动详情',
    close: '关闭活动详情',
    summary: '摘要',
    noSummary: '这一活动没有摘要。',
    apps: '应用与站点',
    distractions: '分心片段',
    frames: '活动帧',
    framesUnavailable: '媒体读取绑定交付后按需加载，不阻塞时间线。',
    actionsUnavailable: '编辑、分类修改与删除将在卡片写入绑定交付后启用',
    readOnly: '当前为只读状态',
  },
  duration: {
    minutes: '{count} 分钟',
    hours: '{count} 小时',
    hoursMinutes: '{hours} 小时 {minutes} 分钟',
  },
}
