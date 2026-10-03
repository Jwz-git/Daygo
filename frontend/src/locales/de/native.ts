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
  journalReminder: {
    title: 'Zeit fürs Tagebuch',
    body: 'Nimm dir ein paar Minuten, um den heutigen Fortschritt und den Plan für morgen festzuhalten.',
  },
  plan: {
    startTitle: "Beginnt jetzt: {title}",
    startBody: "Geplant für {start}–{end}",
    distractionTitle: "Du wirst abgelenkt",
    distractionBody: "{minutes} Minuten Ablenkung während „{title}“.",
    dayDistractionTitle: "Heutige Ablenkung über dem Limit",
    dayDistractionBody: "Heute {minutes} Minuten Ablenkung; das Limit liegt bei {limit}.",
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
