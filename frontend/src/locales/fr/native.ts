export default {
  // Surfaces the native layer renders outside the webview: the frontend pushes
  // this bundle at startup and on every language change (docs/05 §5.5.1).
  applicationPicker: {
    title: 'Choisir une application',
    filterExecutable: 'Applications Windows (*.exe)',
  },
  updater: {
    ownerRequired: 'Installez la mise à jour depuis la fenêtre Daygo en cours d’enregistrement.',
  },
  journalReminder: {
    title: 'C’est l’heure du journal',
    body: 'Prenez quelques minutes pour noter les progrès du jour et le plan de demain.',
  },
  plan: {
    startTitle: "C’est l’heure : {title}",
    startBody: "Prévu de {start} à {end}",
    distractionTitle: "Vous vous dispersez",
    distractionBody: "{minutes} minutes de distraction pendant « {title} ».",
    dayDistractionTitle: "Limite de distraction dépassée aujourd’hui",
    dayDistractionBody: "{minutes} minutes de distraction aujourd’hui ; la limite est de {limit}.",
  },
  applicationMenu: {
    hide: "Masquer Daygo",
    hideOthers: "Masquer les autres",
    showAll: "Tout afficher",
    background: "Continuer à enregistrer en arrière-plan",
    edit: "Édition",
    undo: "Annuler",
    redo: "Rétablir",
    cut: "Couper",
    copy: "Copier",
    paste: "Coller",
    pasteMatch: "Coller et adapter le style",
    delete: "Supprimer",
    selectAll: "Tout sélectionner",
    speech: "Parole",
    startSpeaking: "Commencer la lecture",
    stopSpeaking: "Arrêter la lecture",
    window: "Fenêtre",
    minimize: "Réduire",
    zoom: "Agrandir",
    fullScreen: "Plein écran",
  },
}
