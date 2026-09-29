export default {
  title: 'Semanal',
  developmentFixture: 'Dados de exemplo · só em desenvolvimento',
  navigation: {
    label: 'Navegação por semana',
    current: 'Esta semana',
    backendRequired: 'Trocar de semana está indisponível agora',
  },
  meta: {
    tracked: '{count} min registrados',
  },
  intro: {
    eyebrow: 'Retrospectiva da semana',
    title: 'Para onde a semana foi',
    description: 'O seu tempo registrado, o ritmo de foco, a distribuição por dia e a composição por categoria nesta semana.',
  },
  overview: {
    eyebrow: 'Panorama',
    title: 'Tempo e foco',
    description: 'O tempo de foco exclui distração e inatividade; o tempo registrado exclui os períodos de espaço reservado do sistema.',
    focusAria: 'O foco corresponde a {value} do tempo registrado',
  },
  metric: {
    tracked: 'Tempo registrado',
    focused: 'Tempo de foco',
    other: 'Outro tempo',
    focusRate: 'Fatia de foco',
  },
  categories: {
    eyebrow: 'Composição',
    title: 'Distribuição por categoria',
    count: '{count} categorias',
    distributionAria: 'Fatia do tempo registrado por categoria nesta semana',
    total: 'Total',
  },
  daily: {
    eyebrow: 'Padrão',
    title: 'Linha do tempo diária',
    hint: 'Uma linha por dia; cada bloco é um trecho de categoria',
    idleTag: 'inatividade',
    noActivity: 'Sem atividade',
  },
  rhythm: {
    eyebrow: 'Ritmo',
    title: 'Ritmo de atividade da semana',
    focus: 'Foco',
    idle: 'Inatividade',
    chartAria: 'Minutos de foco e de inatividade por hora do dia',
    empty: 'Ainda não há nada para resumir nesta semana',
  },
  insights: {
    title: 'A semana em resumo',
    activeDays: 'Dias ativos',
    daysCount: '{count} dias',
    ofSeven: 'de 7',
    longestFocus: 'Foco mais longo',
    peakHour: 'Horário de pico',
    avgFocus: 'Foco médio por dia',
    perActiveDay: 'por dia ativo',
    busiestDay: 'Dia mais cheio',
    none: 'Nenhum',
  },
  state: {
    loading: {
      title: 'Carregando esta semana',
      description: 'Só um instante.',
    },
    unavailable: {
      title: 'A visão semanal está indisponível agora',
      description: 'Tente de novo mais tarde ou reinicie o Daygo.',
    },
    failure: {
      title: 'Não foi possível carregar esta semana',
      description: 'Os seus dados não são afetados. Você pode tentar de novo.',
    },
    empty: {
      title: 'Nada registrado nesta semana ainda',
      description: 'A sua retrospectiva semanal aparece aqui quando o Daygo tiver gravado e organizado alguma atividade.',
    },
  },
  scopeNote: 'Com base nos cartões de atividade de cada dia.',
}
