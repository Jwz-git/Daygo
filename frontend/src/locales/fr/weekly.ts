export default {
  title: 'Hebdomadaire',
  developmentFixture: 'Données d’exemple · développement uniquement',
  navigation: {
    label: 'Navigation par semaine',
    current: 'Cette semaine',
    backendRequired: 'Le changement de semaine est indisponible pour le moment',
  },
  meta: {
    tracked: '{count} min enregistrées',
  },
  intro: {
    eyebrow: 'Rétrospective de la semaine',
    title: 'Où est passée la semaine',
    description: 'Votre temps enregistré, votre rythme de concentration, la répartition quotidienne et la composition par catégorie.',
  },
  overview: {
    eyebrow: 'Vue d’ensemble',
    title: 'Temps et concentration',
    description: 'Le temps de concentration exclut les distractions et l’inactivité ; le temps enregistré exclut les périodes système.',
    focusAria: 'La concentration représente {value} du temps enregistré',
  },
  metric: {
    tracked: 'Temps enregistré',
    focused: 'Temps de concentration',
    other: 'Autre temps',
    focusRate: 'Part de concentration',
  },
  categories: {
    eyebrow: 'Composition',
    title: 'Répartition par catégorie',
    count: '{count} catégories',
    distributionAria: 'Part du temps enregistré par catégorie cette semaine',
    total: 'Total',
  },
  daily: {
    eyebrow: 'Répartition',
    title: 'Chronologie quotidienne',
    hint: 'Une ligne par jour ; chaque bloc correspond à un segment de catégorie',
    idleTag: 'inactif',
    noActivity: 'Aucune activité',
  },
  rhythm: {
    eyebrow: 'Rythme',
    title: 'Rythme d’activité hebdomadaire',
    focus: 'Concentration',
    idle: 'Inactivité',
    chartAria: 'Minutes de concentration et d’inactivité par heure de la journée',
    empty: 'Rien à résumer cette semaine pour le moment',
  },
  insights: {
    title: 'La semaine en un coup d’œil',
    activeDays: 'Jours actifs',
    daysCount: '{count} jours',
    ofSeven: 'sur 7 jours',
    longestFocus: 'Plus longue concentration',
    peakHour: 'Heure de pointe',
    avgFocus: 'Concentration quotidienne moyenne',
    perActiveDay: 'par jour actif',
    busiestDay: 'Jour le plus chargé',
    none: 'Aucun',
  },
  state: {
    loading: {
      title: 'Chargement de la semaine',
      description: 'Un instant.',
    },
    unavailable: {
      title: 'La vue hebdomadaire est indisponible pour le moment',
      description: 'Réessayez plus tard ou redémarrez Daygo.',
    },
    failure: {
      title: 'Impossible de charger cette semaine',
      description: 'Vos données ne sont pas affectées. Vous pouvez réessayer.',
    },
    empty: {
      title: 'Rien d’enregistré cette semaine',
      description: 'Votre rétrospective hebdomadaire apparaîtra ici dès que Daygo aura enregistré et organisé de l’activité.',
    },
  },
  scopeNote: 'Calculé à partir de vos cartes d’activité quotidiennes.',
}
