<p align="center">
  <img src="../docs/assets/readme/banner.en.webp" width="100%" alt="Daygo — 一天結束，你還記得自己做了什麼嗎？" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><img src="https://img.shields.io/github/v/release/Jwz-git/Daygo?style=flat-square&color=F3854B&label=version" alt="最新正式版本" /></a>
  <img src="https://img.shields.io/badge/macOS-14%2B-333333?style=flat-square&logo=apple&logoColor=white" alt="macOS 14+" />
  <img src="https://img.shields.io/badge/Windows-11%2024H2%2B-0078D4?style=flat-square" alt="Windows 11 24H2+" />
  <a href="../LICENSE"><img src="https://img.shields.io/badge/license-MIT-6E7DF7?style=flat-square&color=6E7DF7" alt="MIT License" /></a>
  <img src="https://img.shields.io/badge/Go%20%C2%B7%20Wails%20%C2%B7%20Vue%203-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go · Wails · Vue 3" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><b>下載</b></a> ·
  <a href="https://jwz-git.github.io/Daygo/"><b>官網</b></a> ·
  <a href="../docs/README.md">設計文件</a>
</p>

<p align="center">
  <a href="../README.md">简体中文</a> · <strong>繁體中文</strong> · <a href="README.en.md">English</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <a href="README.de.md">Deutsch</a> · <a href="README.fr.md">Français</a> · <a href="README.es.md">Español</a> · <a href="README.pt-BR.md">Português (Brasil)</a>
</p>

<p align="center"><strong>記錄螢幕上的工作，回顧自己的每一天。</strong></p>

Daygo 是面向 macOS 與 Windows 的工作記錄與回顧工具。它在背景按間隔截圖，用你設定的 AI 將螢幕活動整理成**時間軸、每日回顧和每週總結**，為站會、回顧和回憶工作細節提供記錄。

**離散截圖 · 本機儲存 · 自選 AI · 九種介面語言**

[下載與安裝](#下載與安裝) · [快速上手](#快速上手) · [功能預覽](#功能預覽) · [隱私與資料](#隱私與資料) · [參與開發](#參與開發)

## 下載與安裝

在 [最新正式版的 Assets](https://github.com/Jwz-git/Daygo/releases/latest) 中選擇對應安裝檔：

| 平台 | 系統與架構 | 安裝檔 |
|---|---|---|
| macOS | macOS 14+，Apple Silicon（arm64） | `Daygo-<版本>-arm64.dmg` |
| Windows | Windows 11 24H2+（build 26100+），x64（amd64） | `Daygo-<版本>-amd64-installer.exe` |

macOS 是主要開發平台，Windows 提供 x64 安裝檔；其他架構與 Linux 暫無發布安裝檔。原始碼進度可能領先發布版，功能狀態見 [模組總表](../docs/09-roadmap.md#91-模块总表)，簽章、公證與安裝驗證記錄見 [安裝與更新](../docs/modules/delivery.md)。

## 快速上手

1. **安裝並授權**：啟動 Daygo；macOS 按提示授予螢幕錄製權限，並在應用程式提示時重新啟動。
2. **設定 AI**：在設定中加入服務網址、API Key 和模型，儲存後開啟「模型測試與試用」，用文字或一張圖片檢查回覆。
3. **設定記錄方式**：選擇截圖間隔、要封鎖的應用程式和磁碟使用上限，再啟用錄製。
4. **查看結果**：等待首批分析完成，在時間軸中查看卡片與原始影格，按需編輯；到每日、每週頁面回顧活動。

Daygo 不附帶 AI 服務或額度。自動分析需要圖像辨識與對應協定的結構化輸出能力；試用頁收到回覆不代表自動分析能力已經驗證。本機模型也需滿足這些要求。

## 功能預覽

<sub>以下截圖使用匿名範例資料。</sub>

### 自動時間軸

按間隔擷取系統主顯示器，AI 將活動整理成帶有時間、標題、摘要和分類的卡片。展開卡片可查看原始影格，也能編輯、刪除或重新處理結果。

<img src="../docs/assets/readme/timeline.en.webp" width="100%" alt="時間軸：按時段排列的活動卡片與詳細資訊面板" />

### 每日回顧

查看一天的工作流程，產生包含亮點、完成項目和阻礙的站會摘要；記錄日記和每日目標，為回顧補充自己的想法。

<img src="../docs/assets/readme/daily.en.webp" width="100%" alt="每日回顧：工作流程概覽與站會摘要" />

### 每週回顧

從每週工作流程、專注與分心熱圖、分類占比、常用應用程式和時間流向回顧一週。追蹤時長統計排除 System 分類。

<img src="../docs/assets/readme/weekly.en.webp" width="100%" alt="每週回顧：分類常用應用程式與時間流向圖" />

### 背景常駐與自訂

| 功能 | 說明 |
|---|---|
| 背景記錄 | 關閉視窗後繼續錄製，從 macOS 狀態列或 Windows 通知區重新開啟 |
| 暫停與恢復 | 可暫停 15 / 30 / 60 分鐘或一直暫停；定時暫停到期自動恢復 |
| 系統事件 | 睡眠、鎖定螢幕、螢幕保護程式期間暫停擷取；使用者主動關閉錄製後不會被系統事件重新開啟 |
| 截圖設定 | 間隔 1 / 5 / 10 / 20 / 30 / 60 秒，預設 10 秒；高度 720 / 1080 像素，預設 1080 |
| AI 服務 | OpenAI Chat Completions、OpenAI Responses、Anthropic Messages；單一服務多模型與有序備援鏈 |
| 外觀與分類 | 淺色 / 深色 / 跟隨系統；可編輯分類名稱、順序、配色；可設定登入時啟動及 macOS Dock 圖示 |

介面支援簡體中文、繁體中文、英文、日文、韓文、德文、法文、西班牙文和巴西葡萄牙文。

<details>
<summary>查看深色模式</summary>

<p><img src="../docs/assets/readme/dark.en.webp" width="100%" alt="Daygo 深色模式" /></p>

</details>

時間軸、日記和目標以本機凌晨 **4 點**劃分一天；站會摘要按日曆日彙整。深夜活動的日期歸屬可能因此不同。

## 隱私與資料

- **本機儲存**：截圖、時間軸、日記、設定與資料庫存放在你的裝置上，Daygo 沒有自有後端、帳號或同步服務。
- **由你選擇資料去向**：螢幕資料離開裝置的唯一路徑是你明確設定的 AI 服務。使用相容的本機模型時，螢幕分析可在本機完成；第三方服務的資料處理遵循其隱私政策。
- **應用程式封鎖與前景去識別化**：從截圖中排除被封鎖的應用程式；前景應用程式被封鎖時，儲存去識別化的佔位影格。兩層保護同時保留。
- **系統認證資料儲存**：API Key 只存 macOS 鑰匙圈或 Windows Credential Manager，介面只寫不讀，不進入專案資料庫、localStorage 或錯誤訊息。

記錄使用離散截圖，避免持續開啟螢幕錄製串流。使用分析與當機回報預設關閉，回報功能尚未實作。

解除安裝應用程式會保留使用者資料與認證資料。完整的資料邊界見 [隱私與安全](../docs/07-privacy-security.md)。

## 參與開發

Go 負責業務邏輯與資料庫寫入，平台能力透過介面隔離，Vue 透過產生的 Wails 繫結存取 Go。`test` 是日常開發分支，`main` 是穩定分支。

macOS 開發需安裝 Go、Node.js/npm 和 Xcode Command Line Tools；Go 版本要求見 [go.mod](../go.mod)，Node.js 要求見 [開發入口](../scripts/dev.sh)。

```bash
git clone --branch test https://github.com/Jwz-git/Daygo.git
cd Daygo
./scripts/gate.sh   # 引導、建置、Go / 前端測試與文件檢查
```

檢查腳本會依序準備前端產物與 Wails 繫結。各平台開發命令見 [腳本入口](../scripts/README.md)，設計與貢獻約定見 [設計文件](../docs/README.md) 和 [AGENTS.md](../AGENTS.md)。

回報問題請提交 [Issue](https://github.com/Jwz-git/Daygo/issues)，附上系統、應用程式版本、重現步驟與去識別化的錯誤訊息；請勿上傳真實截圖、資料庫或 API Key。

## 授權條款

本專案以 [MIT License](../LICENSE) 開源。

<sub>由 <a href="https://github.com/JerryZLiu/Dayflow">Dayflow</a> 啟發（MIT，© 2025 Jerry Liu）。</sub>
