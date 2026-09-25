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
  overview: {
    eyebrow: '개요',
    title: '시간과 집중',
    description: '집중 시간에는 산만한 시간과 유휴가 포함되지 않습니다. 기록 시간에는 시스템 자리 표시 구간이 포함되지 않습니다.',
    focusAria: '기록 시간 중 집중이 {value}',
  },
  metric: {
    tracked: '기록 시간',
    focused: '집중 시간',
    other: '기타 시간',
    focusRate: '집중 비중',
  },
  categories: {
    eyebrow: '구성',
    title: '카테고리 분포',
    count: '카테고리 {count}개',
    distributionAria: '이번 주 카테고리별 기록 시간 비중',
    total: '합계',
  },
  daily: {
    eyebrow: '패턴',
    title: '요일별 타임라인',
    hint: '한 줄이 하루, 각 블록이 카테고리 구간입니다',
    idleTag: '유휴',
    noActivity: '활동 없음',
  },
  rhythm: {
    eyebrow: '리듬',
    title: '한 주의 활동 리듬',
    focus: '집중',
    idle: '유휴',
    chartAria: '시간대별 집중과 유휴 분',
    empty: '이번 주에는 아직 집계할 구간이 없습니다',
  },
  insights: {
    title: '이번 주 한눈에',
    activeDays: '활동한 날',
    daysCount: '{count}일',
    ofSeven: '7일 중',
    longestFocus: '최장 집중',
    peakHour: '최고 시간대',
    avgFocus: '하루 평균 집중',
    perActiveDay: '활동한 날 기준',
    busiestDay: '가장 바쁜 날',
    none: '없음',
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
}
