export default {
  title: '週報',
  developmentFixture: '範例資料 · 僅開發環境',
  navigation: {
    label: '週導覽',
    current: '本週',
    backendRequired: '暫時無法切換週',
  },
  meta: {
    tracked: '已記錄 {count} 分鐘',
  },
  intro: {
    eyebrow: '每週回顧',
    title: '這一週，時間花在了哪裡',
    description: '看看這一週的投入時長、專注節奏、每天的分布和各類活動的佔比。',
  },
  overview: {
    eyebrow: '概覽',
    title: '時間與專注',
    description: '專注時長不含分心和閒置；記錄時長不含系統佔位時段。',
    focusAria: '專注佔已記錄時間的 {value}',
  },
  metric: {
    tracked: '記錄時長',
    focused: '專注時長',
    other: '其他時長',
    focusRate: '專注佔比',
  },
  categories: {
    eyebrow: '構成',
    title: '分類分布',
    count: '{count} 個分類',
    distributionAria: '本週各分類時長佔比',
    total: '總時長',
  },
  daily: {
    eyebrow: '節奏',
    title: '每日時間分布',
    hint: '每列一天，色塊為該時段的分類',
    idleTag: '閒置',
    noActivity: '無活動',
  },
  rhythm: {
    eyebrow: '節律',
    title: '一週活躍節律',
    focus: '專注',
    idle: '閒置',
    chartAria: '各小時專注與閒置分鐘數柱狀圖',
    empty: '本週還沒有可以統計的時段',
  },
  insights: {
    title: '本週概要',
    activeDays: '活躍天數',
    daysCount: '{count} 天',
    ofSeven: '一週 7 天',
    longestFocus: '最長專注',
    peakHour: '高峰時段',
    avgFocus: '日均專注',
    perActiveDay: '按活躍天計',
    busiestDay: '最忙一天',
    none: '暫無',
  },
  state: {
    loading: {
      title: '正在載入本週資料',
      description: '稍等片刻。',
    },
    unavailable: {
      title: '週報暫時無法顯示',
      description: '請稍後再試，或重新啟動 Daygo。',
    },
    failure: {
      title: '本週資料載入失敗',
      description: '現有資料不受影響，可以重試。',
    },
    empty: {
      title: '這一週還沒有記錄',
      description: 'Daygo 記錄並整理出活動後，週報會出現在這裡。',
    },
  },
  scopeNote: '統計基於每天的活動卡片。',
}
