export default {
  // Surfaces the native layer renders outside the webview: the frontend pushes
  // this bundle at startup and on every language change (docs/05 §5.5.1).
  applicationPicker: {
    title: 'Choose an application',
    filterExecutable: 'Windows applications (*.exe)',
  },
  updater: {
    ownerRequired: 'Install updates from the Daygo window that is currently recording.',
  },
  applicationMenu: {
    hide: "Hide Daygo",
    hideOthers: "Hide Others",
    showAll: "Show All",
    background: "Continue recording in background",
    edit: "Edit",
    undo: "Undo",
    redo: "Redo",
    cut: "Cut",
    copy: "Copy",
    paste: "Paste",
    pasteMatch: "Paste and Match Style",
    delete: "Delete",
    selectAll: "Select All",
    speech: "Speech",
    startSpeaking: "Start Speaking",
    stopSpeaking: "Stop Speaking",
    window: "Window",
    minimize: "Minimize",
    zoom: "Zoom",
    fullScreen: "Full Screen",
  },
}
