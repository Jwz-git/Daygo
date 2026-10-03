export default {
  actionError: '문제가 발생했습니다. 다시 시도해 주세요. 입력한 내용은 그대로 남아 있습니다.',
  retry: '다시 시도',
  working: '생각하는 중…',
  tooLong: '메시지가 너무 깁니다. 줄여서 다시 보내 주세요.',

  title: '채팅',
  newConversation: '새 채팅',
  showSidebar: '사이드바 표시',
  hideSidebar: '사이드바 숨기기',
  emptyConversations: '아직 채팅이 없습니다. 메시지를 보내 시작하세요.',
  deleteConversation: '이 채팅 삭제',
  renameConversation: '이 채팅 이름 바꾸기',
  rename: '이름 바꾸기',
  renameTitlePlaceholder: '채팅 제목 입력',
  renameTitleRequired: '제목은 필수입니다',
  removeConfirm: '이 채팅을 삭제할까요?',
  unavailableTitle: '지금은 채팅을 쓸 수 없습니다',
  unavailableDescription: '나중에 다시 시도하거나 Daygo를 다시 시작하세요.',
  provider: {
    label: 'AI 서비스',
    placeholder: 'AI 서비스를 선택하지 않음',
  },
  model: {
    label: '모델',
    follow: '기본값({model})',
  },
  composer: {
    placeholder: '메시지 입력…(Enter로 전송)',
    send: '보내기',
    cancel: '중지',
  },
  status: {
    failed: '실패',
    canceled: '취소됨',
  },
  failure: {
    no_provider: 'AI 서비스가 설정되어 있지 않습니다. 설정에서 추가하세요.',
    no_provider_selected: '이 대화에는 AI 서비스가 선택되어 있지 않습니다.',
    canceled: '취소되었습니다.',
    authentication: 'API 키가 거부되었습니다. 설정에서 확인하세요.',
    rate_limited: '서비스가 요청을 제한하고 있습니다. 잠시 후 다시 시도하세요.',
    timeout: '서비스가 제때 응답하지 않았습니다.',
    dns: '서비스 주소를 확인할 수 없습니다. 설정에서 확인하세요.',
    connection: '서비스에 연결할 수 없습니다. 실행 중인지, 접근 가능한지 확인하세요.',
    tls: '서비스와의 보안 연결에 실패했습니다. 주소와 인증서를 확인하세요.',
    network: '요청이 서비스에 도달하지 못했습니다. 네트워크를 확인하세요.',
    invalid_request: '서비스가 요청을 거부했습니다. 주소와 모델을 확인하세요.',
    unsupported_feature: '이 모델은 대화에 필요한 구조화 출력을 지원하지 않습니다.',
    invalid_output: '서비스 응답을 해석할 수 없습니다. 모델이 이 API에 맞는지 확인하세요.',
    tool_budget: '이번 턴의 도구 호출 한도를 모두 사용해 중단되었습니다.',
    internal: 'Daygo 내부 오류가 발생했습니다. 다시 시도하세요.',
  },
  memory: {
    title: '전역 지침',
    hint: 'Daygo가 모든 대화에서 기억했으면 하는 내용을 적어 두세요.',
    placeholder: '예: 답변은 짧게; 저는 김 사원이라고 불러 주세요…',
    save: '저장',
    saved: '저장됨',
  },
  loadError: '채팅을 불러오지 못했습니다',

  drawer: {
    conversations: '채팅',
    memory: '전역 지침',
    newChat: '새 채팅',
    today: '오늘',
    yesterday: '어제',
    older: '이전',
    untitled: '제목 없는 채팅',
  },

  bubble: {
    copy: '복사',
    copied: '복사됨',
    roleAssistant: 'Daygo',
  },

  welcome: {
    title: '안녕하세요, 무엇을 도와드릴까요?',
    subtitle: '타임라인, 데일리, 위클리에 대해 무엇이든 물어보세요',
    hints: [
      '오늘 내가 한 일은?',
      '이번 주 카테고리 비중은?',
      '새 카테고리 추가: 학습',
      '어제 저널을 보여줘',
    ],
  },

  dateDivider: {
    today: '오늘',
    yesterday: '어제',
  },
}
