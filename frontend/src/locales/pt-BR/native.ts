export default {
  // Surfaces the native layer renders outside the webview: the frontend pushes
  // this bundle at startup and on every language change (docs/05 §5.5.1).
  applicationPicker: {
    title: 'Escolher um aplicativo',
    filterExecutable: 'Aplicativos do Windows (*.exe)',
  },
  updater: {
    ownerRequired: 'Instale as atualizações na janela do Daygo que está gravando agora.',
  },
  journalReminder: {
    title: 'Hora do diário',
    body: 'Reserve alguns minutos para registrar o progresso de hoje e o plano para amanhã.',
  },
  plan: {
    startTitle: "Começando: {title}",
    startBody: "Previsto para {start}–{end}",
    distractionTitle: "Você está se distraindo",
    distractionBody: "{minutes} minutos de distração durante \"{title}\".",
    dayDistractionTitle: "A distração de hoje passou do limite",
    dayDistractionBody: "Hoje foram {minutes} minutos de distração; o limite é {limit}.",
  },
  applicationMenu: {
    hide: "Ocultar Daygo",
    hideOthers: "Ocultar outros",
    showAll: "Mostrar tudo",
    background: "Continuar gravando em segundo plano",
    edit: "Editar",
    undo: "Desfazer",
    redo: "Refazer",
    cut: "Recortar",
    copy: "Copiar",
    paste: "Colar",
    pasteMatch: "Colar com o mesmo estilo",
    delete: "Apagar",
    selectAll: "Selecionar tudo",
    speech: "Fala",
    startSpeaking: "Iniciar leitura",
    stopSpeaking: "Parar leitura",
    window: "Janela",
    minimize: "Minimizar",
    zoom: "Ampliar",
    fullScreen: "Tela cheia",
  },
}
