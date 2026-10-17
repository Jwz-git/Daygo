export default {
  discardNote: 'Ungespeicherte Änderungen gehen verloren.',
  apply: 'Anwenden',
  cancel: 'Abbrechen',
  applying: 'Wird angewendet …',
  failed: 'Konnte nicht angewendet werden. Deine Kategorien sind unverändert.',
  palette: {
    title: 'Farbpaletten',
    hint: 'Wähle eine Palette, um alle Kategorien der Reihe nach neu einzufärben.',
    confirmBody: 'Färbt alle Kategorien mit „{name}“ neu ein.',
    names: {
      daygoClassic: 'Daygo Klassik',
      forestMoss: 'Waldmoos',
      cyberNeon: 'Cyber-Neon',
      nordicMist: 'Nordischer Nebel',
      sunsetClay: 'Abendton',
    },
  },
  archetype: {
    label: 'Archetyp',
    hint: 'Bestimmt, zu welchen Gebäuden und welcher Landschaft diese Kategorie in der 3D-Welt wird.',
    forge: 'Erschaffen',
    sanctum: 'Denken',
    nexus: 'Verbinden',
    camp: 'Erholen',
    order: 'Ordnen',
  },
  profession: {
    softwareEngineer: {
      name: 'Softwareentwicklung',
      categories: {
        coding: { name: 'Programmieren / Debuggen', details: 'Code in IDE oder Terminal schreiben, umbauen und reparieren' },
        codeReview: { name: 'Code-Review', details: 'PRs prüfen, Diffs lesen und Kommentare hinterlassen' },
        research: { name: 'Recherche', details: 'Doku und Stack Overflow lesen, Werkzeuge und APIs erkunden sowie Design-Dokumente oder technische Spezifikationen schreiben' },
        communication: { name: 'Kommunikation', details: 'Meetings, Standups, Slack, E-Mail, Videoanrufe, Nachrichten und Abstimmungsrunden' },
      },
    },
    founderExecutive: {
      name: 'Gründung / Führung',
      categories: {
        building: { name: 'Engineering / Produkt', details: 'Programmieren, Gestaltung, Features ausliefern und selbst mitbauen' },
        strategy: { name: 'Recherche & Strategie', details: 'Wettbewerbsanalyse, Positionierung, gründliches Nachdenken und Vorbereitung auf Investoren' },
        insights: { name: 'Daten & Erkenntnisse', details: 'Dashboards, Bindungsdaten, Funnels und Finanzen' },
        communication: { name: 'Kommunikation', details: 'Team-Abstimmungen, Investorengespräche, Nutzerdemos und Recruiting' },
      },
    },
    designer: {
      name: 'Design',
      categories: {
        design: { name: 'Gestaltung', details: 'Prototypen, UI-Komponenten, Nutzerflüsse, visuelles Design und Übergabespezifikationen' },
        research: { name: 'Recherche', details: 'Muster sammeln, Wettbewerber prüfen, Nutzerstudien und Kennzahlen ansehen' },
        communication: { name: 'Kommunikation', details: 'Design-Reviews, Standups, Kritikrunden und Konzepte präsentieren' },
      },
    },
    student: {
      name: 'Studium',
      categories: {
        studying: { name: 'Lernen', details: 'Vorlesungen, Lektüre, Folien wiederholen, Karteikarten und Kursmaterial' },
        assignments: { name: 'Aufgaben', details: 'Hausarbeiten, Übungsblätter, Programmierprojekte und Laborberichte' },
        communication: { name: 'Kommunikation', details: 'Lerngruppen, Sprechstunden, Gruppenchats und E-Mails an Lehrende' },
      },
    },
    productManager: {
      name: 'Produktmanagement',
      categories: {
        specs: { name: 'Spezifikation & Planung', details: 'PRDs, Roadmaps, Backlog-Pflege, Sprintplanung und Tickets' },
        research: { name: 'Recherche & Analyse', details: 'Nutzerforschung, Kennzahlen prüfen, Wettbewerbsanalyse und A/B-Tests' },
        communication: { name: 'Kommunikation', details: 'Standups, Stakeholder-Abstimmungen, Design-Reviews und Abstimmung mit der Entwicklung' },
      },
    },
    dataScientist: {
      name: 'Data Science',
      categories: {
        analysis: { name: 'Analyse & Modellierung', details: 'Notebooks, statistische Analysen, ML-Training und Datenexploration' },
        dataEngineering: { name: 'Datenaufbereitung', details: 'SQL-Abfragen, Pipelines, Datenbereinigung und ETL-Skripte' },
        research: { name: 'Recherche', details: 'Paper und Doku lesen, neue Methoden und Werkzeuge erkunden' },
        communication: { name: 'Kommunikation', details: 'Ergebnisse präsentieren, Stakeholder-Abstimmungen und Teamdiskussionen' },
      },
    },
    other: {
      name: 'Sonstiges',
      categories: {
        work: { name: 'Arbeit', details: 'Konzentrierte Arbeit und berufliche Aufgaben, die in keine genauere Kategorie passen' },
        communication: { name: 'Kommunikation', details: 'Meetings, Standups, Slack, E-Mail, Videoanrufe, Nachrichten und Abstimmungsrunden' },
      },
    },
  },
  common: {
    distraction: { name: 'Ablenkung', details: 'Unfokussiertes Browsen und passiver Konsum: Social-Media-Feeds, beliebige Videos, zielloses Scrollen, Unterhaltung ohne klare Absicht und Spiele' },
    personal: { name: 'Persönlich', details: 'Bewusste, zweckgebundene Tätigkeit außerhalb der Arbeit: Kontakt zu Freunden und Familie, Finanzen, Reisebuchungen, Erledigungen, Alltagsorganisation und Hobbys' },
  },
}
