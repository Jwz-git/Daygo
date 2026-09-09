/*
 * Error copy is keyed by the closed error-code table in
 * docs/05-interface-contract.md §5.4. The `message` half of a
 * `daygo:<code>: <message>` error is explicitly NOT localised by the backend,
 * so the UI shows the localised code copy here and keeps the raw message for
 * diagnostics only.
 */
export default {
  code: {
    invalidArgument: '请求参数无效',
    notFound: '未找到对应内容',
    notCaptureOwner: '当前进程不是捕获所有者',
    permissionDenied: '缺少所需的系统权限',
    providerNotConfigured: '尚未配置模型服务',
    providerFailed: '模型服务调用失败',
    nativeUnavailable: '原生能力当前不可用',
    mediaDecodeFailed: '媒体解码失败',
    conflict: '操作与当前状态冲突',
    databaseError: '数据库错误',
    canceled: '操作已取消',
    internal: '内部错误',
  },
  unknown: '发生未知错误',
}
