export default {
  title: 'Settings',
  nav: {
    storage: 'Storage',
    privacy: 'Privacy',
    providers: 'Providers',
    agentAccess: 'MCP / CLI',
    dataExport: 'Export',
    other: 'Other',
  },
  section: {
    storageDescription:
      'Recording status and permission, capture quality, disk limits.',
    privacyDescription: 'Blocked apps never reach a screenshot.',
    providersDescription:
      'Primary and secondary providers, connection tests, prompt overrides.',
    agentAccessDescription:
      'Let local CLI clients connect to the Agent Bridge.',
    dataExportDescription:
      'Export a date range as Markdown, or reprocess a day.',
    otherDescription: 'Appearance, launch, telemetry and language.',
  },
  appearance: {
    theme: 'Theme',
    themeDescription:
      'With "Follow system" the window tracks the macOS light/dark switch live.',
    themeOption: {
      system: 'Follow system',
      light: 'Light',
      dark: 'Dark',
    },
  },
  language: {
    interface: 'Interface language',
    interfaceDescription: 'Applies to all copy in this app.',
    followSystem: 'Follow system',
    output: 'Model output language',
    outputDescription:
      'The language the model writes card titles and summaries in. Independent of the interface language.',
  },
  recognition: {
    title: 'Recognition enhancement',
    hint: 'Increases token usage',
    unavailable: 'Runs inside the Daygo app',
  },
  providers: {
    description:
      'Custom providers only for now: a protocol, a base URL and a model id is all it takes.',
    empty: 'No providers yet.',
    add: 'Add provider',
    removeConfirm: 'Delete "{name}"?',
    protocol: {
      label: 'Protocol',
      openai: 'OpenAI-compatible',
      openai_responses: 'OpenAI Responses',
      anthropic: 'Anthropic',
    },
    form: {
      addTitle: 'Add provider',
      editTitle: 'Edit provider',
      name: 'Name',
      namePlaceholder: 'e.g. Work gateway',
      endpoint: 'Base URL',
      model: 'Model',
      modelPlaceholder: {
        openai: 'e.g. gpt-4o-mini',
        openai_responses: 'e.g. gpt-4o',
        anthropic: 'e.g. claude-sonnet-4-5',
      },
      apiKey: 'API key',
      apiKeyPlaceholder: 'Paste the key',
      apiKeyKeepHint: 'Leave blank to keep the current key.',
      httpWarning:
        'This address is not encrypted (HTTP): your key and screenshots travel the network in plain text.',
    },
    routing: {
      title: 'Call order',
      description: 'The secondary provider is used when the primary call fails.',
      primary: 'Primary',
      secondary: 'Secondary',
      none: 'None',
      primaryBadge: '1st',
      secondaryBadge: '2nd',
    },
    secret: {
      configured: 'Key entered this session',
      missing: 'No key',
      clear: 'Clear key',
      sessionOnlyTitle: 'Keys are never written to disk',
      sessionOnly:
        'A key stays in this process memory and has to be entered again after a restart. Persistence arrives through SetProviderSecret, which writes to the system keychain rather than to browser storage.',
    },
    error: {
      required: 'Required',
      invalidUrl: 'Needs a full http:// or https:// address',
    },
    test: {
      run: 'Test',
      running: 'Testing…',
      passed: 'Pass · {model} · {latency} ms · text/image/structured output',
      error: {
        authentication: 'The key is invalid or expired',
        rate_limited: 'Rate limited — try again shortly',
        timeout: 'Timed out after 30 seconds',
        unavailable: 'Could not reach this address',
        invalid_request: 'The request was rejected — check the model and address',
        unsupported_feature: 'This service does not support structured output',
        invalid_output: 'The model failed the image and echo check',
        canceled: 'The request was canceled',
        wails_unavailable: 'The connection test runs inside the Daygo app',
      },
    },
  },
}
