/**
 * Starting points for a provider's User-Agent override; the field stays free
 * text, so these are fill-in suggestions, not a closed list. Version numbers
 * are a snapshot and will drift.
 */
export interface UserAgentPreset {
  /** i18n key suffix under settings.providers.form.userAgentPreset. */
  key: string
  /** The literal header value. */
  value: string
}

export const USER_AGENT_PRESETS: UserAgentPreset[] = [
  { key: 'claudeCli', value: 'claude-cli/2.1.161 (external, cli)' },
  {
    key: 'chromeMac',
    value: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/154.0.0.0 Safari/537.36',
  },
]
