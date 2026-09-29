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
  overview: {
    eyebrow: 'Überblick',
    title: 'Zeit und Fokus',
    description: 'Fokuszeit schließt Ablenkung und Leerlauf aus; aufgezeichnete Zeit schließt System-Platzhalter aus.',
    focusAria: 'Fokus macht {value} der aufgezeichneten Zeit aus',
  },
  metric: {
    tracked: 'Aufgezeichnete Zeit',
    focused: 'Fokuszeit',
    other: 'Sonstige Zeit',
    focusRate: 'Fokusanteil',
  },
  categories: {
    eyebrow: 'Zusammensetzung',
    title: 'Kategorienverteilung',
    count: '{count} Kategorien',
    distributionAria: 'Anteil der aufgezeichneten Zeit je Kategorie in dieser Woche',
    total: 'Gesamt',
  },
  daily: {
    eyebrow: 'Muster',
    title: 'Tagesverlauf',
    hint: 'Eine Zeile pro Tag; jeder Block ist ein Kategorienabschnitt',
    idleTag: 'Leerlauf',
    noActivity: 'Keine Aktivität',
  },
  rhythm: {
    eyebrow: 'Rhythmus',
    title: 'Wochenrhythmus',
    focus: 'Fokus',
    idle: 'Leerlauf',
    chartAria: 'Fokus- und Leerlaufminuten je Tagesstunde',
    empty: 'Diese Woche gibt es noch nichts zusammenzufassen',
  },
  insights: {
    title: 'Diese Woche auf einen Blick',
    activeDays: 'Aktive Tage',
    daysCount: '{count} Tage',
    ofSeven: 'von 7 Tagen',
    longestFocus: 'Längster Fokus',
    peakHour: 'Stärkste Stunde',
    avgFocus: 'Ø Fokus pro Tag',
    perActiveDay: 'pro aktivem Tag',
    busiestDay: 'Aktivster Tag',
    none: 'Keine',
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
}
