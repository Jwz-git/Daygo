export default {
  actionError: 'Un problème est survenu. Réessayez : votre message est toujours là.',
  retry: 'Réessayer',
  working: 'Réflexion…',
  tooLong: 'Ce message est trop long. Raccourcissez-le et réessayez.',

  title: 'Discussion',
  newConversation: 'Nouvelle discussion',
  showSidebar: 'Afficher le panneau latéral',
  hideSidebar: 'Masquer le panneau latéral',
  emptyConversations: 'Aucune discussion pour le moment. Envoyez un message pour en démarrer une.',
  deleteConversation: 'Supprimer cette discussion',
  renameConversation: 'Renommer cette discussion',
  rename: 'Renommer',
  renameTitlePlaceholder: 'Saisissez un titre pour la discussion',
  renameTitleRequired: 'Un titre est requis',
  removeConfirm: 'Supprimer cette discussion ?',
  unavailableTitle: 'La discussion est indisponible pour le moment',
  unavailableDescription: 'Réessayez plus tard ou redémarrez Daygo.',
  provider: {
    label: 'Service d’IA',
    placeholder: 'Aucun service d’IA sélectionné',
  },
  model: {
    label: 'Modèle',
    follow: 'Par défaut ({model})',
  },
  composer: {
    placeholder: 'Saisissez un message… (Entrée pour envoyer)',
    send: 'Envoyer',
    cancel: 'Arrêter',
  },
  status: {
    failed: 'Échec',
    canceled: 'Annulé',
  },
  failure: {
    no_provider: "Aucun service d'IA n'est configuré. Ajoutez-en un dans les réglages.",
    no_provider_selected: "Aucun service d'IA n'est sélectionné pour cette conversation.",
    canceled: 'Annulé.',
    authentication: "La clé d'API a été refusée. Vérifiez-la dans les réglages.",
    rate_limited: 'Le service limite les requêtes. Réessayez dans un instant.',
    timeout: "Le service n'a pas répondu à temps.",
    dns: "L'adresse du service n'a pas pu être résolue. Vérifiez-la dans les réglages.",
    connection: "Impossible de se connecter au service. Vérifiez qu'il tourne et qu'il est joignable.",
    tls: "La connexion sécurisée au service a échoué. Vérifiez l'adresse et son certificat.",
    network: "La requête n'a pas pu atteindre le service. Vérifiez votre réseau.",
    invalid_request: "Le service a refusé la requête. Vérifiez l'adresse et le modèle.",
    unsupported_feature: "Ce modèle ne prend pas en charge la sortie structurée nécessaire au chat.",
    invalid_output: "La réponse du service est illisible. Vérifiez que le modèle correspond à cette API.",
    tool_budget: "Ce tour a épuisé ses appels d'outils et a été interrompu.",
    internal: "Une erreur interne à Daygo s'est produite. Réessayez.",
  },
  memory: {
    title: 'Instructions globales',
    hint: 'Ce que vous voulez que Daygo garde en mémoire dans chaque discussion.',
    placeholder: 'ex. : réponses courtes ; appelez-moi Camille…',
    save: 'Enregistrer',
    saved: 'Enregistré',
  },
  loadError: 'Impossible de charger les discussions',

  drawer: {
    conversations: 'Discussions',
    memory: 'Instructions globales',
    newChat: 'Nouvelle discussion',
    today: 'Aujourd’hui',
    yesterday: 'Hier',
    older: 'Plus ancien',
    untitled: 'Discussion sans titre',
  },

  bubble: {
    copy: 'Copier',
    copied: 'Copié',
    roleAssistant: 'Daygo',
  },

  welcome: {
    title: 'Bonjour, comment puis-je vous aider ?',
    subtitle: 'Posez-moi vos questions sur votre chronologie, vos mêlées ou vos rétrospectives hebdomadaires',
    hints: [
      'Qu’ai-je fait aujourd’hui ?',
      'Montre-moi la répartition par catégorie de cette semaine.',
      'Ajoute une catégorie : Apprentissage',
      'Regarde le journal d’hier.',
    ],
  },

  dateDivider: {
    today: 'Aujourd’hui',
    yesterday: 'Hier',
  },
}
