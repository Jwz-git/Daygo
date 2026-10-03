export default {
  actionError: 'Etwas ist schiefgelaufen. Versuche es erneut – deine Nachricht ist noch da.',
  retry: 'Erneut versuchen',
  working: 'Denkt nach…',
  tooLong: 'Diese Nachricht ist zu lang. Kürze sie und versuche es erneut.',

  title: 'Chat',
  newConversation: 'Neuer Chat',
  showSidebar: 'Seitenleiste einblenden',
  hideSidebar: 'Seitenleiste ausblenden',
  emptyConversations: 'Noch keine Unterhaltungen. Sende eine Nachricht, um eine zu beginnen.',
  deleteConversation: 'Diese Unterhaltung löschen',
  renameConversation: 'Diese Unterhaltung umbenennen',
  rename: 'Umbenennen',
  renameTitlePlaceholder: 'Gib einen Titel für die Unterhaltung ein',
  renameTitleRequired: 'Ein Titel ist erforderlich',
  removeConfirm: 'Diese Unterhaltung löschen?',
  unavailableTitle: 'Chat ist derzeit nicht verfügbar',
  unavailableDescription: 'Versuche es später erneut oder starte Daygo neu.',
  provider: {
    label: 'KI-Dienst',
    placeholder: 'Kein KI-Dienst ausgewählt',
  },
  model: {
    label: 'Modell',
    follow: 'Standard ({model})',
  },
  composer: {
    placeholder: 'Nachricht eingeben… (Enter zum Senden)',
    send: 'Senden',
    cancel: 'Stoppen',
  },
  status: {
    failed: 'Fehlgeschlagen',
    canceled: 'Abgebrochen',
  },
  failure: {
    no_provider: 'Es ist kein KI-Dienst eingerichtet. Füge einen in den Einstellungen hinzu.',
    no_provider_selected: 'Für diese Unterhaltung ist kein KI-Dienst ausgewählt.',
    canceled: 'Abgebrochen.',
    authentication: 'Der API-Schlüssel wurde abgelehnt. Prüfe ihn in den Einstellungen.',
    rate_limited: 'Der Dienst begrenzt gerade Anfragen. Versuche es gleich noch einmal.',
    timeout: 'Der Dienst hat nicht rechtzeitig geantwortet.',
    dns: 'Die Adresse des Dienstes ließ sich nicht auflösen. Prüfe sie in den Einstellungen.',
    connection: 'Es konnte keine Verbindung zum Dienst hergestellt werden. Prüfe, ob er läuft und erreichbar ist.',
    tls: 'Die sichere Verbindung zum Dienst ist fehlgeschlagen. Prüfe Adresse und Zertifikat.',
    network: 'Die Anfrage konnte den Dienst nicht erreichen. Prüfe dein Netzwerk.',
    invalid_request: 'Der Dienst hat die Anfrage abgelehnt. Prüfe Adresse und Modell.',
    unsupported_feature: 'Dieses Modell unterstützt die für den Chat nötige strukturierte Ausgabe nicht.',
    invalid_output: 'Die Antwort des Dienstes war nicht lesbar. Prüfe, ob das Modell zu dieser API passt.',
    tool_budget: 'Dieser Zug hat alle Werkzeugaufrufe aufgebraucht und wurde beendet.',
    internal: 'In Daygo ist ein Fehler aufgetreten. Versuche es erneut.',
  },
  memory: {
    title: 'Globale Anweisungen',
    hint: 'Dinge, die Daygo in jeder Unterhaltung berücksichtigen soll.',
    placeholder: 'z. B. Antworte kurz; nenn mich Sam…',
    save: 'Sichern',
    saved: 'Gesichert',
  },
  loadError: 'Unterhaltungen konnten nicht geladen werden',

  drawer: {
    conversations: 'Unterhaltungen',
    memory: 'Globale Anweisungen',
    newChat: 'Neuer Chat',
    today: 'Heute',
    yesterday: 'Gestern',
    older: 'Älter',
    untitled: 'Unbenannter Chat',
  },

  bubble: {
    copy: 'Kopieren',
    copied: 'Kopiert',
    roleAssistant: 'Daygo',
  },

  welcome: {
    title: 'Hallo, wie kann ich helfen?',
    subtitle: 'Frag mich alles über deine Zeitleiste, deine Standups oder den Wochenrückblick',
    hints: [
      'Was habe ich heute gemacht?',
      'Zeig mir die Kategorienverteilung dieser Woche.',
      'Füge eine neue Kategorie hinzu: Lernen',
      'Sieh dir das Journal von gestern an.',
    ],
  },

  dateDivider: {
    today: 'Heute',
    yesterday: 'Gestern',
  },
}
