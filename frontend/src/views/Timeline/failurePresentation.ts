// Failure kinds are persisted by the Go analysis pipeline. Keep the source
// classification here so the track and inspector give the same explanation.
export type FailureSource = 'provider' | 'configuration' | 'application'

export interface FailurePresentation {
  source: FailureSource
  titleKey: string
  reasonKey: string
  actionKey: string
}

export function failurePresentation(kind: string): FailurePresentation {
  switch (kind) {
    case 'auth':
    case 'rate_limited':
    case 'network':
    case 'invalid_request':
    case 'invalid_output':
      return {
        source: 'provider',
        titleKey: 'timeline.failure.providerTitle',
        reasonKey: `timeline.failure.reason.${kind}`,
        actionKey: 'timeline.failure.providerAction',
      }
    case 'no_provider':
      return {
        source: 'configuration',
        titleKey: 'timeline.failure.configurationTitle',
        reasonKey: 'timeline.failure.reason.no_provider',
        actionKey: 'timeline.failure.configurationAction',
      }
    default:
      return {
        source: 'application',
        titleKey: 'timeline.failure.title',
        reasonKey: 'timeline.failure.reason.application',
        actionKey: 'timeline.failure.applicationAction',
      }
  }
}
