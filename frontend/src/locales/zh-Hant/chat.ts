export default {
  actionError: '發生錯誤，請重試。你輸入的內容還在。',
  retry: '重試',
  working: '正在思考…',
  tooLong: '訊息太長了，請精簡後再傳送。',

  title: '對話',
  newConversation: '新增對話',
  showSidebar: '顯示側邊欄',
  hideSidebar: '隱藏側邊欄',
  emptyConversations: '還沒有對話。傳送第一則訊息開始。',
  deleteConversation: '刪除這個對話',
  renameConversation: '重新命名這個對話',
  rename: '重新命名',
  renameTitlePlaceholder: '輸入對話標題',
  renameTitleRequired: '標題不能是空的',
  removeConfirm: '要刪除這個對話嗎？',
  unavailableTitle: '對話暫時無法使用',
  unavailableDescription: '請稍後再試，或重新啟動 Daygo。',
  provider: {
    label: 'AI 服務',
    placeholder: '未選擇 AI 服務',
  },
  model: {
    label: '模型',
    follow: '預設（{model}）',
  },
  composer: {
    placeholder: '輸入訊息…（Enter 傳送）',
    send: '傳送',
    cancel: '停止',
  },
  status: {
    failed: '失敗',
    canceled: '已取消',
  },
  failure: {
    no_provider: '還沒有設定 AI 服務，請先在設定中新增。',
    no_provider_selected: '此對話還沒有選擇 AI 服務。',
    canceled: '已取消。',
    authentication: '金鑰被拒絕，請在設定中檢查。',
    rate_limited: '服務正在限流，請稍後再試。',
    timeout: '服務沒有在預期時間內回應。',
    dns: '無法解析該服務的位址，請在設定中檢查。',
    connection: '無法連線到該服務，請確認它正在執行且網路可達。',
    tls: '與該服務的安全連線失敗，請檢查位址與憑證。',
    network: '請求無法送達該服務，請檢查網路。',
    invalid_request: '服務拒絕了該請求，請檢查位址與模型是否正確。',
    unsupported_feature: '目前的模型不支援對話所需的結構化輸出。',
    invalid_output: '無法解析服務回傳的內容，請確認模型是否相符。',
    tool_budget: '本輪工具呼叫次數已用完，已終止。',
    internal: 'Daygo 內部發生錯誤，請重試。',
  },
  memory: {
    title: '全域指令',
    hint: '寫下希望 Daygo 在每次對話中都記住的要求。',
    placeholder: '例如：回答盡量簡潔；叫我小王…',
    save: '儲存',
    saved: '已儲存',
  },
  loadError: '對話載入失敗',

  drawer: {
    conversations: '對話',
    memory: '全域指令',
    newChat: '新增對話',
    today: '今天',
    yesterday: '昨天',
    older: '更早',
    untitled: '未命名對話',
  },

  bubble: {
    copy: '複製',
    copied: '已複製',
    roleAssistant: 'Daygo',
  },

  welcome: {
    title: '你好，有什麼可以幫你的？',
    subtitle: '可以問我時間軸、日報、週報裡的任何事',
    hints: [
      '我今天做了什麼？',
      '本週工作分類佔比？',
      '新增一個分類：學習',
      '幫我看看昨天的日記',
    ],
  },

  dateDivider: {
    today: '今天',
    yesterday: '昨天',
  },
}
