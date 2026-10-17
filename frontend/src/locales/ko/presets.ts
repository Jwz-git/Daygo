export default {
  archetype: {
    label: '원형',
    hint: '3D 세계에서 이 카테고리가 어떤 건물과 풍경이 될지 정합니다.',
    forge: '창작',
    sanctum: '사고',
    nexus: '연결',
    camp: '휴식',
    order: '정돈',
  },
  profession: {
    softwareEngineer: {
      name: '소프트웨어 엔지니어',
      categories: {
        coding: { name: '코딩 / 디버깅', details: 'IDE나 터미널에서 코드 작성, 리팩터링, 수정' },
        codeReview: { name: '코드 리뷰', details: 'PR 리뷰, diff 읽기, 코멘트 남기기' },
        research: { name: '리서치', details: '문서와 Stack Overflow 읽기, 도구와 API 살펴보기, 설계 문서나 기술 사양 작성' },
        communication: { name: '커뮤니케이션', details: '회의, 스탠드업, Slack, 이메일, 화상 통화, 메시지, 동기화 미팅' },
      },
    },
    founderExecutive: {
      name: '창업자 / 경영진',
      categories: {
        building: { name: '엔지니어링 / 제품', details: '코딩, 디자인, 기능 출시 등 직접 만드는 일' },
        strategy: { name: '리서치와 전략', details: '경쟁 조사, 포지셔닝, 깊이 생각하기, 투자자 미팅 준비' },
        insights: { name: '데이터와 인사이트', details: '대시보드, 리텐션 데이터, 퍼널, 재무' },
        communication: { name: '커뮤니케이션', details: '팀 싱크, 투자자 통화, 사용자 데모, 채용' },
      },
    },
    designer: {
      name: '디자이너',
      categories: {
        design: { name: '디자인', details: '프로토타입, UI 컴포넌트, 사용자 흐름, 비주얼 디자인, 핸드오프 사양' },
        research: { name: '리서치', details: '디자인 패턴 탐색, 경쟁사 분석, 사용자 연구, 지표 검토' },
        communication: { name: '커뮤니케이션', details: '디자인 리뷰, 스탠드업, 크리틱 세션, 콘셉트 발표' },
      },
    },
    student: {
      name: '학생',
      categories: {
        studying: { name: '공부', details: '강의, 읽기, 슬라이드 복습, 암기 카드, 수업 자료' },
        assignments: { name: '과제', details: '리포트, 문제 풀이, 코딩 프로젝트, 실험 보고서' },
        communication: { name: '커뮤니케이션', details: '스터디 그룹, 면담 시간, 단체 채팅, 교수님께 보내는 이메일' },
      },
    },
    productManager: {
      name: '프로덕트 매니저',
      categories: {
        specs: { name: '기획과 계획', details: 'PRD, 로드맵, 백로그 정리, 스프린트 계획, 티켓' },
        research: { name: '리서치와 분석', details: '사용자 조사, 지표 검토, 경쟁 분석, A/B 테스트' },
        communication: { name: '커뮤니케이션', details: '스탠드업, 이해관계자 싱크, 디자인 리뷰, 개발팀 점검' },
      },
    },
    dataScientist: {
      name: '데이터 과학자',
      categories: {
        analysis: { name: '분석과 모델링', details: '노트북, 통계 분석, ML 학습, 데이터 탐색' },
        dataEngineering: { name: '데이터 엔지니어링', details: 'SQL 쿼리, 파이프라인, 데이터 정제, ETL 스크립트' },
        research: { name: '리서치', details: '논문과 문서 읽기, 새로운 방법과 도구 탐색' },
        communication: { name: '커뮤니케이션', details: '결과 발표, 이해관계자 싱크, 팀 토론' },
      },
    },
    other: {
      name: '기타',
      categories: {
        work: { name: '업무', details: '더 구체적인 카테고리에 맞지 않는 집중 업무와 직무' },
        communication: { name: '커뮤니케이션', details: '회의, 스탠드업, Slack, 이메일, 화상 통화, 메시지, 동기화 미팅' },
      },
    },
  },
  common: {
    distraction: { name: '산만함', details: '목적 없는 탐색과 수동적 소비: 소셜 미디어 피드, 무심코 보는 동영상, 의미 없는 스크롤, 뚜렷한 의도 없는 오락, 게임' },
    personal: { name: '개인', details: '목적이 분명한 비업무 활동: 가족과 지인 연락, 재정 관리, 여행 예약, 볼일, 생활 잡무, 취미' },
  },
}
