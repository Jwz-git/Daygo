export default {
  discardNote: '保存していない変更は破棄されます。',
  apply: '適用',
  cancel: 'キャンセル',
  applying: '適用中…',
  failed: '適用できませんでした。カテゴリは変更されていません。',
  palette: {
    title: 'カラーパレット',
    hint: 'パレットを選ぶと、すべてのカテゴリを順番に塗り直します。',
    confirmBody: '「{name}」ですべてのカテゴリを塗り直します。',
    names: {
      daygoClassic: 'Daygo クラシック',
      forestMoss: '静かな森',
      cyberNeon: 'サイバーネオン',
      nordicMist: '北欧ミスト',
      sunsetClay: '夕焼けテラコッタ',
    },
  },
  archetype: {
    label: 'アーキタイプ',
    hint: '3D ワールドでこのカテゴリがどんな建物や景観になるかを決めます。',
    forge: '創る',
    sanctum: '考える',
    nexus: 'つながる',
    camp: '休む',
    order: '整える',
  },
  profession: {
    softwareEngineer: {
      name: 'ソフトウェアエンジニア',
      categories: {
        coding: { name: 'コーディング / デバッグ', details: 'IDE やターミナルでのコードの作成、リファクタリング、修正' },
        codeReview: { name: 'コードレビュー', details: 'PR のレビュー、差分の確認、コメント' },
        research: { name: 'リサーチ', details: 'ドキュメントや Stack Overflow を読む、ツールや API を調べる、設計書や技術仕様を書く' },
        communication: { name: 'コミュニケーション', details: '会議、スタンドアップ、Slack、メール、ビデオ通話、メッセージ、同期の場' },
      },
    },
    founderExecutive: {
      name: '創業者 / 経営者',
      categories: {
        building: { name: 'エンジニアリング / プロダクト', details: 'コーディング、デザイン、機能のリリースなど自ら手を動かす作業' },
        strategy: { name: 'リサーチと戦略', details: '競合調査、ポジショニング、じっくり考える時間、投資家向けの準備' },
        insights: { name: 'データとインサイト', details: 'ダッシュボード、継続率、ファネル、財務' },
        communication: { name: 'コミュニケーション', details: 'チームの同期、投資家との通話、ユーザー向けデモ、採用' },
      },
    },
    designer: {
      name: 'デザイナー',
      categories: {
        design: { name: 'デザイン', details: 'プロトタイプ、UI コンポーネント、ユーザーフロー、ビジュアルデザイン、引き渡し仕様' },
        research: { name: 'リサーチ', details: 'デザインパターンの収集、競合の調査、ユーザー調査、指標の確認' },
        communication: { name: 'コミュニケーション', details: 'デザインレビュー、朝会、講評会、コンセプトの発表' },
      },
    },
    student: {
      name: '学生',
      categories: {
        studying: { name: '勉強', details: '講義、読書、スライドの復習、単語カード、授業の資料' },
        assignments: { name: '課題', details: 'レポート、問題演習、プログラミング課題、実験レポート' },
        communication: { name: 'コミュニケーション', details: '勉強会、オフィスアワー、グループチャット、教員へのメール' },
      },
    },
    productManager: {
      name: 'プロダクトマネージャー',
      categories: {
        specs: { name: '仕様と計画', details: 'PRD、ロードマップ、バックログ整理、スプリント計画、チケット' },
        research: { name: 'リサーチと分析', details: 'ユーザー調査、指標の確認、競合分析、A/B テスト' },
        communication: { name: 'コミュニケーション', details: '朝会、関係者との同期、デザインレビュー、エンジニアとの確認' },
      },
    },
    dataScientist: {
      name: 'データサイエンティスト',
      categories: {
        analysis: { name: '分析とモデリング', details: 'ノートブック、統計分析、機械学習の訓練、データ探索' },
        dataEngineering: { name: 'データエンジニアリング', details: 'SQL クエリ、パイプライン、データクレンジング、ETL スクリプト' },
        research: { name: 'リサーチ', details: '論文やドキュメントを読む、新しい手法やツールを試す' },
        communication: { name: 'コミュニケーション', details: '分析結果の発表、関係者との同期、チームでの議論' },
      },
    },
    other: {
      name: 'その他',
      categories: {
        work: { name: '仕事', details: 'より具体的なカテゴリに当てはまらない集中作業や業務' },
        communication: { name: 'コミュニケーション', details: '会議、スタンドアップ、Slack、メール、ビデオ通話、メッセージ、同期の場' },
      },
    },
  },
  common: {
    distraction: { name: '気が散る', details: '目的のない閲覧と受け身の消費：SNS のタイムライン、なんとなく見る動画、だらだらしたスクロール、意図のはっきりしない娯楽、ゲーム' },
    personal: { name: '個人', details: '目的のある仕事以外の活動：友人や家族との連絡、家計の管理、旅行の手配、用事、生活の雑務、趣味' },
  },
}
