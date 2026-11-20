import { GetTokenUsage } from '../../bindings/github.com/Jwz-git/Daygo/internal/app/backend'

export interface TokenUsageBucket {
  startTs: number
  endTs: number
  inputTokens: number
  outputTokens: number
  calls: number
  unknownCalls: number
}
export interface TokenUsage {
  period: string
  timeZone: string
  buckets: TokenUsageBucket[]
}
export async function getTokenUsage(period: 'day' | 'week', day: string): Promise<TokenUsage> {
  const value = await GetTokenUsage(period, day)
  return { ...value, buckets: value.buckets ?? [] }
}
