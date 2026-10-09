<p align="center">
  <img src="../docs/assets/readme/banner.en.webp" width="100%" alt="Daygo — Weißt du am Ende des Tages noch, woran du gearbeitet hast?" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><img src="https://img.shields.io/github/v/release/Jwz-git/Daygo?style=flat-square&color=F3854B&label=version" alt="Neueste stabile Version" /></a>
  <img src="https://img.shields.io/badge/macOS-14%2B-333333?style=flat-square&logo=apple&logoColor=white" alt="macOS 14+" />
  <img src="https://img.shields.io/badge/Windows-11%2024H2%2B-0078D4?style=flat-square" alt="Windows 11 24H2+" />
  <a href="../LICENSE"><img src="https://img.shields.io/badge/license-MIT-6E7DF7?style=flat-square&color=6E7DF7" alt="MIT License" /></a>
  <img src="https://img.shields.io/badge/Go%20%C2%B7%20Wails%20%C2%B7%20Vue%203-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go · Wails · Vue 3" />
</p>

<p align="center">
  <a href="https://github.com/Jwz-git/Daygo/releases/latest"><b>Herunterladen</b></a> ·
  <a href="https://jwz-git.github.io/Daygo/"><b>Website</b></a> ·
  <a href="../docs/README.md">Designdokumentation</a>
</p>

<p align="center">
  <a href="../README.md">简体中文</a> · <a href="README.zh-Hant.md">繁體中文</a> · <a href="README.en.md">English</a> · <a href="README.ja.md">日本語</a> · <a href="README.ko.md">한국어</a> · <strong>Deutsch</strong> · <a href="README.fr.md">Français</a> · <a href="README.es.md">Español</a> · <a href="README.pt-BR.md">Português (Brasil)</a>
</p>

<p align="center"><strong>Halte deine Arbeit am Bildschirm fest. Blicke auf deinen Tag zurück.</strong></p>

Daygo ist ein Werkzeug zur Arbeitsaufzeichnung und zum Rückblick für macOS und Windows. Es nimmt im Hintergrund in regelmäßigen Abständen Screenshots auf. Dein gewählter KI-Anbieter ordnet die Bildschirmaktivität in eine **Zeitleiste, Tagesrückblicke und Wochenübersichten** ein — als Grundlage für Stand-ups, Rückblicke und das Erinnern an Arbeitsdetails.

**Einzelne Screenshots · Lokale Speicherung · Freie KI-Auswahl · Neun Oberflächensprachen**

[Herunterladen und installieren](#herunterladen-und-installieren) · [Schnellstart](#schnellstart) · [Funktionsvorschau](#funktionsvorschau) · [Datenschutz und Daten](#datenschutz-und-daten) · [Zur Entwicklung beitragen](#zur-entwicklung-beitragen)

## Herunterladen und installieren

Wähle unter [Assets der neuesten stabilen Version](https://github.com/Jwz-git/Daygo/releases/latest) das Installationspaket für deine Plattform:

| Plattform | System und Architektur | Installationspaket |
|---|---|---|
| macOS | macOS 14+, Apple Silicon (arm64) | `Daygo-<Version>-arm64.dmg` |
| Windows | Windows 11 24H2+ (Build 26100+), x64 (amd64) | `Daygo-<Version>-amd64-installer.exe` |

macOS ist die Hauptentwicklungsplattform; für Windows gibt es ein x64-Installationspaket. Für andere Architekturen und Linux sind noch keine Installationspakete veröffentlicht. Der Quellcode kann der veröffentlichten Version voraus sein. Den Funktionsstand zeigt die [Modulübersicht](../docs/09-roadmap.md#91-模块总表); Nachweise zu Signierung, Notarisierung und Installation stehen unter [Auslieferung und Updates](../docs/modules/delivery.md).

## Schnellstart

1. **Installieren und Berechtigung erteilen**: Starte Daygo. Erlaube unter macOS die Bildschirmaufnahme und starte die App neu, wenn sie dich dazu auffordert.
2. **KI konfigurieren**: Trage in den Einstellungen die Dienst-URL, den API-Schlüssel und das Modell ein. Speichere und öffne „Modelltest und Erprobung“, um eine Antwort mit Text oder einem Bild zu prüfen.
3. **Aufzeichnung einstellen**: Wähle Screenshot-Intervall, blockierte Apps und Speicherlimit. Aktiviere anschließend die Aufzeichnung.
4. **Ergebnisse ansehen**: Nach dem ersten Analysebatch kannst du Karten und Originalbilder in der Zeitleiste prüfen und bearbeiten. Auf den Tages- und Wochenseiten blickst du auf deine Aktivitäten zurück.

Daygo enthält keinen KI-Dienst und kein Nutzungsguthaben. Die automatische Analyse benötigt Bilderkennung und strukturierte Ausgaben für die konfigurierten Protokolle. Eine Antwort auf der Testseite bestätigt nicht, dass die gesamte Analyse funktioniert. Auch lokale Modelle benötigen diese Fähigkeiten.

## Funktionsvorschau

<sub>Die folgenden Screenshots verwenden anonyme Beispieldaten.</sub>

### Automatische Zeitleiste

Daygo erfasst das Hauptdisplay in regelmäßigen Abständen. Die KI erstellt Aktivitätskarten mit Zeit, Titel, Zusammenfassung und Kategorie. Öffne eine Karte, um die Originalbilder anzusehen, oder bearbeite, lösche und verarbeite Ergebnisse erneut.

<img src="../docs/assets/readme/timeline.en.webp" width="100%" alt="Zeitleiste: chronologische Aktivitätskarten mit Detailansicht" />

### Tagesrückblick

Sieh dir den Arbeitsverlauf eines Tages an und erstelle eine Stand-up-Zusammenfassung mit Höhepunkten, abgeschlossenen Aufgaben und Hindernissen. Ergänze deine eigene Sicht mit einem Tagebuch und Tageszielen.

<img src="../docs/assets/readme/daily.en.webp" width="100%" alt="Tagesrückblick: Arbeitsverlauf und Stand-up-Zusammenfassung" />

### Wochenrückblick

Blicke mit dem Wochenverlauf, der Heatmap für Fokus und Ablenkung, Kategorieanteilen, häufig genutzten Apps und Zeitflüssen auf die Woche zurück. Die erfasste Gesamtzeit schließt die Kategorie System aus.

<img src="../docs/assets/readme/weekly.en.webp" width="100%" alt="Wochenrückblick: häufig genutzte Apps je Kategorie und Zeitflussdiagramm" />

### Hintergrundaufzeichnung und Anpassung

| Funktion | Beschreibung |
|---|---|
| Hintergrundaufzeichnung | Die Aufzeichnung läuft nach dem Schließen des Fensters weiter. Öffne es über die macOS-Menüleiste oder den Windows-Infobereich erneut |
| Pause und Fortsetzung | Pausiere für 15 / 30 / 60 Minuten oder unbegrenzt. Zeitlich begrenzte Pausen enden automatisch |
| Systemereignisse | Bei Ruhezustand, Bildschirmsperre und Bildschirmschoner pausiert die Aufnahme. Manuell ausgeschaltete Aufzeichnung wird durch Systemereignisse nicht aktiviert |
| Screenshot-Einstellungen | Intervalle von 1 / 5 / 10 / 20 / 30 / 60 Sekunden, standardmäßig 10. Höhe 720 / 1080 Pixel, standardmäßig 1080 |
| KI-Dienste | OpenAI Chat Completions, OpenAI Responses und Anthropic Messages. Mehrere Modelle je Anbieter und eine geordnete Ausweichkette |
| Darstellung und Kategorien | Hell / Dunkel / System. Namen, Reihenfolge und Farben der Kategorien bearbeiten; Start bei Anmeldung und macOS-Dock-Symbol einstellen |

Die Oberfläche unterstützt vereinfachtes und traditionelles Chinesisch, Englisch, Japanisch, Koreanisch, Deutsch, Französisch, Spanisch und brasilianisches Portugiesisch.

<details>
<summary>Dunkelmodus ansehen</summary>

<p><img src="../docs/assets/readme/dark.en.webp" width="100%" alt="Daygo im Dunkelmodus" /></p>

</details>

Tage in Zeitleiste, Tagebuch und Zielen beginnen um **4 Uhr Ortszeit**. Stand-up-Zusammenfassungen verwenden Kalendertage. Aktivitäten in der Nacht können deshalb je nach Ansicht unterschiedlichen Tagen zugeordnet sein.

## Datenschutz und Daten

- **Lokale Speicherung**: Screenshots, Zeitleisten, Tagebuch, Einstellungen und Datenbank bleiben auf deinem Gerät. Daygo hat kein eigenes Backend, Konto oder Synchronisierungsangebot.
- **Du wählst das Ziel**: Bildschirmdaten verlassen das Gerät ausschließlich für einen ausdrücklich konfigurierten KI-Dienst. Ein kompatibles lokales Modell kann die Bildschirmanalyse auf deinem Gerät ausführen. Drittanbieter verarbeiten Daten nach ihren eigenen Datenschutzrichtlinien.
- **App-Blockierung und Schutz der Vordergrund-App**: Blockierte Apps werden aus Screenshots ausgeschlossen. Ist eine blockierte App im Vordergrund, speichert Daygo ein bereinigtes Platzhalterbild. Beide Schutzmaßnahmen bleiben aktiv.
- **Systemspeicher für Zugangsdaten**: API-Schlüssel liegen ausschließlich im macOS-Schlüsselbund oder Windows Credential Manager. Die Oberfläche kann sie schreiben, aber nicht lesen. Sie gelangen nicht in App-Datenbank, localStorage oder Fehlermeldungen.

Die Aufzeichnung verwendet einzelne Screenshots und keinen fortlaufenden Bildschirmaufnahmestrom. Nutzungsanalyse und Absturzberichte sind standardmäßig deaktiviert; die Übermittlung ist noch nicht implementiert.

Eine Deinstallation bewahrt Benutzerdaten und Zugangsdaten. Die vollständigen Datengrenzen beschreibt [Datenschutz und Sicherheit](../docs/07-privacy-security.md).

## Zur Entwicklung beitragen

Go übernimmt Produktlogik und Datenbankschreibzugriffe. Plattformfähigkeiten sind über Schnittstellen getrennt; Vue greift über generierte Wails-Bindings auf Go zu. `test` ist der Entwicklungsbranch, `main` der stabile Branch.

Für die Entwicklung unter macOS werden Go, Node.js/npm und Xcode Command Line Tools benötigt. Die Go-Version steht in [go.mod](../go.mod), die Node.js-Anforderungen im [Entwicklungsskript](../scripts/dev.sh).

```bash
git clone --branch test https://github.com/Jwz-git/Daygo.git
cd Daygo
./scripts/gate.sh   # Vorbereitung, Build, Go- und Frontend-Tests sowie Dokumentationsprüfung
```

Das Prüfsystem bereitet Frontend-Artefakte und Wails-Bindings in der erforderlichen Reihenfolge vor. Plattformbefehle stehen unter [Skripteinstiegspunkte](../scripts/README.md), Design und Beitragsregeln in der [Designdokumentation](../docs/README.md) und [AGENTS.md](../AGENTS.md).

Melde Probleme als [Issue](https://github.com/Jwz-git/Daygo/issues) mit System, App-Version, Reproduktionsschritten und bereinigten Fehlermeldungen. Lade keine echten Screenshots, Datenbanken oder API-Schlüssel hoch.

## Lizenz

Veröffentlicht unter der [MIT License](../LICENSE).

<sub>Inspiriert von <a href="https://github.com/JerryZLiu/Dayflow">Dayflow</a> (MIT, © 2025 Jerry Liu).</sub>
