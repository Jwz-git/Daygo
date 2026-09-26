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
