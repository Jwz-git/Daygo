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
  charts: {
    distribution: {
      title: 'Répartition de la semaine',
      total: 'Total',
      aria: 'Part du temps par catégorie cette semaine',
    },
    context: {
      title: 'Changements de contexte et distractions',
      shifts: 'Changements de contexte',
      distractions: 'Distractions',
      distribution: 'Au fil de la journée (10:00–18:00)',
      comparison: 'Par jour',
      insight: '{day} a connu le plus d’interruptions : {shifts} changements de contexte et {distracted} distractions.',
      insightNone: 'Aucun schéma notable de changements ou de distractions cette semaine.',
      aria: 'Changements de contexte et distractions par jour',
    },
    workflow: {
      title: 'Votre flux de travail cette semaine',
      total: 'Total de la semaine',
      aria: 'Catégorie principale pour chaque moment de chaque jour',
    },
    heatmap: {
      title: 'Carte de chaleur concentration / distraction',
      focused: 'Travail concentré',
      distracted: 'Distrait',
      aria: 'Degré de concentration ou de distraction à chaque moment de chaque jour',
    },
    treemap: {
      title: 'Les plus utilisées par catégorie',
      empty: 'Pas encore de données d’applications cette semaine',
      aria: 'Applications avec le plus de temps dans chaque catégorie',
      // Dayflow's compact units ("6hr 4m", "+ 152m"), kept in English on purpose.
      hoursMinutes: '{hours}hr {minutes}m',
      hours: '{hours}hr',
      minutes: '{minutes}m',
    },
    sankey: {
      title: 'Temps entre catégories et applications',
      aria: 'Comment le temps de la semaine a circulé des catégories vers les applications',
    },
    tooltip: {
      shifts: '{count} changements de contexte',
      distractions: 'Distractions : {count}',
      noRecord: 'Rien d’enregistré',
      share: '{value} de {name}',
      change: '{value} par rapport à la semaine dernière',
      newThisWeek: 'Non utilisée la semaine dernière',
      pinHint: 'Cliquez pour figer la mise en évidence, cliquez à nouveau pour la libérer',
    },
    otherApp: 'Autres',
    otherCategory: 'Autres',
  },
}
