<p align="center">
  <img src="../docs/assets/readme/banner.en.webp" width="100%" alt="Daygo — No fim do dia, você lembra no que trabalhou?" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><img src="https://img.shields.io/github/v/release/Jwz-git/Daygo?style=flat-square&color=F3854B&label=version" alt="Versão estável mais recente" /></a>
  <img src="https://img.shields.io/badge/macOS-14%2B-333333?style=flat-square&logo=apple&logoColor=white" alt="macOS 14+" />
  <img src="https://img.shields.io/badge/Windows-11%2024H2%2B-0078D4?style=flat-square" alt="Windows 11 24H2+" />
  <a href="../LICENSE"><img src="https://img.shields.io/badge/license-MIT-6E7DF7?style=flat-square" alt="MIT License" /></a>
  <img src="https://img.shields.io/badge/Go%20%C2%B7%20Wails%20%C2%B7%20Vue%203-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go · Wails · Vue 3" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><b>Baixar</b></a> ·
  <a href="https://jwz-git.github.io/Daygo/"><b>Site oficial</b></a> ·
  <a href="../docs/README.md">Documentação de design</a>
</p>

<p align="center">
  <a href="../README.md">简体中文</a> · <a href="README.zh-Hant.md">繁體中文</a> · <a href="README.en.md">English</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.de.md">Deutsch</a> · <a href="README.fr.md">Français</a> · <a href="README.es.md">Español</a> · <strong>Português (Brasil)</strong>
</p>

<p align="center"><strong>Registre seu trabalho na tela. Reveja seu dia.</strong></p>

Daygo é uma ferramenta de registro e revisão do trabalho para macOS e Windows. Ela captura a tela em intervalos, em segundo plano, e usa o provedor de IA que você escolher para organizar as atividades em uma **linha do tempo, revisão diária e resumo semanal**, para reuniões de acompanhamento, retrospectivas e para lembrar detalhes do trabalho.

**Capturas pontuais · Armazenamento local · IA à sua escolha · Interface em nove idiomas**

[Download e instalação](#download-e-instalação) · [Primeiros passos](#primeiros-passos) · [Visão das funcionalidades](#visão-das-funcionalidades) · [Privacidade e dados](#privacidade-e-dados) · [Contribuir com o desenvolvimento](#contribuir-com-o-desenvolvimento)

## Download e instalação

Escolha o instalador da sua plataforma nos [Assets da versão estável mais recente](https://github.com/Jwz-git/Daygo/releases/latest):

| Plataforma | Sistema e arquitetura | Instalador |
|---|---|---|
| macOS | macOS 14+, Apple Silicon (arm64) | `Daygo-<versão>-arm64.dmg` |
| Windows | Windows 11 24H2+ (build 26100+), x64 (amd64) | `Daygo-<versão>-amd64-installer.exe` |

macOS é a plataforma principal de desenvolvimento; o Windows também tem um instalador x64. Ainda não há instaladores publicados para outras arquiteturas ou Linux. O código-fonte pode estar à frente da versão publicada. Confira o [estado dos módulos](../docs/09-roadmap.md#91-模块总表) e os registros de assinatura, notarização e instalação em [Distribuição e atualizações](../docs/modules/delivery.md).

## Primeiros passos

1. **Instale e conceda permissão**: inicie o Daygo. No macOS, permita a captura de tela e reinicie quando o aplicativo solicitar.
2. **Configure a IA**: adicione a URL do serviço, a chave de API e o modelo nas configurações. Salve e abra “Teste e experimentação do modelo” para conferir uma resposta com texto ou uma imagem.
3. **Escolha como registrar**: defina o intervalo de captura, os aplicativos bloqueados e o limite de disco. Depois, ative o registro.
4. **Confira os resultados**: após o primeiro lote de análise, veja os cartões e as imagens originais na linha do tempo e edite o que precisar. As páginas diária e semanal permitem rever suas atividades.

O Daygo não inclui serviço de IA nem créditos. A análise automática exige reconhecimento de imagens e saídas estruturadas compatíveis com os protocolos configurados. Receber uma resposta na página de teste não valida todo o processo de análise. Modelos locais também precisam dessas capacidades.

## Visão das funcionalidades

<sub>As capturas abaixo usam dados de exemplo anônimos.</sub>

### Linha do tempo automática

O Daygo captura a tela principal em intervalos, e a IA cria cartões com horários, títulos, resumos e categorias. Abra um cartão para conferir as imagens originais ou edite, exclua e reprocesse os resultados.

<img src="../docs/assets/readme/timeline.en.webp" width="100%" alt="Linha do tempo: cartões de atividades por horário e painel de detalhes" />

### Revisão diária

Veja o fluxo de trabalho do dia e gere um resumo para a reunião de acompanhamento com destaques, tarefas concluídas e impedimentos. Acrescente sua perspectiva com um diário e metas diárias.

<img src="../docs/assets/readme/daily.en.webp" width="100%" alt="Revisão diária: fluxo de trabalho e resumo para a reunião" />

### Revisão semanal

Reveja a semana com o fluxo de trabalho, o mapa de foco e distração, as proporções por categoria, os aplicativos mais usados e os fluxos de tempo. O tempo total registrado exclui a categoria System.

<img src="../docs/assets/readme/weekly.en.webp" width="100%" alt="Revisão semanal: aplicativos por categoria e diagrama de fluxos de tempo" />

### Registro em segundo plano e personalização

| Funcionalidade | Descrição |
|---|---|
| Registro em segundo plano | O registro continua após fechar a janela; reabra pela barra de menus do macOS ou pela área de notificação do Windows |
| Pausa e retomada | Pause por 15 / 30 / 60 minutos ou por tempo indeterminado; pausas com duração definida terminam automaticamente |
| Eventos do sistema | A captura pausa durante o repouso, bloqueio e protetor de tela; eventos do sistema não reativam um registro que você desativou manualmente |
| Configurações de captura | Intervalos de 1 / 5 / 10 / 20 / 30 / 60 segundos, padrão de 10; altura de 720 / 1080 pixels, padrão de 1080 |
| Serviços de IA | OpenAI Chat Completions, OpenAI Responses e Anthropic Messages; vários modelos por provedor e uma cadeia de alternativas ordenada |
| Aparência e categorias | Tema claro / escuro / do sistema; nomes, ordem e cores das categorias editáveis; início ao entrar e ícone do Dock no macOS |

A interface oferece chinês simplificado, chinês tradicional, inglês, japonês, coreano, alemão, francês, espanhol e português do Brasil.

<details>
<summary>Ver o modo escuro</summary>

<p><img src="../docs/assets/readme/dark.en.webp" width="100%" alt="Daygo no modo escuro" /></p>

</details>

Os dias da linha do tempo, do diário e das metas começam às **4h no horário local**. Os resumos de reunião usam dias do calendário. Assim, uma atividade de madrugada pode pertencer a datas diferentes conforme a visualização.

## Privacidade e dados

- **Armazenamento local**: capturas, linhas do tempo, diário, configurações e banco de dados ficam no seu dispositivo. O Daygo não tem servidor próprio, contas ou serviço de sincronização.
- **Você escolhe o destino**: os dados de tela só saem do dispositivo para um serviço de IA que você configure explicitamente. Um modelo local compatível pode analisar a tela no próprio dispositivo. Serviços de terceiros tratam os dados conforme suas políticas de privacidade.
- **Bloqueio de aplicativos e proteção do primeiro plano**: aplicativos bloqueados são excluídos das capturas. Quando um aplicativo bloqueado está em primeiro plano, o Daygo salva uma imagem substituta sem seu conteúdo. As duas proteções são mantidas.
- **Armazenamento de credenciais do sistema**: as chaves de API ficam apenas no Acesso às Chaves do macOS ou no Windows Credential Manager. A interface pode gravá-las, mas não lê-las. Elas não entram no banco de dados do aplicativo, no localStorage ou nas mensagens de erro.

O registro usa capturas pontuais e evita um fluxo contínuo de gravação da tela. A análise de uso e os relatórios de falhas ficam desativados por padrão; o envio ainda não foi implementado.

A desinstalação preserva os dados e as credenciais do usuário. Veja os limites completos em [Privacidade e segurança](../docs/07-privacy-security.md).

## Contribuir com o desenvolvimento

Go cuida da lógica de produto e das gravações no banco de dados. Os recursos de cada plataforma ficam isolados por interfaces; Vue acessa Go pelos bindings Wails gerados. `test` é a branch de desenvolvimento e `main` é a branch estável.

O desenvolvimento no macOS exige Go, Node.js/npm e Xcode Command Line Tools. Veja a versão de Go em [go.mod](../go.mod) e os requisitos de Node.js no [script de desenvolvimento](../scripts/dev.sh).

```bash
git clone --branch test https://github.com/Jwz-git/Daygo.git
cd Daygo
./scripts/gate.sh   # Preparação, compilação, testes Go / frontend e verificação da documentação
```

O script de verificação prepara os artefatos frontend e os bindings Wails na ordem necessária. Os comandos por plataforma estão em [Entradas dos scripts](../scripts/README.md); as regras de design e contribuição estão na [documentação de design](../docs/README.md) e em [AGENTS.md](../AGENTS.md).

Relate problemas em uma [Issue](https://github.com/Jwz-git/Daygo/issues) com o sistema, a versão do aplicativo, os passos de reprodução e erros sem informações sensíveis. Não envie capturas reais, bancos de dados ou chaves de API.

## Licença

Publicado sob a [MIT License](../LICENSE).

<sub>Inspirado por <a href="https://github.com/JerryZLiu/Dayflow">Dayflow</a> (MIT, © 2025 Jerry Liu).</sub>
