<p align="center">
  <img src="../docs/assets/readme/banner.en.webp" width="100%" alt="Daygo — Wissen Sie am Ende des Tages noch, woran Sie gearbeitet haben?" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><img src="https://img.shields.io/github/v/release/Jwz-git/Daygo?style=flat-square&color=F3854B&label=Release" alt="Aktuelle Version" /></a>
  <img src="https://img.shields.io/badge/macOS-14%2B-333333?style=flat-square&logo=apple&logoColor=white" alt="macOS 14+" />
  <img src="https://img.shields.io/badge/Windows-11%2024H2%2B-0078D4?style=flat-square&logo=windows&logoColor=white" alt="Windows 11 24H2+" />
  <a href="../LICENSE"><img src="https://img.shields.io/badge/license-MIT-6E7DF7?style=flat-square" alt="MIT License" /></a>
  <img src="https://img.shields.io/badge/Go%20%C2%B7%20Wails%203%20%C2%B7%20Vue%203-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go · Wails 3 · Vue 3" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><b>Herunterladen</b></a> ·
  <a href="https://jwz-git.github.io/Daygo/"><b>Website</b></a> ·
  <a href="../docs/README.md">Design-Dokumentation</a>
</p>

<p align="center">
  <a href="../README.md">简体中文</a> · <a href="README.zh-Hant.md">繁體中文</a> · <a href="README.en.md">English</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <strong>Deutsch</strong> · <a href="README.fr.md">Français</a> · <a href="README.es.md">Español</a> · <a href="README.pt-BR.md">Português (Brasil)</a>
</p>

<p align="center"><strong>Kein manuelles Zeiterfassen. Arbeit automatisch festhalten und jeden Tag klar überblicken.</strong></p>

Daygo ist ein Local-First-Tool für Arbeitsprotokolle und Rückblicke unter **macOS** und **Windows**. Es erstellt im Hintergrund in festen Abständen einzelne Screenshots Ihres Hauptbildschirms und ordnet Ihre Aktivitäten mithilfe Ihrer selbst gewählten KI in **automatische Zeitachsen, tägliche Standup-Zusammenfassungen, Wochenrückblicke und Plan-Erinnerungen** ein — ohne Timer und ohne Cloud-Konto.

<p align="center"><b>Einzel-Screenshots · Lokale Speicherung · Eigene KI (inkl. lokaler Modelle) · 9 Sprachen</b></p>

<p align="center">
  <a href="#funktionen">Funktionen</a> ·
  <a href="#herunterladen-und-installieren">Herunterladen &amp; Installieren</a> ·
  <a href="#schnellstart">Schnellstart</a> ·
  <a href="#datenschutz-und-sicherheit">Datenschutz &amp; Sicherheit</a> ·
  <a href="#mitwirken">Mitwirken</a>
</p>

---

## Funktionen

<sub>Alle folgenden Screenshots zeigen anonymisierte Beispieldaten.</sub>

### Automatische Zeitachse

Daygo erfasst diskret den primären Bildschirm und fasst Aktivitätsblöcke per KI in Karten mit Zeitspanne, Titel, Zusammenfassung und Kategorie zusammen. Aufgeklappte Karten zeigen die Originalframes; neben Tages- und Wochenansicht unterstützt die Zeitachse Bearbeiten, Löschen, Neuverarbeiten einzelner Karten oder ganzer Tage, Löschen fehlgeschlagener Tages-Batches per Klick sowie heutige Planblöcke im Inspektor.

<img src="../docs/assets/readme/timeline.en.webp" width="100%" alt="Automatische Zeitachse: Aktivitäten nach Zeitabschnitten mit Detailinspektor" />

### Tagesrückblick

Sehen Sie Ihren Arbeitsablauf auf einen Blick und erstellen Sie Standup-Zusammenfassungen mit Highlights, erledigten Aufgaben und Blockern. Führen Sie ein Journal und Tagesziele, aktivieren Sie tägliche Journal-Erinnerungen und blenden Sie optional ein Token-Verbrauchsdiagramm am Ende des Tagesberichts ein (standardmäßig aus).

<img src="../docs/assets/readme/daily.en.webp" width="100%" alt="Tagesrückblick: Arbeitsablauf und Standup-Zusammenfassung" />

### Wochenrückblick

Analysieren Sie Ihre Woche mit Wochenverlauf, Fokus-/Ablenkungs-Heatmap, Kategorieanteilen, häufig genutzten Apps und Zeitfluss — ebenfalls mit optionalem Token-Diagramm (die erfasste Dauer schließt die Kategorie `System` aus).

<img src="../docs/assets/readme/weekly.en.webp" width="100%" alt="Wochenrückblick: Kategorien, häufig genutzte Apps und Zeitfluss" />

<details>
<summary>Dunkelmodus ansehen</summary>

<p><img src="../docs/assets/readme/dark.en.webp" width="100%" alt="Vorschau des Dunkelmodus von Daygo" /></p>

</details>

### Hintergrundbetrieb & Anpassung

| Funktion | Details |
|---|---|
| Hintergrundaufnahme | Das Schließen des Hauptfensters beendet die Aufnahme nicht (unter macOS wird der Dock-Indikator ausgeblendet); jederzeit über die macOS-Menüleiste oder den Windows-Infobereich wieder öffnen |
| Pause & Systemereignisse | Pause für 15 / 30 / 60 Minuten oder dauerhaft (zeitgesteuerte Pausen setzen automatisch fort); automatische Pause bei Ruhezustand, Sperrbildschirm oder Bildschirmschoner; manuell gestoppte Aufnahmen werden durch Systemereignisse nicht neu gestartet |
| Aufnahme & Speicher | Intervall 1 / 5 / 10 / 20 / 30 / 60 s (Standard 10 s); Höhe 720 / 1080 px (Standard 1080); konfigurierbares Speicherlimit mit Segment-Bereinigung; unter Windows lässt sich der Aufnahmeordner verschieben |
| Multi-Protokoll-KI & Fallback | API Key unterstützt OpenAI Chat Completions, OpenAI Responses, Anthropic Messages und Google Gemini (bevorzugt direkten Upload geeigneter abgeschlossener MP4-Segmente, sonst Fallback auf Screenshots); außerdem ChatGPT-Abo per OAuth, mehrere Modelle pro Anbieter und geordnete Fallback-Ketten |
| Pläne & Benachrichtigungen | Heutige Planblöcke mit Erledigt-Markierung; native macOS-/Windows-Benachrichtigungen für Journal-Erinnerungen, Planstart und Ablenkungshinweise |
| Design, Kategorien & Sprachen | Hell / Dunkel / System; Berufskategorie-Vorlagen beim Erststart mit frei anpassbaren Namen, Reihenfolge und Farben; Autostart beim Anmelden; 9 UI-Sprachen |

> **Logischer Tag ab 4:00 Uhr**: Zeitachse, Pläne, Journal und Ziele trennen Tage um **4:00 Uhr Ortszeit**, während Standup-Zusammenfassungen nach Kalendertagen gruppieren. Spätabendliche Arbeit kann daher unterschiedlich zugeordnet sein.

---

## Herunterladen und installieren

Laden Sie das passende Installationspaket aus dem [aktuellen GitHub Release](https://github.com/Jwz-git/Daygo/releases/latest) herunter:

| Plattform | System & Architektur | Paket |
|---|---|---|
| macOS | macOS 14+, Apple Silicon (`arm64`) | `Daygo-<Version>-arm64.dmg` |
| Windows | Windows 11 24H2+ (Build 26100+), x64 (`amd64`) | `Daygo-<Version>-amd64-installer.exe` |

macOS ist die Hauptentwicklungsplattform, für Windows gibt es ein x64-Installationsprogramm; für andere Architekturen und Linux gibt es noch keine Release-Pakete. Der Quelltext kann neuer als das letzte Release sein — siehe [Modulübersicht](../docs/09-roadmap.md#91-模块总表) und [Installation & Updates](../docs/modules/delivery.md).

---

## Schnellstart

1. **Installieren & Einrichten**: Starten Sie Daygo und wählen Sie im Onboarding eine Kategorievorlage und Grundeinstellungen. Unter macOS die Berechtigung **Bildschirmaufnahme** erteilen und bei Aufforderung neu starten.
2. **KI verbinden**: Wählen Sie im Onboarding oder in den Einstellungen **ChatGPT-Abo** (Browser-OAuth) oder **API Key** (Endpunkt, Schlüssel sowie manuell eingegebene oder abgerufene Mehrfach-Modelle). Prüfen Sie die Antwort anschließend im Bereich **Modelltest & Ausprobieren** mit Text oder einem Einzelbild.
3. **Aufnahme & Datenschutz einstellen**: Intervall, blockierte Apps und Speicherlimit anpassen (unter Windows optional Aufnahmeordner verschieben) und die Aufnahme starten.
4. **Rückblick nutzen**: Nach dem ersten Analyse-Batch Karten und Originalframes in der Zeitachse prüfen, bei Bedarf bearbeiten oder neu verarbeiten und Tages- sowie Wochenrückblicke ansehen.

> Daygo enthält kein eigenes KI-Kontingent. Die automatische Analyse setzt Bildverständnis und strukturierte Ausgaben im gewählten Protokoll voraus (auch bei lokalen Modellen); eine erfolgreiche Testantwort belegt noch nicht die vollständige Pipeline-Kompatibilität.

---

## Datenschutz und Sicherheit

- **Lokale Speicherung**: Screenshot-Segmente, Zeitachse, Journal, Pläne, Einstellungen und SQLite-Datenbank verbleiben auf Ihrem Gerät. Es gibt kein eigenes Backend, kein Konto und keine Cloud-Synchronisierung.
- **Sie bestimmen das Ziel**: Bildschirmdaten verlassen Ihr Gerät ausschließlich an einen ausdrücklich konfigurierten KI-Anbieter. Mit einem kompatiblen lokalen Modell bleibt alles auf dem Gerät; bei externen Anbietern gelten deren Datenschutzrichtlinien.
- **Doppelter App-Schutz**: Blockierte Apps werden aus Screenshots ausgeschlossen; befindet sich eine blockierte App im Vordergrund, speichert Daygo nur einen geschwärzten Platzhalter-Frame. Beide Schutzebenen greifen immer gemeinsam.
- **System-Schlüsselbund (Write-Only)**: API Keys und ChatGPT-OAuth-Daten liegen ausschließlich im Systemspeicher (macOS-Schlüsselbund / Windows-Anmeldeinformationsverwaltung). Oberfläche und Bindings können keine Klartext-Schlüssel lesen; sie landen weder in SQLite noch in `localStorage` oder Fehlermeldungen.

Die Erfassung nutzt **einzelne Screenshots** statt eines dauerhaften Aufnahme-Streams. Nutzungsanalyse und Absturzberichte sind standardmäßig deaktiviert (noch ohne Sende-Consumer). Bei einer Deinstallation bleiben Ihre Daten und Zugangsdaten erhalten. Details in [Datenschutz & Sicherheit](../docs/07-privacy-security.md).

---

## Mitwirken

Die laufende Entwicklung erfolgt im Branch `test` mit **Go 1.26.10 + Wails v3.1.0 + Vue 3**; `main` ist der stabile Branch (siehe [Wails v3 GA Migrationsprotokoll](../docs/decisions/wails-v3-migration.md)). Go übernimmt Geschäftslogik und SQLite-Schreibzugriffe, Plattformfunktionen sind über Schnittstellen entkoppelt, und Vue kommuniziert über generierte TypeScript-Bindings mit Go.

### Voraussetzungen

- In [go.mod](../go.mod) festgelegte Go-Version
- Node.js `20.19+` (20.x) oder `22.12+` mit npm
- **macOS**: Xcode Command Line Tools
- **Windows**: MinGW-w64 (inkl. binutils), Visual Studio 2022 C++-Tools, Windows SDK 26100 und WebView2 Runtime — siehe [Windows-Build-Abhängigkeiten](../scripts/README.md#windows-build-dependencies)
- **Linux**: Standardmäßig GTK4- und WebKitGTK-6.0-Entwicklungspakete (arm64-Host-Build verifiziert; Bildschirmaufnahme und Systemintegration noch ausstehend)

### Klonen, prüfen und starten

```bash
git clone --branch test https://github.com/Jwz-git/Daygo.git
cd Daygo
./scripts/gate.sh   # Bootstrap, Build, Go-/Frontend-Tests und Dokumentationsprüfung
```

| Plattform | Entwicklungs-App starten |
|---|---|
| macOS | `./scripts/dev.sh` |
| Windows | `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\dev.ps1` |
| Linux | `./scripts/dev-linux.sh` |

Weitere Build- und Paketierungsbefehle stehen in [scripts/README.md](../scripts/README.md), Architektur- und Entwicklungsregeln in [Design-Dokumentation](../docs/README.md) und [AGENTS.md](../AGENTS.md).

Bitte melden Sie Fehler über ein [Issue](https://github.com/Jwz-git/Daygo/issues) mit System, App-Version, Reproduktionsschritten und bereinigten Fehlermeldungen. **Laden Sie niemals echte Screenshots, Datenbanken oder API Keys hoch.**

---

## Geplante Schritte

- Entwicklung eines Plugin-Systems
- Bildschirmaufnahme und Systemintegration unter Linux

---

## Lizenz

Veröffentlicht unter der [MIT License](../LICENSE).

<sub>Inspiriert von <a href="https://github.com/JerryZLiu/Dayflow">Dayflow</a> (MIT, © 2025 Jerry Liu).</sub>
