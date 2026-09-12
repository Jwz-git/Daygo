export default {
  state: {
    loading: '正在读取',
    unknown: '状态未知',
    idle: '录制已关闭',
    starting: '正在启动',
    capturing: '正在记录',
    paused: '录制已暂停',
  },
  action: {
    start: '开始',
    pause: '暂停',
    resume: '继续',
    stop: '停止',
  },
  working: '处理中…',
  notOwner: '另一个 Daygo 实例正在管理录制。',
  permissionRequired: '需要允许屏幕录制后才能开始。',
  actionFailed: '操作没有完成，已保留最后确认的状态。',
}
