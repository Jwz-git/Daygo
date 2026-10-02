export default {
  title: 'Wöchentlich',
  developmentFixture: 'Beispieldaten · nur in der Entwicklung',
  navigation: {
    label: 'Wochennavigation',
    current: 'Diese Woche',
    backendRequired: 'Das Wechseln der Woche ist derzeit nicht möglich',
  },
  meta: {
    tracked: '{count} Min. aufgezeichnet',
  },
  intro: {
    eyebrow: 'Wochenrückblick',
    title: 'Wohin die Woche ging',
    description: 'Deine aufgezeichnete Zeit, dein Fokusrhythmus, die Verteilung über die Tage und die Kategorienmischung der Woche.',
  },
  state: {
    loading: {
      title: 'Diese Woche wird geladen',
      description: 'Einen Moment bitte.',
    },
    unavailable: {
      title: 'Wochenansicht ist derzeit nicht verfügbar',
      description: 'Versuche es später erneut oder starte Daygo neu.',
    },
    failure: {
      title: 'Diese Woche konnte nicht geladen werden',
      description: 'Deine Daten sind nicht betroffen. Du kannst es erneut versuchen.',
    },
    empty: {
      title: 'Diese Woche ist noch nichts aufgezeichnet',
      description: 'Dein Wochenrückblick erscheint hier, sobald Daygo Aktivität aufgezeichnet und aufbereitet hat.',
    },
  },
  scopeNote: 'Basiert auf deinen täglichen Aktivitätskarten.',
  charts: {
    distribution: {
      title: 'Wochenverteilung',
      total: 'Gesamt',
      aria: 'Zeitanteil pro Kategorie in dieser Woche',
    },
    context: {
      title: 'Kontextwechsel und Ablenkungen',
      shifts: 'Kontextwechsel',
      distractions: 'Ablenkungen',
      distribution: 'Über den Tag (10:00–18:00)',
      comparison: 'Pro Tag',
      insight: '{day} hatte die meisten Unterbrechungen: {shifts} Kontextwechsel und {distracted} Ablenkungen.',
      insightNone: 'Diese Woche gab es kein auffälliges Muster aus Wechseln oder Ablenkungen.',
      aria: 'Kontextwechsel und Ablenkungen pro Tag',
    },
    workflow: {
      title: 'Dein Arbeitsfluss diese Woche',
      total: 'Wochensumme',
      aria: 'Hauptkategorie für jeden Abschnitt jedes Tages',
    },
    heatmap: {
      title: 'Heatmap für Fokus und Ablenkung',
      focused: 'Fokussierte Arbeit',
      distracted: 'Abgelenkt',
      aria: 'Wie fokussiert oder abgelenkt jeder Abschnitt jedes Tages war',
    },
    treemap: {
      title: 'Meistgenutzt pro Kategorie',
      empty: 'Diese Woche gibt es noch keine App-Daten',
      aria: 'Apps mit der meisten Zeit in jeder Kategorie',
      // Dayflow's compact units ("6hr 4m", "+ 152m"), kept in English on purpose.
      hoursMinutes: '{hours}hr {minutes}m',
      hours: '{hours}hr',
      minutes: '{minutes}m',
    },
    sankey: {
      title: 'Zeit zwischen Kategorien und Apps',
      aria: 'Wie die Zeit dieser Woche von Kategorien in Apps floss',
    },
    tooltip: {
      shifts: '{count} Kontextwechsel',
      distractions: '{count} Ablenkungen',
      noRecord: 'Nichts erfasst',
      share: '{value} von {name}',
      change: '{value} ggü. Vorwoche',
      newThisWeek: 'Letzte Woche nicht genutzt',
      pinHint: 'Klicken, um die Hervorhebung festzuhalten; erneut klicken zum Lösen',
    },
    otherApp: 'Sonstige',
    otherCategory: 'Sonstige',
  },
}
