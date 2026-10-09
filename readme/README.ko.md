<p align="center">
  <img src="../docs/assets/readme/banner.en.webp" width="100%" alt="Daygo — 하루가 끝나면 어떤 일을 했는지 기억하시나요?" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><img src="https://img.shields.io/github/v/release/Jwz-git/Daygo?style=flat-square&color=F3854B&label=version" alt="최신 정식 버전" /></a>
  <img src="https://img.shields.io/badge/macOS-14%2B-333333?style=flat-square&logo=apple&logoColor=white" alt="macOS 14+" />
  <img src="https://img.shields.io/badge/Windows-11%2024H2%2B-0078D4?style=flat-square" alt="Windows 11 24H2+" />
  <a href="../LICENSE"><img src="https://img.shields.io/badge/license-MIT-6E7DF7?style=flat-square&color=6E7DF7" alt="MIT License" /></a>
  <img src="https://img.shields.io/badge/Go%20%C2%B7%20Wails%20%C2%B7%20Vue%203-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go · Wails · Vue 3" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><b>다운로드</b></a> ·
  <a href="https://jwz-git.github.io/Daygo/"><b>공식 사이트</b></a> ·
  <a href="../docs/README.md">설계 문서</a>
</p>

<p align="center">
  <a href="../README.md">简体中文</a> · <a href="README.zh-Hant.md">繁體中文</a> · <a href="README.en.md">English</a> · <a href="README.ja.md">日本語</a> · <strong>한국어</strong> · <a href="README.de.md">Deutsch</a> · <a href="README.fr.md">Français</a> · <a href="README.es.md">Español</a> · <a href="README.pt-BR.md">Português (Brasil)</a>
</p>

<p align="center"><strong>화면 속 작업을 기록하고 하루를 돌아보세요.</strong></p>

Daygo는 macOS와 Windows용 작업 기록 및 회고 도구입니다. 백그라운드에서 일정 간격으로 스크린샷을 찍고, 사용자가 설정한 AI로 화면 활동을 **타임라인, 일일 회고, 주간 요약**으로 정리합니다. 스탠드업 미팅과 회고, 작업 세부 내용을 떠올릴 때 활용할 수 있습니다.

**일정 간격 스크린샷 · 로컬 저장 · 직접 선택하는 AI · 9개 인터페이스 언어**

[다운로드 및 설치](#다운로드-및-설치) · [빠른 시작](#빠른-시작) · [기능 미리보기](#기능-미리보기) · [개인정보와 데이터](#개인정보와-데이터) · [개발 참여](#개발-참여)

## 다운로드 및 설치

[최신 정식 버전의 Assets](https://github.com/Jwz-git/Daygo/releases/latest)에서 플랫폼에 맞는 설치 파일을 선택하세요.

| 플랫폼 | 시스템 및 아키텍처 | 설치 파일 |
|---|---|---|
| macOS | macOS 14+, Apple Silicon(arm64) | `Daygo-<버전>-arm64.dmg` |
| Windows | Windows 11 24H2+(build 26100+), x64(amd64) | `Daygo-<버전>-amd64-installer.exe` |

주 개발 플랫폼은 macOS이며 Windows용 x64 설치 파일도 제공합니다. 다른 아키텍처와 Linux용 설치 파일은 아직 공개되지 않았습니다. 소스 코드가 배포 버전보다 앞서 있을 수 있습니다. 기능 상태는 [모듈 현황](../docs/09-roadmap.md#91-模块总表), 서명·공증·설치 검증 기록은 [설치 및 업데이트](../docs/modules/delivery.md)를 확인하세요.

## 빠른 시작

1. **설치 및 권한 허용**: Daygo를 실행하세요. macOS에서는 화면 캡처 권한을 허용하고 앱의 안내에 따라 다시 시작하세요.
2. **AI 설정**: 설정에서 서비스 URL, API 키, 모델을 추가하세요. 저장 후 ‘모델 테스트 및 체험’을 열어 텍스트나 이미지 한 장으로 응답을 확인하세요.
3. **기록 방식 선택**: 스크린샷 간격, 차단할 앱, 디스크 사용량 제한을 정한 뒤 기록을 활성화하세요.
4. **결과 확인**: 첫 분석이 완료되면 타임라인에서 카드와 원본 프레임을 확인하고 필요한 부분을 편집하세요. 일일·주간 페이지에서 활동을 돌아볼 수 있습니다.

Daygo에는 AI 서비스나 이용 크레딧이 포함되지 않습니다. 자동 분석에는 이미지 인식과 해당 프로토콜의 구조화된 출력 지원이 필요합니다. 체험 페이지에서 응답을 받았다고 해서 전체 분석 기능이 검증된 것은 아닙니다. 로컬 모델에도 같은 기능이 필요합니다.

## 기능 미리보기

<sub>아래 스크린샷은 익명 예시 데이터를 사용합니다.</sub>

### 자동 타임라인

시스템의 주 디스플레이를 일정 간격으로 캡처하고, AI가 활동을 시간·제목·요약·분류가 있는 카드로 정리합니다. 카드를 펼쳐 원본 프레임을 확인하고 결과를 편집, 삭제하거나 다시 처리할 수 있습니다.

<img src="../docs/assets/readme/timeline.en.webp" width="100%" alt="타임라인: 시간순 활동 카드와 상세 패널" />

### 일일 회고

하루의 작업 흐름을 확인하고 주요 성과, 완료한 작업, 진행을 막는 문제를 담은 스탠드업 요약을 생성하세요. 일기와 일일 목표에 자신의 생각을 더할 수 있습니다.

<img src="../docs/assets/readme/daily.en.webp" width="100%" alt="일일 회고: 작업 흐름과 스탠드업 요약" />

### 주간 회고

주간 작업 흐름, 집중과 산만함 히트맵, 분류별 비율, 자주 사용하는 앱, 시간 흐름으로 한 주를 돌아보세요. 추적 시간 합계에서 System 분류는 제외됩니다.

<img src="../docs/assets/readme/weekly.en.webp" width="100%" alt="주간 회고: 분류별 주요 앱과 시간 흐름도" />

### 백그라운드 기록과 맞춤 설정

| 기능 | 설명 |
|---|---|
| 백그라운드 기록 | 창을 닫아도 기록이 계속됩니다. macOS 메뉴 막대나 Windows 알림 영역에서 다시 열 수 있습니다 |
| 일시 중지와 재개 | 15 / 30 / 60분 또는 무기한 중지할 수 있으며, 시간 지정 중지는 만료되면 자동으로 재개됩니다 |
| 시스템 이벤트 | 잠자기, 화면 잠금, 화면 보호기 실행 중에는 캡처를 중지합니다. 직접 끈 기록은 시스템 이벤트로 다시 켜지지 않습니다 |
| 스크린샷 설정 | 간격 1 / 5 / 10 / 20 / 30 / 60초, 기본값 10초. 높이 720 / 1080픽셀, 기본값 1080 |
| AI 서비스 | OpenAI Chat Completions, OpenAI Responses, Anthropic Messages. 서비스별 여러 모델 및 순서에 따른 대체 서비스 연결 지원 |
| 외관과 분류 | 라이트 / 다크 / 시스템 테마. 분류 이름·순서·색상 편집, 로그인 시 실행 및 macOS Dock 아이콘 설정 |

인터페이스는 중국어 간체, 중국어 번체, 영어, 일본어, 한국어, 독일어, 프랑스어, 스페인어, 브라질 포르투갈어를 지원합니다.

<details>
<summary>다크 모드 보기</summary>

<p><img src="../docs/assets/readme/dark.en.webp" width="100%" alt="Daygo 다크 모드" /></p>

</details>

타임라인, 일기, 목표의 하루는 현지 시간 **오전 4시**에 시작합니다. 스탠드업 요약은 달력 날짜를 기준으로 집계하므로 심야 활동의 날짜가 서로 다를 수 있습니다.

## 개인정보와 데이터

- **로컬 저장**: 스크린샷, 타임라인, 일기, 설정, 데이터베이스는 기기에 저장됩니다. Daygo 자체 백엔드, 계정, 동기화 서비스는 없습니다.
- **사용자가 정하는 전송 대상**: 화면 데이터는 사용자가 명시적으로 설정한 AI 서비스로만 기기 밖에 전송됩니다. 호환되는 로컬 모델을 사용하면 화면 분석을 기기에서 실행할 수 있습니다. 외부 서비스의 데이터 처리는 해당 개인정보 처리방침을 따릅니다.
- **앱 차단과 전경 앱 가림 처리**: 차단된 앱은 스크린샷에서 제외됩니다. 차단된 앱이 전경에 있으면 내용을 담지 않는 대체 프레임을 저장합니다. 두 보호 조치를 함께 유지합니다.
- **시스템 자격 증명 저장소**: API 키는 macOS 키체인 또는 Windows Credential Manager에만 저장됩니다. 인터페이스에서는 쓰기만 가능하고 읽을 수 없으며, 앱 데이터베이스·localStorage·오류 메시지에 포함되지 않습니다.

기록은 개별 스크린샷 방식이며 연속 화면 녹화 스트림을 사용하지 않습니다. 사용 분석과 충돌 보고는 기본적으로 꺼져 있고, 전송 기능은 아직 구현되지 않았습니다.

앱을 제거해도 사용자 데이터와 자격 증명은 유지됩니다. 전체 데이터 경계는 [개인정보 및 보안](../docs/07-privacy-security.md)을 확인하세요.

## 개발 참여

Go가 비즈니스 로직과 데이터베이스 쓰기를 담당하고, 플랫폼 기능은 인터페이스로 분리합니다. Vue는 생성된 Wails 바인딩을 통해 Go에 접근합니다. `test`는 개발 브랜치, `main`은 안정 브랜치입니다.

macOS 개발에는 Go, Node.js/npm, Xcode Command Line Tools가 필요합니다. Go 버전 요구 사항은 [go.mod](../go.mod), Node.js 요구 사항은 [개발 스크립트](../scripts/dev.sh)를 확인하세요.

```bash
git clone --branch test https://github.com/Jwz-git/Daygo.git
cd Daygo
./scripts/gate.sh   # 초기 준비, 빌드, Go / 프런트엔드 테스트 및 문서 검사
```

검사 스크립트는 필요한 순서대로 프런트엔드 산출물과 Wails 바인딩을 준비합니다. 플랫폼별 개발 명령은 [스크립트 안내](../scripts/README.md), 설계와 기여 규칙은 [설계 문서](../docs/README.md) 및 [AGENTS.md](../AGENTS.md)를 확인하세요.

문제는 [Issue](https://github.com/Jwz-git/Daygo/issues)에 시스템, 앱 버전, 재현 절차, 민감 정보를 제거한 오류를 첨부해 알려주세요. 실제 스크린샷, 데이터베이스, API 키는 업로드하지 마세요.

## 라이선스

[MIT License](../LICENSE)로 공개합니다.

<sub><a href="https://github.com/JerryZLiu/Dayflow">Dayflow</a>에서 영감을 받았습니다(MIT, © 2025 Jerry Liu).</sub>
