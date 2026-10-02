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
  charts: {
    distribution: {
      title: '本週分布',
      total: '總計',
      aria: '本週各分類時長占比',
    },
    context: {
      title: '情境切換與分心對比',
      shifts: '情境切換',
      distractions: '分心',
      distribution: '時段分布（10:00–18:00）',
      comparison: '每日對比',
      insight: '{day}被打斷最多：{shifts} 次切換，{distracted} 次分心。',
      insightNone: '本週沒有發現明顯的切換或分心。',
      aria: '每天的情境切換與分心次數',
    },
    workflow: {
      title: '本週工作流',
      total: '本週合計',
      aria: '每天各時段的主要分類',
    },
    heatmap: {
      title: '專注與分心熱力圖',
      focused: '專注工作',
      distracted: '分心',
      aria: '每天各時段的專注與分心程度',
    },
    treemap: {
      title: '各分類最常用的應用程式',
      empty: '本週還沒有應用程式資料',
      aria: '各分類中使用時間最長的應用程式',
      // Dayflow's compact units ("6hr 4m", "+ 152m"), kept in English on purpose.
      hoursMinutes: '{hours}hr {minutes}m',
      hours: '{hours}hr',
      minutes: '{minutes}m',
    },
    sankey: {
      title: '分類與應用程式的時間流向',
      aria: '本週時間從分類流向應用程式',
    },
    tooltip: {
      shifts: '{count} 次情境切換',
      distractions: '{count} 次分心',
      noRecord: '無紀錄',
      share: '占{name}的 {value}',
      change: '較上週 {value}',
      newThisWeek: '上週未使用',
      pinHint: '點擊可固定醒目提示，再次點擊取消',
    },
    otherApp: '其他',
    otherCategory: '其他',
  },
}
