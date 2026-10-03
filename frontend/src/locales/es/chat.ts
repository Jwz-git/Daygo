export default {
  actionError: 'Algo salió mal. Inténtalo de nuevo: tu mensaje sigue aquí.',
  retry: 'Reintentar',
  working: 'Pensando…',
  tooLong: 'Ese mensaje es demasiado largo. Acórtalo e inténtalo de nuevo.',

  title: 'Chat',
  newConversation: 'Chat nuevo',
  showSidebar: 'Mostrar la barra lateral',
  hideSidebar: 'Ocultar la barra lateral',
  emptyConversations: 'Todavía no hay conversaciones. Envía un mensaje para empezar una.',
  deleteConversation: 'Eliminar esta conversación',
  renameConversation: 'Cambiar el nombre de esta conversación',
  rename: 'Cambiar el nombre',
  renameTitlePlaceholder: 'Escribe un título para la conversación',
  renameTitleRequired: 'El título es obligatorio',
  removeConfirm: '¿Eliminar esta conversación?',
  unavailableTitle: 'El chat no está disponible ahora mismo',
  unavailableDescription: 'Inténtalo más tarde o reinicia Daygo.',
  provider: {
    label: 'Servicio de IA',
    placeholder: 'Ningún servicio de IA seleccionado',
  },
  model: {
    label: 'Modelo',
    follow: 'Predeterminado ({model})',
  },
  composer: {
    placeholder: 'Escribe un mensaje… (Enter para enviar)',
    send: 'Enviar',
    cancel: 'Detener',
  },
  status: {
    failed: 'Falló',
    canceled: 'Cancelado',
  },
  failure: {
    no_provider: 'No hay ningún servicio de IA configurado. Añade uno en los ajustes.',
    no_provider_selected: 'Esta conversación no tiene ningún servicio de IA seleccionado.',
    canceled: 'Cancelado.',
    authentication: 'La clave de API fue rechazada. Revísala en los ajustes.',
    rate_limited: 'El servicio está limitando las solicitudes. Inténtalo de nuevo en un momento.',
    timeout: 'El servicio no respondió a tiempo.',
    dns: 'No se pudo resolver la dirección del servicio. Revísala en los ajustes.',
    connection: 'No se pudo conectar con el servicio. Comprueba que esté en marcha y sea accesible.',
    tls: 'Falló la conexión segura con el servicio. Revisa la dirección y su certificado.',
    network: 'La solicitud no pudo llegar al servicio. Revisa tu red.',
    invalid_request: 'El servicio rechazó la solicitud. Revisa la dirección y el modelo.',
    unsupported_feature: 'Este modelo no admite la salida estructurada que necesita el chat.',
    invalid_output: 'No se pudo leer la respuesta del servicio. Comprueba que el modelo encaje con esta API.',
    tool_budget: 'Este turno agotó sus llamadas a herramientas y se detuvo.',
    internal: 'Se produjo un error interno en Daygo. Inténtalo de nuevo.',
  },
  memory: {
    title: 'Instrucciones globales',
    hint: 'Cosas que quieres que Daygo recuerde en todas las conversaciones.',
    placeholder: 'Por ejemplo: responde de forma breve; llámame Ana…',
    save: 'Guardar',
    saved: 'Guardado',
  },
  loadError: 'No se pudieron cargar las conversaciones',

  drawer: {
    conversations: 'Conversaciones',
    memory: 'Instrucciones globales',
    newChat: 'Chat nuevo',
    today: 'Hoy',
    yesterday: 'Ayer',
    older: 'Anteriores',
    untitled: 'Chat sin título',
  },

  bubble: {
    copy: 'Copiar',
    copied: 'Copiado',
    roleAssistant: 'Daygo',
  },

  welcome: {
    title: 'Hola, ¿en qué puedo ayudarte?',
    subtitle: 'Pregúntame lo que quieras sobre tu cronología, tus reuniones diarias o tu repaso semanal',
    hints: [
      '¿Qué hice hoy?',
      'Muéstrame el desglose por categorías de esta semana.',
      'Añade una categoría nueva: Aprendizaje',
      'Revisa el diario de ayer.',
    ],
  },

  dateDivider: {
    today: 'Hoy',
    yesterday: 'Ayer',
  },
}
