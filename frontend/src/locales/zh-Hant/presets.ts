export default {
  archetype: {
    label: '元類型',
    hint: '用於決定該分類在 3D 世界中的建築與自然景觀形態。',
    forge: '創造',
    sanctum: '思考',
    nexus: '連結',
    camp: '休憩',
    order: '秩序',
  },
  profession: {
    softwareEngineer: {
      name: '軟體工程師',
      categories: {
        coding: { name: '程式設計 / 除錯', details: '在 IDE 或終端機裡撰寫、重構和修正程式碼' },
        codeReview: { name: '程式碼審查', details: '審查 PR、閱讀 diff 並留下評論' },
        research: { name: '研究', details: '閱讀文件和 Stack Overflow、了解工具與 API，以及撰寫設計文件或技術方案' },
        communication: { name: '溝通', details: '會議、站立會議、Slack、郵件、視訊通話、即時訊息和同步溝通' },
      },
    },
    founderExecutive: {
      name: '創辦人 / 管理者',
      categories: {
        building: { name: '工程 / 產品', details: '寫程式、做設計、發布功能等親自動手的建構工作' },
        strategy: { name: '研究與策略', details: '競品研究、定位、深度思考和募資準備' },
        insights: { name: '數據與洞察', details: '數據看板、留存資料、轉換漏斗和財務' },
        communication: { name: '溝通', details: '團隊同步、投資人通話、使用者展示和招募' },
      },
    },
    designer: {
      name: '設計師',
      categories: {
        design: { name: '設計', details: '原型、UI 元件、使用者流程、視覺設計和交付規格' },
        research: { name: '研究', details: '瀏覽設計模式、競品審查、使用者研究和查看指標' },
        communication: { name: '溝通', details: '設計審查、站立會議、評圖會和方案簡報' },
      },
    },
    student: {
      name: '學生',
      categories: {
        studying: { name: '學習', details: '聽課、閱讀、複習投影片、記憶卡和課程資料' },
        assignments: { name: '作業', details: '論文、習題、程式專案和實驗報告' },
        communication: { name: '溝通', details: '讀書會、答疑時間、群組聊天和寄信給教授' },
      },
    },
    productManager: {
      name: '產品經理',
      categories: {
        specs: { name: '規格與規劃', details: 'PRD、路線圖、待辦清單整理、衝刺規劃和工單' },
        research: { name: '研究與分析', details: '使用者研究、指標檢視、競品分析和 A/B 測試' },
        communication: { name: '溝通', details: '站立會議、利害關係人同步、設計審查和與工程對齊' },
      },
    },
    dataScientist: {
      name: '資料科學家',
      categories: {
        analysis: { name: '分析與建模', details: 'Notebook、統計分析、機器學習訓練和資料探索' },
        dataEngineering: { name: '資料工程', details: 'SQL 查詢、資料管線、資料清理和 ETL 腳本' },
        research: { name: '研究', details: '閱讀論文和文件，探索新的方法與工具' },
        communication: { name: '溝通', details: '報告結論、利害關係人同步和團隊討論' },
      },
    },
    other: {
      name: '其他',
      categories: {
        work: { name: '工作', details: '無法歸入更具體類別的專注工作和職責事務' },
        communication: { name: '溝通', details: '會議、站立會議、Slack、郵件、視訊通話、即時訊息和同步溝通' },
      },
    },
  },
  common: {
    distraction: { name: '分心', details: '沒有目的的瀏覽與被動消費內容：滑社群媒體、隨便看影片、無意義地捲動、沒有明確意圖的娛樂，以及玩遊戲' },
    personal: { name: '個人', details: '有明確目的的非工作活動：聯絡親友、處理財務、訂行程、辦事、生活雜事和興趣' },
  },
}
