export default {
  actionError: '出错了，请重试。你输入的内容还在。',
  retry: '重试',
  working: '正在思考…',
  tooLong: '消息太长了，请精简后再发送。',

  title: '对话',
  newConversation: '新对话',
  showSidebar: '显示侧栏',
  hideSidebar: '隐藏侧栏',
  emptyConversations: '还没有对话。发送第一条消息开始。',
  deleteConversation: '删除该对话',
  renameConversation: '重命名该对话',
  rename: '重命名',
  renameTitlePlaceholder: '输入对话标题',
  renameTitleRequired: '标题不能为空',
  removeConfirm: '删除这个对话？',
  unavailableTitle: '对话暂时不可用',
  unavailableDescription: '请稍后再试，或重启 Daygo。',
  provider: {
    label: 'AI 服务',
    placeholder: '未选择 AI 服务',
  },
  model: {
    label: '模型',
    follow: '默认（{model}）',
  },
  composer: {
    placeholder: '输入消息…（Enter 发送）',
    send: '发送',
    cancel: '停止',
  },
  status: {
    failed: '失败',
    canceled: '已取消',
  },
  failure: {
    no_provider: '还没有配置 AI 服务，请先在设置中添加。',
    no_provider_selected: '该会话还没有选择 AI 服务。',
    canceled: '已取消。',
    authentication: '密钥被拒绝，请在设置中检查。',
    rate_limited: '服务正在限流，请稍后再试。',
    timeout: '服务没有在预期时间内响应。',
    dns: '无法解析该服务的地址，请在设置中检查。',
    connection: '无法连接到该服务，请确认它正在运行且网络可达。',
    tls: '与该服务的安全连接失败，请检查地址和证书。',
    network: '请求无法送达该服务，请检查网络。',
    invalid_request: '服务拒绝了该请求，请检查地址和模型是否正确。',
    unsupported_feature: '当前模型不支持对话所需的结构化输出。',
    invalid_output: '无法解析服务返回的内容，请确认模型是否匹配该接口。',
    tool_budget: '本轮工具调用次数已用完，已终止。',
    internal: 'Daygo 内部出错，请重试。',
  },
  memory: {
    title: '全局指令',
    hint: '写下希望 Daygo 在每次对话中都记住的要求。',
    placeholder: '例如：回答尽量简洁；称呼我为小王…',
    save: '保存',
    saved: '已保存',
  },
  loadError: '对话加载失败',

  // New: drawer
  drawer: {
    conversations: '对话',
    memory: '全局指令',
    newChat: '新建对话',
    today: '今天',
    yesterday: '昨天',
    older: '更早',
    untitled: '未命名对话',
  },

  // New: bubble
  bubble: {
    copy: '复制',
    copied: '已复制',
    roleAssistant: 'Daygo',
  },

  // New: welcome
  welcome: {
    title: '你好，有什么可以帮你的？',
    subtitle: '可以问我时间线、日报、周报里的任何事',
    hints: [
      '今天我做了什么？',
      '本周工作分类占比？',
      '添加一个新分类：学习',
      '帮我看看昨天的日记',
    ],
  },

  // New: context bar
  // New: date divider
  dateDivider: {
    today: '今天',
    yesterday: '昨天',
  },
}
