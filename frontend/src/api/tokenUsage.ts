import { GetTokenUsage } from '../../wailsjs/go/app/Backend'

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
  return GetTokenUsage(period, day)
}
