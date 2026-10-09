<p align="center">
  <img src="../docs/assets/readme/banner.en.webp" width="100%" alt="Daygo — 一日の終わりに、何をしていたか思い出せますか？" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><img src="https://img.shields.io/github/v/release/Jwz-git/Daygo?style=flat-square&color=F3854B&label=version" alt="最新の正式リリース" /></a>
  <img src="https://img.shields.io/badge/macOS-14%2B-333333?style=flat-square&logo=apple&logoColor=white" alt="macOS 14+" />
  <img src="https://img.shields.io/badge/Windows-11%2024H2%2B-0078D4?style=flat-square" alt="Windows 11 24H2+" />
  <a href="../LICENSE"><img src="https://img.shields.io/badge/license-MIT-6E7DF7?style=flat-square&color=6E7DF7" alt="MIT License" /></a>
  <img src="https://img.shields.io/badge/Go%20%C2%B7%20Wails%20%C2%B7%20Vue%203-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go · Wails · Vue 3" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><b>ダウンロード</b></a> ·
  <a href="https://jwz-git.github.io/Daygo/"><b>公式サイト</b></a> ·
  <a href="../docs/README.md">設計ドキュメント</a>
</p>

<p align="center">
  <a href="../README.md">简体中文</a> · <a href="README.zh-Hant.md">繁體中文</a> · <a href="README.en.md">English</a> · <strong>日本語</strong> · <a href="README.ko.md">한국어</a> · <a href="README.de.md">Deutsch</a> · <a href="README.fr.md">Français</a> · <a href="README.es.md">Español</a> · <a href="README.pt-BR.md">Português (Brasil)</a>
</p>

<p align="center"><strong>画面上の仕事を記録して、一日を振り返る。</strong></p>

Daygo は macOS と Windows 向けの作業記録・振り返りツールです。バックグラウンドで一定間隔のスクリーンショットを撮影し、設定した AI が画面上の活動を**タイムライン、日次レビュー、週次まとめ**に整理します。朝会や振り返り、作業の詳細を思い出すために役立ちます。

**一定間隔のスクリーンショット · ローカル保存 · 選べる AI · 9 言語のインターフェース**

[ダウンロードとインストール](#ダウンロードとインストール) · [使い始める](#使い始める) · [機能プレビュー](#機能プレビュー) · [プライバシーとデータ](#プライバシーとデータ) · [開発への参加](#開発への参加)

## ダウンロードとインストール

[最新の正式リリースの Assets](https://github.com/Jwz-git/Daygo/releases/latest) から、プラットフォームに合ったインストーラーを選んでください。

| プラットフォーム | システムとアーキテクチャ | インストーラー |
|---|---|---|
| macOS | macOS 14+、Apple Silicon（arm64） | `Daygo-<バージョン>-arm64.dmg` |
| Windows | Windows 11 24H2+（build 26100+）、x64（amd64） | `Daygo-<バージョン>-amd64-installer.exe` |

主な開発プラットフォームは macOS で、Windows 向けには x64 インストーラーを提供しています。他のアーキテクチャと Linux 向けのインストーラーは未公開です。ソースコードの進捗はリリース版より先行する場合があります。機能の状態は [モジュール一覧](../docs/09-roadmap.md#91-模块总表)、署名・公証・インストールの検証記録は [配布と更新](../docs/modules/delivery.md) を参照してください。

## 使い始める

1. **インストールと権限の許可**：Daygo を起動します。macOS では画面収録の権限を許可し、アプリの案内に従って再起動してください。
2. **AI の設定**：設定でサービスの URL、API キー、モデルを追加します。保存後に「モデルのテストと試用」を開き、テキストまたは画像 1 枚で応答を確認します。
3. **記録方法の設定**：撮影間隔、除外するアプリ、ディスク使用量の上限を選び、記録を有効にします。
4. **結果の確認**：最初の分析バッチが完了したら、タイムラインでカードと元の画像を確認し、必要に応じて編集します。日次・週次ページで活動を振り返れます。

Daygo に AI サービスや利用枠は付属しません。自動分析には画像認識と、設定したプロトコルでの構造化出力への対応が必要です。試用ページで応答を受け取っても、分析全体が動作することの検証にはなりません。ローカルモデルにも同じ能力が必要です。

## 機能プレビュー

<sub>以下のスクリーンショットには匿名のサンプルデータを使用しています。</sub>

### 自動タイムライン

システムのメインディスプレイを一定間隔で撮影し、AI が活動を時刻・タイトル・要約・カテゴリ付きのカードに整理します。カードを開くと元の画像を確認でき、結果の編集・削除・再処理も可能です。

<img src="../docs/assets/readme/timeline.en.webp" width="100%" alt="タイムライン：時刻順の活動カードと詳細パネル" />

### 日次レビュー

一日の作業の流れを確認し、成果・完了した作業・課題を含む朝会用の要約を生成します。日記と日々の目標で、自分の考えも記録できます。

<img src="../docs/assets/readme/daily.en.webp" width="100%" alt="日次レビュー：作業の流れと朝会用の要約" />

### 週次レビュー

週間の作業の流れ、集中と気散じのヒートマップ、カテゴリの割合、よく使うアプリ、時間の配分で一週間を振り返れます。記録時間の合計からは System カテゴリを除外します。

<img src="../docs/assets/readme/weekly.en.webp" width="100%" alt="週次レビュー：カテゴリ別の利用アプリと時間の配分図" />

### バックグラウンド記録とカスタマイズ

| 機能 | 説明 |
|---|---|
| バックグラウンド記録 | ウィンドウを閉じても記録を継続。macOS のメニューバーや Windows の通知領域から再表示できます |
| 一時停止と再開 | 15 / 30 / 60 分、または無期限に停止可能。時間指定の停止は期限が来ると自動で再開します |
| システムイベント | スリープ・画面ロック・スクリーンセーバー中は撮影を停止。手動で無効にした記録はシステムイベントで再開しません |
| 撮影設定 | 間隔は 1 / 5 / 10 / 20 / 30 / 60 秒、既定値は 10 秒。高さは 720 / 1080 ピクセル、既定値は 1080 |
| AI サービス | OpenAI Chat Completions、OpenAI Responses、Anthropic Messages。サービスごとに複数モデルを設定でき、順序付きフォールバックに対応 |
| 外観とカテゴリ | ライト / ダーク / システムに合わせる。カテゴリの名前・順序・色を編集でき、ログイン時の起動と macOS Dock アイコンも設定可能 |

インターフェースは簡体字中国語、繁体字中国語、英語、日本語、韓国語、ドイツ語、フランス語、スペイン語、ブラジルポルトガル語に対応しています。

<details>
<summary>ダークモードを見る</summary>

<p><img src="../docs/assets/readme/dark.en.webp" width="100%" alt="Daygo のダークモード" /></p>

</details>

タイムライン・日記・目標の一日は現地時刻の **午前 4 時**から始まります。朝会用の要約は暦日で集計するため、深夜の活動が属する日付は表示によって異なる場合があります。

## プライバシーとデータ

- **ローカル保存**：スクリーンショット、タイムライン、日記、設定、データベースは端末に保存されます。Daygo 独自のバックエンド、アカウント、同期サービスはありません。
- **送信先は自分で選択**：画面データを端末外へ送る先は、明示的に設定した AI サービスだけです。対応するローカルモデルを使えば画面分析を端末内で実行できます。第三者サービスでのデータの扱いは、そのプライバシーポリシーに従います。
- **アプリの除外と前面アプリの保護**：除外対象のアプリを撮影画像から取り除きます。対象アプリが前面にある場合は、内容を含まない代替フレームを保存します。両方の保護を維持します。
- **OS の資格情報ストア**：API キーは macOS キーチェーンまたは Windows Credential Manager にのみ保存します。画面からは書き込みだけが可能で、読み出せません。アプリのデータベース、localStorage、エラーメッセージには入りません。

記録は個別のスクリーンショットで行い、継続的な画面録画ストリームは使用しません。利用分析とクラッシュ報告は既定で無効で、送信機能は未実装です。

アンインストール後もユーザーデータと資格情報は保持されます。データの扱いの詳細は [プライバシーとセキュリティ](../docs/07-privacy-security.md) を参照してください。

## 開発への参加

Go が業務ロジックとデータベースへの書き込みを担当し、プラットフォーム機能はインターフェースで分離します。Vue は生成した Wails バインディングを通して Go にアクセスします。日常の開発は `test`、安定版は `main` ブランチです。

macOS での開発には Go、Node.js/npm、Xcode Command Line Tools が必要です。Go の要件は [go.mod](../go.mod)、Node.js の要件は [開発用スクリプト](../scripts/dev.sh) を参照してください。

```bash
git clone --branch test https://github.com/Jwz-git/Daygo.git
cd Daygo
./scripts/gate.sh   # 初期準備、ビルド、Go / フロントエンドのテスト、ドキュメント検査
```

検査スクリプトは必要な順序でフロントエンド成果物と Wails バインディングを準備します。各 OS の開発コマンドは [スクリプト一覧](../scripts/README.md)、設計と貢献ルールは [設計ドキュメント](../docs/README.md) と [AGENTS.md](../AGENTS.md) を参照してください。

不具合は [Issue](https://github.com/Jwz-git/Daygo/issues) に OS、アプリのバージョン、再現手順、機密情報を除いたエラーを添えて報告してください。実際のスクリーンショット、データベース、API キーはアップロードしないでください。

## ライセンス

[MIT License](../LICENSE) で公開しています。

<sub><a href="https://github.com/JerryZLiu/Dayflow">Dayflow</a> に着想を得ています（MIT、© 2025 Jerry Liu）。</sub>
