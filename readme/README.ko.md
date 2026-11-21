<p align="center">
  <img src="../docs/assets/readme/banner.en.webp" width="100%" alt="Daygo — 하루가 끝날 때, 오늘 무엇을 했는지 기억나시나요?" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><img src="https://img.shields.io/github/v/release/Jwz-git/Daygo?style=flat-square&color=F3854B&label=%EB%A6%B4%EB%A6%AC%EC%8A%A4" alt="최신 정식 릴리스" /></a>
  <img src="https://img.shields.io/badge/macOS-14%2B-333333?style=flat-square&logo=apple&logoColor=white" alt="macOS 14+" />
  <img src="https://img.shields.io/badge/Windows-11%2024H2%2B-0078D4?style=flat-square&logo=windows&logoColor=white" alt="Windows 11 24H2+" />
  <a href="../LICENSE"><img src="https://img.shields.io/badge/license-MIT-6E7DF7?style=flat-square" alt="MIT License" /></a>
  <img src="https://img.shields.io/badge/Go%20%C2%B7%20Wails%203%20%C2%B7%20Vue%203-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go · Wails 3 · Vue 3" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><b>다운로드</b></a> ·
  <a href="https://jwz-git.github.io/Daygo/"><b>공식 웹사이트</b></a> ·
  <a href="../docs/README.md">설계 문서</a>
</p>

<p align="center">
  <a href="../README.md">简体中文</a> · <a href="README.zh-Hant.md">繁體中文</a> · <a href="README.en.md">English</a> · <a href="README.ja.md">日本語</a> · <strong>한국어</strong> · <a href="README.de.md">Deutsch</a> · <a href="README.fr.md">Français</a> · <a href="README.es.md">Español</a> · <a href="README.pt-BR.md">Português (Brasil)</a>
</p>

<p align="center"><strong>수동 타이머 없이 화면 작업을 자동 기록하고, 하루를 또렷하게 회고하세요.</strong></p>

Daygo는 **macOS**와 **Windows**용 로컬 우선 업무 기록·회고 도구입니다. 백그라운드에서 기본 디스플레이를 일정 간격으로 이산 캡처하고, 사용자가 설정한 AI로 **자동 타임라인, 일간 스탠드업 요약, 주간 회고, 일정 알림**을 정리합니다. 타이머를 누르거나 클라우드 계정을 만들 필요가 없습니다.

<p align="center"><b>이산 스크린샷 · 로컬 저장 · 원하는 AI 선택(로컬 모델 포함) · 9개 UI 언어</b></p>

<p align="center">
  <a href="#핵심-기능">핵심 기능</a> ·
  <a href="#다운로드-및-설치">다운로드 및 설치</a> ·
  <a href="#빠른-시작">빠른 시작</a> ·
  <a href="#개인정보-및-보안">개인정보 및 보안</a> ·
  <a href="#개발-참여">개발 참여</a>
</p>

---

## 핵심 기능

<sub>아래 스크린샷은 모두 익명 샘플 데이터를 사용합니다.</sub>

### 자동 타임라인

시스템 기본 디스플레이를 간격마다 이산 캡처하고, AI가 시간·제목·요약·카테고리가 담긴 카드로 정리합니다. 카드를 펼쳐 원본 프레임을 확인하고, 일간/주간 뷰 전환, 편집·삭제, 단일 카드 또는 하루 단위 재처리, 당일 실패 배치 일괄 삭제, 인스펙터에서의 오늘 계획 블록 등록을 지원합니다.

<img src="../docs/assets/readme/timeline.en.webp" width="100%" alt="자동 타임라인: 시간대별 활동 카드와 상세 패널" />

### 일간 회고

하루의 작업 흐름을 한눈에 보고, 하이라이트·완료 항목·블로커를 담은 스탠드업 요약을 생성합니다. 일기와 일일 목표를 기록하고, 매일 일기 알림을 켜거나 보고서 하단에 Token 사용량 차트(기본 꺼짐)를 선택적으로 표시할 수 있습니다.

<img src="../docs/assets/readme/daily.en.webp" width="100%" alt="일간 회고: 작업 흐름 개요와 스탠드업 요약" />

### 주간 회고

주간 작업 흐름, 집중·방해 히트맵, 카테고리 비중, 자주 쓴 앱, 시간 흐름으로 한 주를 돌아봅니다. 마찬가지로 Token 사용량 차트를 선택 표시할 수 있으며, 추적 시간 통계에서 `System` 카테고리는 제외됩니다.

<img src="../docs/assets/readme/weekly.en.webp" width="100%" alt="주간 회고: 카테고리별 주요 앱과 시간 흐름 차트" />

<details>
<summary>다크 모드 보기</summary>

<p><img src="../docs/assets/readme/dark.en.webp" width="100%" alt="Daygo 다크 모드 미리보기" /></p>

</details>

### 백그라운드 상주 및 맞춤 설정

| 기능 | 설명 |
|---|---|
| 백그라운드 기록 | 메인 창을 닫아도 녹화가 계속되며(macOS는 Dock 실행 표시도 숨김), macOS 메뉴 막대나 Windows 알림 영역에서 언제든 다시 열 수 있습니다 |
| 일시 중지 및 시스템 이벤트 | 15 / 30 / 60분 또는 무기한 일시 중지(시간 지정 시 자동 재개). 절전·화면 잠금·화면 보호기 시 자동 중지되며, 사용자가 직접 끈 녹화는 시스템 이벤트로 다시 켜지지 않습니다 |
| 캡처 및 저장소 | 간격 1 / 5 / 10 / 20 / 30 / 60초(기본 10초), 높이 720 / 1080 px(기본 1080). 디스크 용량 상한 및 세그먼트 단위 정리 지원, Windows는 녹화 폴더 이전 지원 |
| 멀티 프로토콜 AI 및 폴백 | API Key는 OpenAI Chat Completions, OpenAI Responses, Anthropic Messages, Google Gemini(조건에 맞는 완료된 MP4 세그먼트 우선 직접 전송, 미충족 시 스크린샷 폴백) 지원. ChatGPT 구독 OAuth, 단일 서비스 다중 모델 및 순차 폴백 체인 지원 |
| 계획 및 시스템 알림 | 오늘 계획 블록과 완료 표시. macOS / Windows에서 일일 일기 알림, 계획 시작 및 주의 분산 시스템 알림 지원 |
| 테마·카테고리·언어 | 라이트 / 다크 / 시스템 따르기. 첫 실행 가이드에서 직군별 카테고리 프리셋 제공(이름·순서·색상 편집 가능), 로그인 시 자동 실행 및 9개 UI 언어 지원 |

> **오전 4시 논리 일자**: 타임라인·계획·일기·목표는 현지 시각 **오전 4:00**을 기준으로 하루를 나누며, 스탠드업 요약은 달력 날짜 기준으로 집계됩니다. 심야 작업은 화면에 따라 귀속 날짜가 다를 수 있습니다.

---

## 다운로드 및 설치

[최신 정식 릴리스](https://github.com/Jwz-git/Daygo/releases/latest) Assets에서 환경에 맞는 설치 파일을 다운로드하세요.

| 플랫폼 | OS 및 아키텍처 | 패키지 |
|---|---|---|
| macOS | macOS 14+, Apple Silicon(`arm64`) | `Daygo-<버전>-arm64.dmg` |
| Windows | Windows 11 24H2+(build 26100+), x64(`amd64`) | `Daygo-<버전>-amd64-installer.exe` |

macOS가 주력 개발 플랫폼이며 Windows는 x64 설치 프로그램을 제공합니다. 그 외 아키텍처와 Linux용 릴리스 패키지는 아직 제공되지 않습니다. 소스 코드는 배포판보다 앞서 있을 수 있으며, 구현 상태는 [모듈 현황표](../docs/09-roadmap.md#91-模块总表), 서명·공증·설치 기록은 [설치 및 업데이트](../docs/modules/delivery.md)를 참고하세요.

---

## 빠른 시작

1. **설치 및 초기 안내**: Daygo를 실행해 직군 카테고리 프리셋과 기본 설정을 선택합니다. macOS에서는 안내에 따라 ‘화면 기록’ 권한을 허용하고 요청 시 재시작하세요.
2. **AI 연결**: 초기 안내 또는 설정에서 **ChatGPT 구독**(브라우저 OAuth) 또는 **API Key**(엔드포인트·키·직접 입력 또는 불러온 모델 다중 선택)를 설정합니다. 저장 후 ‘모델 테스트 및 체험’에서 텍스트나 이미지 한 장으로 응답을 확인하세요.
3. **기록 및 개인정보 설정**: 캡처 간격, 차단 앱 목록, 디스크 한도(Windows는 녹화 폴더 이전 가능)를 조정한 뒤 녹화를 켭니다.
4. **결과 회고**: 첫 분석이 끝나면 타임라인에서 카드와 원본 프레임을 확인·편집·재처리하고, 일간·주간 페이지에서 하루와 한 주를 돌아봅니다.

> Daygo는 AI 서비스나 사용량을 기본 제공하지 않습니다. 자동 타임라인 분석에는 이미지 이해와 해당 프로토콜의 구조화 출력 기능이 필요하며(로컬 모델 동일), 체험 페이지 응답 성공이 자동 분석 파이프라인 전체 검증을 뜻하지는 않습니다.

---

## 개인정보 및 보안

- **로컬 우선 저장**: 스크린샷 세그먼트, 타임라인, 일기, 계획, 설정, SQLite 데이터베이스가 모두 사용자 기기에 보관됩니다. 자체 백엔드, 계정, 클라우드 동기화가 없습니다.
- **데이터 전송 경로 직접 선택**: 화면 데이터가 기기를 떠나는 유일한 경로는 사용자가 명시적으로 설정한 AI 서비스입니다. 호환 로컬 모델을 쓰면 기기 안에서만 분석할 수 있으며, 외부 서비스는 해당 제공업체의 개인정보 처리방침을 따릅니다.
- **앱 차단 + 포그라운드 마스킹 이중 보호**: 차단 앱은 캡처에서 제외되며, 차단 앱이 포그라운드에 있을 때는 마스킹된 플레이스홀더 프레임만 저장합니다.
- **쓰기 전용 시스템 자격 증명 저장소**: API Key와 ChatGPT OAuth 자격 증명은 시스템 자격 증명 저장소(macOS 키체인 / Windows 자격 증명 관리자)에만 저장되며, UI와 바인딩 계층에서 원문을 읽을 수 없고 데이터베이스·`localStorage`·오류 메시지에도 남지 않습니다.

연속 화면 녹화 스트림이 아닌 **이산 스크린샷** 방식을 사용합니다. 사용 분석과 크래시 보고는 기본 꺼짐 상태입니다(전송 컨슈머 미구현). 앱을 제거해도 사용자 데이터와 자격 증명은 유지됩니다. 자세한 경계는 [개인정보 및 보안](../docs/07-privacy-security.md)을 참고하세요.

---

## 개발 참여

일상 개발 브랜치 `test`는 **Go 1.26.10 + Wails v3.1.0 + Vue 3** 기반이며 `main`이 안정 브랜치입니다([Wails v3 GA 마이그레이션 기록](../docs/decisions/wails-v3-migration.md) 참고). Go가 비즈니스 로직과 SQLite 쓰기를 담당하고, 플랫폼 기능은 인터페이스로 격리되며, Vue는 생성된 TypeScript 바인딩으로 Go와 통신합니다.

### 요구 환경

- [go.mod](../go.mod)에 명시된 Go 버전
- npm이 포함된 Node.js `20.19+`(20.x) 또는 `22.12+`
- **macOS**: Xcode Command Line Tools
- **Windows**: MinGW-w64(binutils 포함), Visual Studio 2022 C++ 도구, Windows SDK 26100, WebView2 Runtime([Windows 빌드 의존성](../scripts/README.md#windows-build-dependencies) 참고)
- **Linux**: 기본 GTK4 및 WebKitGTK 6.0 개발 패키지(arm64 호스트 빌드 검증 완료, 화면 캡처 및 시스템 통합은 구현 예정)

### 클론, 검증 및 실행

```bash
git clone --branch test https://github.com/Jwz-git/Daygo.git
cd Daygo
./scripts/gate.sh   # 클린 체크아웃 부트스트랩, 빌드, Go / 프런트엔드 테스트 및 문서 검사
```

| 플랫폼 | 개발 앱 실행 |
|---|---|
| macOS | `./scripts/dev.sh` |
| Windows | `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dev.ps1` |
| Linux | `./scripts/dev-linux.sh` |

빌드·패키징 명령은 [scripts/README.md](../scripts/README.md), 설계 사양과 개발 규칙은 [설계 문서](../docs/README.md) 및 [AGENTS.md](../AGENTS.md)를 참고하세요.

버그 제보는 [Issue](https://github.com/Jwz-git/Daygo/issues)에 OS, 앱 버전, 재현 절차, 민감 정보가 제거된 오류 메시지를 함께 남겨 주세요. **실제 스크린샷, 데이터베이스, API Key는 절대 업로드하지 마세요.**

---

## 향후 계획

- 플러그인 기능 개발
- Linux 화면 캡처 및 시스템 통합

---

## 라이선스

본 프로젝트는 [MIT License](../LICENSE)에 따라 오픈소스로 배포됩니다.

<sub><a href="https://github.com/JerryZLiu/Dayflow">Dayflow</a>(MIT, © 2025 Jerry Liu)에서 영감을 받았습니다.</sub>
