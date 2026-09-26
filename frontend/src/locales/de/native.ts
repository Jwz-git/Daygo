export default {
  // Surfaces the native layer renders outside the webview: the frontend pushes
  // this bundle at startup and on every language change (docs/05 §5.5.1).
  applicationPicker: {
    title: 'Programm auswählen',
    filterExecutable: 'Windows-Programme (*.exe)',
  },
  updater: {
    ownerRequired: 'Installiere Updates im Daygo-Fenster, das gerade aufzeichnet.',
  },
  applicationMenu: {
    hide: "Daygo ausblenden",
    hideOthers: "Andere ausblenden",
    showAll: "Alle einblenden",
    background: "Im Hintergrund weiter aufzeichnen",
    edit: "Bearbeiten",
    undo: "Widerrufen",
    redo: "Wiederholen",
    cut: "Ausschneiden",
    copy: "Kopieren",
    paste: "Einsetzen",
    pasteMatch: "Einsetzen und Stil anpassen",
    delete: "Löschen",
    selectAll: "Alles auswählen",
    speech: "Sprachausgabe",
    startSpeaking: "Sprachausgabe starten",
    stopSpeaking: "Sprachausgabe stoppen",
    window: "Fenster",
    minimize: "Minimieren",
    zoom: "Vergrößern",
    fullScreen: "Vollbild",
  },
}
