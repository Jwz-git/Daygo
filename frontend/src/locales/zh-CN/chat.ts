export default {
  actionError: '操作失败，请重试。输入内容已保留。',
  retry: '重试',
  working: '正在处理，可随时停止…',
  tooLong: '消息不能超过 32 KiB，请缩短后发送。',

  title: '对话',
  newConversation: '新对话',
  conversations: '对话列表',
  showSidebar: '显示侧栏',
  hideSidebar: '隐藏侧栏',
  emptyConversations: '还没有对话。发送第一条消息开始。',
  deleteConversation: '删除该对话',
  renameConversation: '重命名该对话',
  rename: '重命名',
  renameTitlePlaceholder: '输入对话标题',
  renameTitleRequired: '标题不能为空',
  removeConfirm: '删除这个对话？',
  unavailableTitle: '对话功能不可用',
  unavailableDescription: '需要在 Daygo 应用内运行，且数据库可用。',
  provider: {
    label: '供应商',
    placeholder: '未选择供应商',
  },
  model: {
    label: '模型',
    follow: '跟随供应商（{model}）',
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
  memory: {
    title: '全局指令',
    hint: '附加到每个对话的系统提示（类似 CLAUDE.md）。',
    placeholder: '例如：回答保持简洁；我是时间跟踪工具的重度用户…',
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
    subtitle: '问我关于你的时间线、日报、周报、分类或卡片',
    hints: [
      '「今天我做了什么？」',
      '「本周工作分类占比？」',
      '「添加一个新分类：学习」',
      '「帮我看看昨天的日记」',
    ],
  },

  // New: context bar
  contextBar: {
    readonly: '只读',
  },

  // New: date divider
  dateDivider: {
    today: '今天',
    yesterday: '昨天',
  },
}
