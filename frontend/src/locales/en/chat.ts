export default {
  actionError: 'Something went wrong. Please try again — your message is still here.',
  retry: "Retry",
  working: 'Thinking…',
  tooLong: 'That message is too long. Please shorten it and try again.',

  title: "Chat",
  newConversation: "New chat",
  showSidebar: "Show sidebar",
  hideSidebar: "Hide sidebar",
  emptyConversations: "No conversations yet. Send a message to start one.",
  deleteConversation: "Delete this conversation",
  renameConversation: "Rename this conversation",
  rename: "Rename",
  renameTitlePlaceholder: "Enter a title for the conversation",
  renameTitleRequired: "A title is required",
  removeConfirm: "Delete this conversation?",
  unavailableTitle: 'Chat is unavailable right now',
  unavailableDescription: 'Try again later, or restart Daygo.',
  provider: {
    label: 'AI service',
    placeholder: 'No AI service selected',
  },
  model: {
    label: "Model",
    follow: 'Default ({model})',
  },
  composer: {
    placeholder: "Type a message… (Enter to send)",
    send: "Send",
    cancel: "Stop",
  },
  status: {
    failed: "Failed",
    canceled: "Canceled",
  },
  failure: {
    no_provider: "No AI service is configured. Add one in Settings.",
    no_provider_selected: "This conversation has no AI service selected.",
    canceled: "Canceled.",
    authentication: "The API key was rejected. Check it in Settings.",
    rate_limited: "The service is rate limiting requests. Try again shortly.",
    timeout: "The service did not respond in time.",
    dns: "The service's address could not be resolved. Check it in Settings.",
    connection: "Could not connect to the service. Check that it is running and reachable.",
    tls: "The secure connection to the service failed. Check the address and its certificate.",
    network: "The request could not reach the service. Check your network.",
    invalid_request: "The service rejected the request. Check the address and model.",
    unsupported_feature: "This model does not support the structured output chat needs.",
    invalid_output: "The service's reply could not be read. Check that the model fits this API.",
    tool_budget: "This turn used all of its tool calls and was stopped.",
    internal: "Something went wrong inside Daygo. Try again.",
  },
  memory: {
    title: "Global instructions",
    hint: 'Things you want Daygo to remember in every conversation.',
    placeholder: 'e.g. Keep answers short; call me Sam…',
    save: "Save",
    saved: "Saved",
  },
  loadError: "Failed to load conversations",

  drawer: {
    conversations: "Conversations",
    memory: "Global instructions",
    newChat: "New chat",
    today: "Today",
    yesterday: "Yesterday",
    older: "Older",
    untitled: "Untitled chat",
  },

  bubble: {
    copy: "Copy",
    copied: "Copied",
    roleAssistant: "Daygo",
  },

  welcome: {
    title: "Hi, how can I help you?",
    subtitle: 'Ask me anything about your timeline, standups, or weekly review',
    hints: [
      'What did I do today?',
      "Show me this week's category breakdown.",
      'Add a new category: Learning',
      "Check yesterday's journal.",
    ],
  },

  dateDivider: {
    today: "Today",
    yesterday: "Yesterday",
  },
}
