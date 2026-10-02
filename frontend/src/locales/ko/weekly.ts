export default {
  title: '위클리',
  developmentFixture: '예시 데이터 · 개발 전용',
  navigation: {
    label: '주 탐색',
    current: '이번 주',
    backendRequired: '지금은 주를 바꿀 수 없습니다',
  },
  meta: {
    tracked: '{count}분 기록됨',
  },
  intro: {
    eyebrow: '위클리 리뷰',
    title: '이번 주, 시간은 어디로',
    description: '이번 주의 기록 시간, 집중 리듬, 요일별 분포, 카테고리 구성을 확인하세요.',
  },
  state: {
    loading: {
      title: '이번 주를 불러오는 중',
      description: '잠시만 기다려 주세요.',
    },
    unavailable: {
      title: '지금은 위클리를 표시할 수 없습니다',
      description: '나중에 다시 시도하거나 Daygo를 다시 시작하세요.',
    },
    failure: {
      title: '이번 주를 불러오지 못했습니다',
      description: '데이터에는 영향이 없습니다. 다시 시도할 수 있습니다.',
    },
    empty: {
      title: '이번 주는 아직 기록이 없습니다',
      description: 'Daygo가 활동을 기록하고 정리하면 여기에 위클리 리뷰가 나타납니다.',
    },
  },
  scopeNote: '매일의 활동 카드를 기준으로 집계합니다.',
  charts: {
    distribution: {
      title: '이번 주 분포',
      total: '합계',
      aria: '이번 주 카테고리별 시간 비율',
    },
    context: {
      title: '맥락 전환과 산만함 비교',
      shifts: '맥락 전환',
      distractions: '산만함',
      distribution: '시간대 분포(10:00–18:00)',
      comparison: '일별 비교',
      insight: '{day}에 가장 많이 끊겼습니다: 전환 {shifts}회, 산만함 {distracted}회.',
      insightNone: '이번 주에는 눈에 띄는 전환이나 산만함이 없었습니다.',
      aria: '일별 맥락 전환과 산만함 횟수',
    },
    workflow: {
      title: '이번 주 워크플로',
      total: '이번 주 합계',
      aria: '매일 시간대별 주요 카테고리',
    },
    heatmap: {
      title: '집중과 산만함 히트맵',
      focused: '집중 작업',
      distracted: '산만함',
      aria: '매일 시간대별 집중과 산만함 정도',
    },
    treemap: {
      title: '카테고리별 가장 많이 쓴 앱',
      empty: '이번 주 앱 데이터가 아직 없습니다',
      aria: '카테고리마다 가장 오래 쓴 앱',
      // Dayflow's compact units ("6hr 4m", "+ 152m"), kept in English on purpose.
      hoursMinutes: '{hours}hr {minutes}m',
      hours: '{hours}hr',
      minutes: '{minutes}m',
    },
    sankey: {
      title: '카테고리와 앱 사이의 시간 흐름',
      aria: '이번 주 시간이 카테고리에서 앱으로 흐른 모습',
    },
    tooltip: {
      shifts: '맥락 전환 {count}회',
      distractions: '산만함 {count}회',
      noRecord: '기록 없음',
      share: '{name}의 {value}',
      change: '지난주 대비 {value}',
      newThisWeek: '지난주에는 사용 안 함',
      pinHint: '클릭하면 강조가 고정되고, 다시 클릭하면 해제됩니다',
    },
    otherApp: '기타',
    otherCategory: '기타',
  },
}
