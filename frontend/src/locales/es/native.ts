export default {
  // Surfaces the native layer renders outside the webview: the frontend pushes
  // this bundle at startup and on every language change (docs/05 §5.5.1).
  applicationPicker: {
    title: 'Elige una aplicación',
    filterExecutable: 'Aplicaciones de Windows (*.exe)',
  },
  updater: {
    ownerRequired: 'Instala las actualizaciones desde la ventana de Daygo que está grabando en este momento.',
  },
  journalReminder: {
    title: 'Es hora del diario',
    body: 'Dedica unos minutos a anotar el avance de hoy y el plan para mañana.',
  },
  applicationMenu: {
    hide: "Ocultar Daygo",
    hideOthers: "Ocultar otros",
    showAll: "Mostrar todo",
    background: "Seguir grabando en segundo plano",
    edit: "Edición",
    undo: "Deshacer",
    redo: "Rehacer",
    cut: "Cortar",
    copy: "Copiar",
    paste: "Pegar",
    pasteMatch: "Pegar con el mismo estilo",
    delete: "Eliminar",
    selectAll: "Seleccionar todo",
    speech: "Voz",
    startSpeaking: "Iniciar lectura",
    stopSpeaking: "Detener lectura",
    window: "Ventana",
    minimize: "Minimizar",
    zoom: "Ampliar",
    fullScreen: "Pantalla completa",
  },
}
