export default {
  discardNote: 'As alterações não salvas serão descartadas.',
  apply: 'Aplicar',
  cancel: 'Cancelar',
  applying: 'Aplicando…',
  failed: 'Não foi possível aplicar. Suas categorias não mudaram.',
  palette: {
    title: 'Paletas de cores',
    hint: 'Escolha uma paleta para recolorir todas as categorias em ordem.',
    confirmBody: 'Recolore todas as categorias com “{name}”.',
    names: {
      daygoClassic: 'Daygo clássico',
      forestMoss: 'Musgo da floresta',
      cyberNeon: 'Neon cibernético',
      nordicMist: 'Névoa nórdica',
      sunsetClay: 'Argila do pôr do sol',
    },
  },
  archetype: {
    label: 'Arquétipo',
    hint: 'Define em quais construções e paisagens esta categoria se transforma no mundo 3D.',
    forge: 'Criar',
    sanctum: 'Pensar',
    nexus: 'Conectar',
    camp: 'Descansar',
    order: 'Organizar',
  },
  profession: {
    softwareEngineer: {
      name: 'Engenharia de software',
      categories: {
        coding: { name: 'Programação / depuração', details: 'Escrever, refatorar e corrigir código na IDE ou no terminal' },
        codeReview: { name: 'Revisão de código', details: 'Revisar PRs, ler diffs e deixar comentários' },
        research: { name: 'Pesquisa', details: 'Ler documentação e o Stack Overflow, explorar ferramentas e APIs e escrever documentos de projeto ou especificações técnicas' },
        communication: { name: 'Comunicação', details: 'Reuniões, reuniões diárias, Slack, e-mail, chamadas de vídeo, mensagens e alinhamentos' },
      },
    },
    founderExecutive: {
      name: 'Fundação / liderança',
      categories: {
        building: { name: 'Engenharia / produto', details: 'Programar, desenhar, lançar funcionalidades e construir com as próprias mãos' },
        strategy: { name: 'Pesquisa e estratégia', details: 'Análise da concorrência, posicionamento, reflexão aprofundada e preparação para investidores' },
        insights: { name: 'Dados e insights', details: 'Painéis, dados de retenção, funis e finanças' },
        communication: { name: 'Comunicação', details: 'Alinhamentos do time, conversas com investidores, demonstrações para usuários e contratações' },
      },
    },
    designer: {
      name: 'Design',
      categories: {
        design: { name: 'Design de interface', details: 'Protótipos, componentes de UI, fluxos de usuário, design visual e especificações de entrega' },
        research: { name: 'Pesquisa', details: 'Explorar padrões, auditorias da concorrência, estudos com usuários e análise de métricas' },
        communication: { name: 'Comunicação', details: 'Revisões de design, reuniões diárias, sessões de crítica e apresentação de conceitos' },
      },
    },
    student: {
      name: 'Estudante',
      categories: {
        studying: { name: 'Estudos', details: 'Aulas, leituras, revisão de slides, flashcards e material do curso' },
        assignments: { name: 'Trabalhos', details: 'Artigos, listas de exercícios, projetos de programação e relatórios de laboratório' },
        communication: { name: 'Comunicação', details: 'Grupos de estudo, plantão de dúvidas, grupos de conversa e e-mails para professores' },
      },
    },
    productManager: {
      name: 'Gestão de produto',
      categories: {
        specs: { name: 'Especificações e planejamento', details: 'PRDs, roadmaps, refinamento do backlog, planejamento de sprint e tickets' },
        research: { name: 'Pesquisa e análise', details: 'Pesquisa com usuários, revisão de métricas, análise da concorrência e testes A/B' },
        communication: { name: 'Comunicação', details: 'Reuniões diárias, alinhamentos com stakeholders, revisões de design e conversas com a engenharia' },
      },
    },
    dataScientist: {
      name: 'Ciência de dados',
      categories: {
        analysis: { name: 'Análise e modelagem', details: 'Notebooks, análise estatística, treinamento de modelos e exploração de dados' },
        dataEngineering: { name: 'Engenharia de dados', details: 'Consultas SQL, pipelines, limpeza de dados e scripts de ETL' },
        research: { name: 'Pesquisa', details: 'Ler artigos e documentação, explorar novos métodos e ferramentas' },
        communication: { name: 'Comunicação', details: 'Apresentar resultados, alinhamentos com stakeholders e discussões do time' },
      },
    },
    other: {
      name: 'Outro',
      categories: {
        work: { name: 'Trabalho', details: 'Trabalho focado e responsabilidades profissionais que não se encaixam em uma categoria mais específica' },
        communication: { name: 'Comunicação', details: 'Reuniões, reuniões diárias, Slack, e-mail, chamadas de vídeo, mensagens e alinhamentos' },
      },
    },
  },
  common: {
    distraction: { name: 'Distração', details: 'Navegação sem foco e consumo passivo de conteúdo: redes sociais, vídeos aleatórios, rolagem sem rumo, entretenimento sem intenção clara e jogos' },
    personal: { name: 'Pessoal', details: 'Atividades não profissionais com propósito definido: falar com amigos e família, cuidar das finanças, reservar viagens, resolver pendências, tarefas da vida e hobbies' },
  },
}
