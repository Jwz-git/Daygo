export default {
  actionError: 'Algo deu errado. Tente de novo — a sua mensagem continua aqui.',
  retry: 'Tentar de novo',
  working: 'Pensando…',
  tooLong: 'Essa mensagem é longa demais. Encurte e tente de novo.',

  title: 'Conversa',
  newConversation: 'Nova conversa',
  showSidebar: 'Mostrar a barra lateral',
  hideSidebar: 'Ocultar a barra lateral',
  emptyConversations: 'Nenhuma conversa ainda. Envie uma mensagem para começar.',
  deleteConversation: 'Excluir esta conversa',
  renameConversation: 'Renomear esta conversa',
  rename: 'Renomear',
  renameTitlePlaceholder: 'Digite um título para a conversa',
  renameTitleRequired: 'O título é obrigatório',
  removeConfirm: 'Excluir esta conversa?',
  unavailableTitle: 'A conversa está indisponível agora',
  unavailableDescription: 'Tente de novo mais tarde ou reinicie o Daygo.',
  provider: {
    label: 'Serviço de IA',
    placeholder: 'Nenhum serviço de IA selecionado',
  },
  model: {
    label: 'Modelo',
    follow: 'Padrão ({model})',
  },
  composer: {
    placeholder: 'Digite uma mensagem… (Enter para enviar)',
    send: 'Enviar',
    cancel: 'Parar',
  },
  status: {
    failed: 'Falhou',
    canceled: 'Cancelado',
  },
  failure: {
    no_provider: 'Nenhum serviço de IA está configurado. Adicione um nos ajustes.',
    no_provider_selected: 'Esta conversa não tem nenhum serviço de IA selecionado.',
    canceled: 'Cancelado.',
    authentication: 'A chave de API foi recusada. Verifique nos ajustes.',
    rate_limited: 'O serviço está limitando as solicitações. Tente de novo em instantes.',
    timeout: 'O serviço não respondeu a tempo.',
    dns: 'Não foi possível resolver o endereço do serviço. Verifique nos ajustes.',
    connection: 'Não foi possível conectar ao serviço. Confira se ele está ativo e acessível.',
    tls: 'A conexão segura com o serviço falhou. Verifique o endereço e o certificado.',
    network: 'A solicitação não conseguiu chegar ao serviço. Verifique a sua rede.',
    invalid_request: 'O serviço recusou a solicitação. Verifique o endereço e o modelo.',
    unsupported_feature: 'Este modelo não oferece a saída estruturada que o chat exige.',
    invalid_output: 'Não foi possível ler a resposta do serviço. Confira se o modelo combina com esta API.',
    tool_budget: 'Este turno esgotou as chamadas de ferramenta e foi encerrado.',
    internal: 'Ocorreu um erro interno no Daygo. Tente de novo.',
  },
  memory: {
    title: 'Instruções globais',
    hint: 'O que você quer que o Daygo lembre em todas as conversas.',
    placeholder: 'Por exemplo: respostas curtas; me chame de Ana…',
    save: 'Salvar',
    saved: 'Salvo',
  },
  loadError: 'Não foi possível carregar as conversas',

  drawer: {
    conversations: 'Conversas',
    memory: 'Instruções globais',
    newChat: 'Nova conversa',
    today: 'Hoje',
    yesterday: 'Ontem',
    older: 'Mais antigas',
    untitled: 'Conversa sem título',
  },

  bubble: {
    copy: 'Copiar',
    copied: 'Copiado',
    roleAssistant: 'Daygo',
  },

  welcome: {
    title: 'Oi, como posso ajudar?',
    subtitle: 'Pergunte o que quiser sobre a sua linha do tempo, o diário ou a retrospectiva semanal',
    hints: [
      'O que eu fiz hoje?',
      'Mostre a divisão por categoria desta semana.',
      'Adicione uma categoria: Estudo',
      'Veja o diário de ontem.',
    ],
  },

  dateDivider: {
    today: 'Hoje',
    yesterday: 'Ontem',
  },
}
