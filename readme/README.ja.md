<p align="center">
  <img src="../docs/assets/readme/banner.en.webp" width="100%" alt="Daygo — 1日の終わりに、自分が何に取り組んだか覚えていますか？" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><img src="https://img.shields.io/github/v/release/Jwz-git/Daygo?style=flat-square&color=F3854B&label=%E3%83%AA%E3%83%AA%E3%83%BC%E3%82%B9" alt="最新正式リリース" /></a>
  <img src="https://img.shields.io/badge/macOS-14%2B-333333?style=flat-square&logo=apple&logoColor=white" alt="macOS 14+" />
  <img src="https://img.shields.io/badge/Windows-11%2024H2%2B-0078D4?style=flat-square&logo=windows&logoColor=white" alt="Windows 11 24H2+" />
  <a href="../LICENSE"><img src="https://img.shields.io/badge/license-MIT-6E7DF7?style=flat-square" alt="MIT License" /></a>
  <img src="https://img.shields.io/badge/Go%20%C2%B7%20Wails%203%20%C2%B7%20Vue%203-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go · Wails 3 · Vue 3" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><b>ダウンロード</b></a> ·
  <a href="https://jwz-git.github.io/Daygo/"><b>公式サイト</b></a> ·
  <a href="../docs/README.md">設計ドキュメント</a>
</p>

<p align="center">
  <a href="../README.md">简体中文</a> · <a href="README.zh-Hant.md">繁體中文</a> · <a href="README.en.md">English</a> · <strong>日本語</strong> · <a href="README.ko.md">한국어</a> · <a href="README.de.md">Deutsch</a> · <a href="README.fr.md">Français</a> · <a href="README.es.md">Español</a> · <a href="README.pt-BR.md">Português (Brasil)</a>
</p>

<p align="center"><strong>手動タイマー不要。画面上の作業を自動記録し、毎日をはっきり振り返る。</strong></p>

Daygo は **macOS** と **Windows** 向けのローカルファーストな作業記録・振り返りツールです。バックグラウンドでメインディスプレイを一定間隔で離散キャプチャし、あなたが設定した AI を使って**自動タイムライン・日次スタンドアップ要約・週次レビュー・計画リマインダー**へと整理します。タイマーの開始やクラウドアカウントは不要です。

<p align="center"><b>離散スクリーンショット · ローカル保存 · 自由な AI 選択（ローカルモデル対応） · 9言語 UI</b></p>

<p align="center">
  <a href="#主な機能">主な機能</a> ·
  <a href="#ダウンロードとインストール">ダウンロードとインストール</a> ·
  <a href="#クイックスタート">クイックスタート</a> ·
  <a href="#プライバシーとセキュリティ">プライバシーとセキュリティ</a> ·
  <a href="#開発への参加">開発への参加</a>
</p>

---

## 主な機能

<sub>以下のスクリーンショットには匿名化されたサンプルデータを使用しています。</sub>

### 自動タイムライン

メインディスプレイを一定間隔で離散キャプチャし、AI が時間帯・タイトル・要約・カテゴリ付きのカードとして整理します。カードを展開すると元フレームを確認でき、日 / 週ビューの切り替え、編集・削除、単一カードまたは日単位の再処理、当日の失敗バッチ一括削除、インスペクターでの今日の計画ブロック作成にも対応しています。

<img src="../docs/assets/readme/timeline.en.webp" width="100%" alt="自動タイムライン：時系列の活動カードと詳細パネル" />

### 日次レビュー

1日のワークフローを一覧し、ハイライト・完了事項・ブロッカーをまとめたスタンドアップ要約を生成できます。ジャーナルと日次目標の記録、毎日のジャーナル通知リマインダー、レポート末尾への Token 使用量グラフ表示（デフォルトオフ）にも対応しています。

<img src="../docs/assets/readme/daily.en.webp" width="100%" alt="日次レビュー：ワークフロー概要とスタンドアップ要約" />

### 週次レビュー

週次ワークフロー、集中と気が散りのヒートマップ、カテゴリ割合、よく使ったアプリ、時間の流れから1週間を振り返れます。日次と同様に Token 使用量グラフを任意表示できます（計測時間から `System` カテゴリは除外されます）。

<img src="../docs/assets/readme/weekly.en.webp" width="100%" alt="週次レビュー：カテゴリ別よく使ったアプリと時間の流れ" />

<details>
<summary>ダークモードを見る</summary>

<p><img src="../docs/assets/readme/dark.en.webp" width="100%" alt="Daygo ダークモードのプレビュー" /></p>

</details>

### バックグラウンド常駐とカスタマイズ

| 機能 | 内容 |
|---|---|
| バックグラウンド記録 | メインウィンドウを閉じても記録を継続（macOS では Dock 実行インジケーターも非表示）。macOS メニューバーまたは Windows 通知領域からいつでも再表示可能 |
| 一時停止とシステムイベント | 15 / 30 / 60 分または無期限で一時停止（時間指定は自動で再開）。スリープ・ロック・スクリーンセーバー時に自動一時停止し、手動停止した記録はシステムイベントで自動再開しません |
| キャプチャと保存 | 間隔 1 / 5 / 10 / 20 / 30 / 60 秒（デフォルト 10 秒）、高さ 720 / 1080 px（デフォルト 1080）。ディスク容量上限とセグメント単位の削除に対応し、Windows では録画フォルダーの移行も可能 |
| マルチプロトコル AI とフォールバック | API Key は OpenAI Chat Completions、OpenAI Responses、Anthropic Messages、Google Gemini（条件を満たす確定済み MP4 セグメントは直接送信を優先し、非該当時はスクリーンショットへフォールバック）に対応。ChatGPT サブスクリプション OAuth、単一プロバイダー複数モデル、順序付きフォールバックチェーンもサポート |
| 計画とシステム通知 | 今日の計画ブロックと完了チェック。macOS / Windows で毎日のジャーナル通知、計画開始通知、気が散り警告を送信 |
| 外観・カテゴリ・言語 | ライト / ダーク / システム連動。初回ガイドに職種別カテゴリプリセットを搭載し、名称・順序・色を自由に編集可能。ログイン時起動と 9 言語 UI に対応 |

> **午前 4 時区切りの論理日**：タイムライン・計画・ジャーナル・目標は現地時間の**午前 4:00** で1日を区切りますが、スタンドアップ要約は暦日単位で集計されます。深夜の作業はビューによって帰属日が異なる場合があります。

---

## ダウンロードとインストール

[最新正式リリース](https://github.com/Jwz-git/Daygo/releases/latest) の Assets からお使いの環境向けインストーラーを入手してください：

| プラットフォーム | 対応 OS とアーキテクチャ | パッケージ |
|---|---|---|
| macOS | macOS 14+、Apple Silicon（`arm64`） | `Daygo-<バージョン>-arm64.dmg` |
| Windows | Windows 11 24H2+（build 26100+）、x64（`amd64`） | `Daygo-<バージョン>-amd64-installer.exe` |

macOS がメイン開発プラットフォームであり、Windows は x64 インストーラーを提供しています。その他のアーキテクチャと Linux 向けリリースインストーラーは未提供です。ソースコードはリリース版より先行している場合があるため、実装状況は [モジュール総表](../docs/09-roadmap.md#91-模块总表)、署名・公証・更新の検証記録は [インストールと更新](../docs/modules/delivery.md) を参照してください。

---

## クイックスタート

1. **インストールと初回ガイド**：Daygo を起動し、職種別カテゴリプリセットと基本設定を選びます。macOS では案内に従い「画面収録」権限を許可して再起動してください。
2. **AI の設定**：ガイドまたは設定で **ChatGPT サブスクリプション**（ブラウザ OAuth）または **API Key**（エンドポイント・キー・手入力または取得したモデルの複数選択）を設定します。保存後、「モデルテストと試用」でテキストや単一画像の応答を確認できます。
3. **記録とプライバシー設定**：撮影間隔、ブロック対象アプリ、ディスク上限（Windows は録画フォルダー移行も可）を調整して記録を有効にします。
4. **振り返る**：最初の分析が完了したらタイムラインでカードと元フレームを確認し、必要に応じて編集・再処理します。日次・週次ページで全体を振り返れます。

> Daygo には AI サービスや利用枠は含まれていません。自動タイムライン分析には画像認識と対象プロトコルの構造化出力能力が必要です（ローカルモデルも同様）。試用ページで応答が得られても自動分析全体の適合が保証されるわけではありません。

---

## プライバシーとセキュリティ

- **完全ローカル保存**：スクリーンショットのセグメント、タイムライン、ジャーナル、計画、設定、SQLite データベースはすべて端末内に保存されます。独自バックエンド・アカウント・クラウド同期はありません。
- **送信先はあなたが選択**：画面データが端末外に出る唯一の経路は、あなたが明示的に設定した AI サービスです。対応ローカルモデルを使えば端末内のみで完結でき、外部サービス利用時は各社のプライバシーポリシーが適用されます。
- **アプリブロック＋前面マスキングの二重保護**：ブロック対象アプリはキャプチャから除外され、ブロック対象アプリが前面にある間は匿名化されたプレースホルダーフレームのみ保存します。
- **書き込み専用のシステム資格情報管理**：API Key と ChatGPT OAuth 資格情報はシステム資格情報ストア（macOS キーチェーン / Windows 資格情報マネージャー）のみに保存され、UI・バインディング層からは読み戻せず、データベース・`localStorage`・エラーメッセージにも含まれません。

記録は連続的な画面録画ストリームではなく**離散スクリーンショット**を使用します。利用状況分析とクラッシュ報告はデフォルトでオフです（送信コンシューマーは未実装）。アンインストールしてもユーザーデータと資格情報は保持されます。詳細は [プライバシーとセキュリティ](../docs/07-privacy-security.md) を参照してください。

---

## 開発への参加

日常開発ブランチ `test` は **Go 1.26.10 + Wails v3.1.0 + Vue 3** 構成、`main` は安定ブランチです（移行の詳細は [Wails v3 GA 移行記録](../docs/decisions/wails-v3-migration.md)）。Go がビジネスロジックと SQLite 書き込みを担い、プラットフォーム機能はインターフェース越しに分離し、Vue は生成された TypeScript バインディングで Go を呼び出します。

### 前提環境

- [go.mod](../go.mod) で指定された Go バージョン
- npm 同梱の Node.js `20.19+`（20.x）または `22.12+`
- **macOS**：Xcode Command Line Tools
- **Windows**：MinGW-w64（binutils 含む）、Visual Studio 2022 C++ ツール、Windows SDK 26100、WebView2 Runtime（詳細は [Windows ビルド依存関係](../scripts/README.md#windows-build-dependencies)）
- **Linux**：既定で GTK4 と WebKitGTK 6.0 開発パッケージ（arm64 ホストビルド検証済み、画面キャプチャとシステム統合は未実装）

### クローン・検証・起動

```bash
git clone --branch test https://github.com/Jwz-git/Daygo.git
cd Daygo
./scripts/gate.sh   # クリーンチェックアウトのブートストラップ、ビルド、Go / フロントエンドテスト、ドキュメント検査
```

| プラットフォーム | 開発アプリの起動 |
|---|---|
| macOS | `./scripts/dev.sh` |
| Windows | `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dev.ps1` |
| Linux | `./scripts/dev-linux.sh` |

ビルド・パッケージング手順は [scripts/README.md](../scripts/README.md)、設計仕様と開発規約は [設計ドキュメント](../docs/README.md) と [AGENTS.md](../AGENTS.md) を参照してください。

不具合の報告は [Issue](https://github.com/Jwz-git/Daygo/issues) へ OS・アプリバージョン・再現手順・機密除去済みエラーを添えてお送りください。**実際のスクリーンショット・データベース・API Key は絶対に添付しないでください。**

---

## 今後の予定

- プラグイン機能の開発
- Linux での画面キャプチャとシステム統合

---

## ライセンス

本プロジェクトは [MIT License](../LICENSE) のもとで公開されています。

<sub><a href="https://github.com/JerryZLiu/Dayflow">Dayflow</a>（MIT、© 2025 Jerry Liu）から着想を得ています。</sub>
