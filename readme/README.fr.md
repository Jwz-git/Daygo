<p align="center">
  <img src="../docs/assets/readme/banner.en.webp" width="100%" alt="Daygo — À la fin de la journée, vous souvenez-vous de votre travail ?" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><img src="https://img.shields.io/github/v/release/Jwz-git/Daygo?style=flat-square&color=F3854B&label=version" alt="Dernière version stable" /></a>
  <img src="https://img.shields.io/badge/macOS-14%2B-333333?style=flat-square&logo=apple&logoColor=white" alt="macOS 14+" />
  <img src="https://img.shields.io/badge/Windows-11%2024H2%2B-0078D4?style=flat-square" alt="Windows 11 24H2+" />
  <a href="../LICENSE"><img src="https://img.shields.io/badge/license-MIT-6E7DF7?style=flat-square" alt="MIT License" /></a>
  <img src="https://img.shields.io/badge/Go%20%C2%B7%20Wails%20%C2%B7%20Vue%203-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go · Wails · Vue 3" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><b>Télécharger</b></a> ·
  <a href="https://jwz-git.github.io/Daygo/"><b>Site officiel</b></a> ·
  <a href="../docs/README.md">Documentation de conception</a>
</p>

<p align="center">
  <a href="../README.md">简体中文</a> · <a href="README.zh-Hant.md">繁體中文</a> · <a href="README.en.md">English</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.de.md">Deutsch</a> · <strong>Français</strong> · <a href="README.es.md">Español</a> · <a href="README.pt-BR.md">Português (Brasil)</a>
</p>

<p align="center"><strong>Gardez une trace du travail à l’écran. Revenez sur votre journée.</strong></p>

Daygo est un outil de suivi du travail et de bilan pour macOS et Windows. Il prend des captures d’écran à intervalles réguliers en arrière-plan. Le service d’IA que vous choisissez organise l’activité à l’écran en **chronologie, bilan quotidien et résumé hebdomadaire**, pour les réunions d’équipe, les rétrospectives et le rappel des détails de votre travail.

**Captures ponctuelles · Stockage local · IA au choix · Interface en neuf langues**

[Téléchargement et installation](#téléchargement-et-installation) · [Premiers pas](#premiers-pas) · [Aperçu des fonctionnalités](#aperçu-des-fonctionnalités) · [Confidentialité et données](#confidentialité-et-données) · [Contribuer au développement](#contribuer-au-développement)

## Téléchargement et installation

Choisissez l’installeur adapté à votre plateforme dans les [Assets de la dernière version stable](https://github.com/Jwz-git/Daygo/releases/latest) :

| Plateforme | Système et architecture | Installeur |
|---|---|---|
| macOS | macOS 14+, Apple Silicon (arm64) | `Daygo-<version>-arm64.dmg` |
| Windows | Windows 11 24H2+ (build 26100+), x64 (amd64) | `Daygo-<version>-amd64-installer.exe` |

macOS est la plateforme de développement principale ; Windows dispose d’un installeur x64. Aucun installeur n’est encore publié pour les autres architectures ou Linux. Le code source peut être en avance sur la version publiée. Consultez l’[état des modules](../docs/09-roadmap.md#91-模块总表) pour les fonctionnalités et la [distribution et les mises à jour](../docs/modules/delivery.md) pour les vérifications de signature, notarisation et installation.

## Premiers pas

1. **Installer et autoriser** : lancez Daygo. Sous macOS, accordez l’autorisation de capture d’écran et redémarrez lorsque l’application le demande.
2. **Configurer l’IA** : ajoutez l’URL du service, la clé API et le modèle dans les paramètres. Enregistrez, puis ouvrez « Test et essai du modèle » pour vérifier une réponse avec du texte ou une image.
3. **Choisir le suivi** : définissez l’intervalle des captures, les applications bloquées et la limite d’espace disque, puis activez l’enregistrement.
4. **Consulter les résultats** : après le premier lot d’analyse, examinez les cartes et les images d’origine dans la chronologie, puis modifiez-les au besoin. Les pages quotidienne et hebdomadaire permettent de revenir sur votre activité.

Daygo n’inclut ni service d’IA ni crédits. L’analyse automatique nécessite la reconnaissance d’images et les sorties structurées prévues par les protocoles configurés. Recevoir une réponse dans la page d’essai ne valide pas l’ensemble de l’analyse. Les modèles locaux doivent aussi disposer de ces capacités.

## Aperçu des fonctionnalités

<sub>Les captures ci-dessous utilisent des données d’exemple anonymes.</sub>

### Chronologie automatique

Daygo capture l’écran principal à intervalles réguliers. L’IA crée des cartes d’activité avec horaires, titres, résumés et catégories. Ouvrez une carte pour examiner les images d’origine, ou modifiez, supprimez et retraitez les résultats.

<img src="../docs/assets/readme/timeline.en.webp" width="100%" alt="Chronologie : cartes d’activité par horaire et panneau de détail" />

### Bilan quotidien

Consultez le déroulement d’une journée et générez un résumé pour votre réunion d’équipe avec les faits marquants, les tâches terminées et les obstacles. Ajoutez votre point de vue avec un journal et des objectifs quotidiens.

<img src="../docs/assets/readme/daily.en.webp" width="100%" alt="Bilan quotidien : déroulement du travail et résumé pour la réunion d’équipe" />

### Bilan hebdomadaire

Revenez sur la semaine avec le déroulement du travail, la carte de concentration et de distraction, la répartition par catégorie, les applications les plus utilisées et les flux de temps. Le temps total suivi exclut la catégorie System.

<img src="../docs/assets/readme/weekly.en.webp" width="100%" alt="Bilan hebdomadaire : applications par catégorie et diagramme des flux de temps" />

### Suivi en arrière-plan et personnalisation

| Fonctionnalité | Description |
|---|---|
| Suivi en arrière-plan | L’enregistrement continue après la fermeture de la fenêtre ; rouvrez-la depuis la barre des menus macOS ou la zone de notification Windows |
| Pause et reprise | Pause de 15 / 30 / 60 minutes ou sans limite ; les pauses temporisées se terminent automatiquement |
| Événements système | La capture s’arrête pendant la veille, le verrouillage et l’économiseur d’écran ; les événements système ne réactivent pas un enregistrement désactivé manuellement |
| Réglages des captures | Intervalles de 1 / 5 / 10 / 20 / 30 / 60 secondes, 10 par défaut ; hauteur de 720 / 1080 pixels, 1080 par défaut |
| Services d’IA | OpenAI Chat Completions, OpenAI Responses et Anthropic Messages ; plusieurs modèles par fournisseur et une chaîne de repli ordonnée |
| Apparence et catégories | Thème clair / sombre / système ; noms, ordre et couleurs des catégories modifiables ; lancement à la connexion et icône du Dock macOS |

L’interface prend en charge le chinois simplifié, le chinois traditionnel, l’anglais, le japonais, le coréen, l’allemand, le français, l’espagnol et le portugais brésilien.

<details>
<summary>Voir le mode sombre</summary>

<p><img src="../docs/assets/readme/dark.en.webp" width="100%" alt="Daygo en mode sombre" /></p>

</details>

Les journées de la chronologie, du journal et des objectifs commencent à **4 h, heure locale**. Les résumés de réunion utilisent les jours calendaires. Une activité nocturne peut donc relever de dates différentes selon la vue.

## Confidentialité et données

- **Stockage local** : captures, chronologies, journal, paramètres et base de données restent sur votre appareil. Daygo n’a ni serveur propre, ni compte, ni service de synchronisation.
- **Vous choisissez la destination** : les données d’écran quittent l’appareil uniquement pour un service d’IA explicitement configuré. Un modèle local compatible peut effectuer l’analyse sur votre appareil. Les services tiers traitent les données selon leurs propres politiques de confidentialité.
- **Blocage et protection de l’application au premier plan** : les applications bloquées sont exclues des captures. Lorsqu’une application bloquée est au premier plan, Daygo conserve une image de remplacement expurgée. Les deux protections sont maintenues.
- **Stockage système des identifiants** : les clés API restent uniquement dans le Trousseau macOS ou Windows Credential Manager. L’interface peut les écrire mais pas les lire. Elles ne sont pas stockées dans la base de l’application, localStorage ou les messages d’erreur.

Le suivi utilise des captures ponctuelles, sans flux continu d’enregistrement d’écran. L’analyse d’utilisation et les rapports de plantage sont désactivés par défaut ; leur transmission n’est pas encore implémentée.

La désinstallation conserve les données utilisateur et les identifiants. Les limites complètes figurent dans [Confidentialité et sécurité](../docs/07-privacy-security.md).

## Contribuer au développement

Go gère la logique métier et les écritures en base. Les capacités propres aux plateformes sont isolées par des interfaces ; Vue accède à Go par les liaisons Wails générées. `test` est la branche de développement, `main` la branche stable.

Le développement sous macOS nécessite Go, Node.js/npm et Xcode Command Line Tools. Voir [go.mod](../go.mod) pour la version de Go et le [script de développement](../scripts/dev.sh) pour les exigences Node.js.

```bash
git clone --branch test https://github.com/Jwz-git/Daygo.git
cd Daygo
./scripts/gate.sh   # Préparation, compilation, tests Go / frontend et vérification des documents
```

Le script de vérification prépare les artefacts frontend et les liaisons Wails dans l’ordre requis. Les commandes par plateforme se trouvent dans les [points d’entrée des scripts](../scripts/README.md), les règles de conception et de contribution dans la [documentation de conception](../docs/README.md) et [AGENTS.md](../AGENTS.md).

Signalez les problèmes dans une [Issue](https://github.com/Jwz-git/Daygo/issues) avec le système, la version de l’application, les étapes de reproduction et des erreurs expurgées. Ne publiez pas de captures réelles, de bases de données ou de clés API.

## Licence

Publié sous [MIT License](../LICENSE).

<sub>Inspiré de <a href="https://github.com/JerryZLiu/Dayflow">Dayflow</a> (MIT, © 2025 Jerry Liu).</sub>
