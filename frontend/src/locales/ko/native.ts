export default {
  // Surfaces the native layer renders outside the webview: the frontend pushes
  // this bundle at startup and on every language change (docs/05 §5.5.1).
  applicationPicker: {
    title: '응용 프로그램 선택',
    filterExecutable: 'Windows 응용 프로그램 (*.exe)',
  },
  updater: {
    ownerRequired: '지금 기록 중인 Daygo 창에서 업데이트를 설치하세요.',
  },
  applicationMenu: {
    hide: "Daygo 가리기",
    hideOthers: "다른 앱 가리기",
    showAll: "모두 보기",
    background: "백그라운드에서 계속 기록",
    edit: "편집",
    undo: "실행 취소",
    redo: "다시 실행",
    cut: "잘라내기",
    copy: "복사",
    paste: "붙여넣기",
    pasteMatch: "붙여넣고 스타일 일치",
    delete: "삭제",
    selectAll: "모두 선택",
    speech: "말하기",
    startSpeaking: "말하기 시작",
    stopSpeaking: "말하기 중단",
    window: "윈도우",
    minimize: "최소화",
    zoom: "확대／축소",
    fullScreen: "전체 화면",
  },
}
