export default {
  actionError: "The action failed. Please retry; your input has been kept.",
  retry: "Retry",
  working: "Working… You can stop at any time.",
  tooLong: "Messages must be no larger than 32 KiB. Please shorten your message.",

  title: "Chat",
  newConversation: "New chat",
  conversations: "Conversations",
  showSidebar: "Show sidebar",
  hideSidebar: "Hide sidebar",
  emptyConversations: "No conversations yet. Send a message to start one.",
  deleteConversation: "Delete this conversation",
  renameConversation: "Rename this conversation",
  rename: "Rename",
  renameTitlePlaceholder: "Enter a title for the conversation",
  renameTitleRequired: "A title is required",
  removeConfirm: "Delete this conversation?",
  unavailableTitle: "Chat is unavailable",
  unavailableDescription: "It needs to run inside the Daygo app with a database available.",
  provider: {
    label: "Provider",
    placeholder: "No provider selected",
  },
  model: {
    label: "Model",
    follow: "Follow provider ({model})",
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
  memory: {
    title: "Global instructions",
    hint: "Appended to every conversation's system prompt (like a CLAUDE.md).",
    placeholder: "e.g. Keep answers short; I track my time heavily…",
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
    subtitle: "Ask about your timeline, daily summary, weekly review, categories or cards",
    hints: [
      "\"What did I do today?\"",
      "\"Show me this week's category breakdown.\"",
      "\"Add a new category: Learning\"",
      "\"Check yesterday's journal.\"",
    ],
  },

  contextBar: {
    readonly: "Read-only",
  },

  dateDivider: {
    today: "Today",
    yesterday: "Yesterday",
  },
}
