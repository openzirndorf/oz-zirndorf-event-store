---
title: '5.2: Ort des Programmpunkts in der öffentlichen API'
type: 'feature'
created: '2026-11-12'
status: 'done'
route: 'dispatch'
baseline_commit: '0fa6c45ecdf4d30988b30d3f69c677c7e3507980'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-5-context.md'
  - '{project-root}/_bmad-output/planning-artifacts/sprint-change-proposal-2026-11-11.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Seit 5.1 kann ein Programmpunkt einen eigenen Ort haben, aber `/v1/events` und `/v1/archive/events` liefern ihn nicht. Die Karten-App zeigt das Konzert in der Kirche deshalb weiter am Marktplatz.

**Approach:** Jeder Programmpunkt der Leseform bekommt `location`: den vollständigen Ort wie beim Event (`EventLocation`) oder `null` für „findet am Ort des Events statt“. Der Kern ergänzt die Orte der Programmpunkte beim Listen, der Adapter bildet sie ab. Die Spec trennt `TimetableEntry` (Lesen) und das neue `TimetableEntryInput` (Schreiben, `location` optional als `EventInputLocation`).

## Boundaries & Constraints

**Always:** Test-first, 100 % Abdeckung in `core` und `publicapi/v1`. Feldnamen nur in `openapi.yaml` (AD-9), `api.gen.go` per `go generate ./...`. Rückwärtskompatibel (NFR-2): nur ein Feld kommt hinzu, keins fällt weg oder ändert den Typ. Keine Ortskennung in der Antwort (FR-10, AD-14). Code und Spec englisch.

**Never:** Keine Änderung an Datenbank, Migrationen, sqlc-Queries, Admin, Import-Lesen und -Klassifizieren (5.3), Duplikatprüfung, `effective*`, Sortierung, Cache-Headern. `import-v1.schema.json` bleibt unverändert, weil es `EventInput` per `$ref` einbindet. Die Pflichtfelder der Schreibform bleiben wie heute (`description`, `date`, `startTime`, `endTime`).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Eigener Ort | Punkt mit `LocationID` eines Orts | `location` mit `name`, `address`, `latitude`, `longitude`, `precision`, `note` | – |
| Ort des Events | Punkt ohne `LocationID` | `location: null` | – |
| Archiv | dasselbe Event in `/v1/archive/events` | dieselbe Form | – |
| Ort fehlt im Bestand | `LocationID` ohne Ort (Inkonsistenz) | Liste scheitert wie beim fehlenden Event-Ort | `ErrNotFound` → 500 |
| Altes Event | Ablaufplan ohne eigene Orte | jeder Punkt `location: null`, sonst unverändert | – |

</frozen-after-approval>

## Code Map

- `api/v1/openapi.yaml` -- `TimetableEntry` (Z. 615): `location` als Pflichtfeld `anyOf: [$ref EventLocation, type: 'null']` (Muster `Event.endPrecision`), Beschreibung mit „`null` … takes place at the location of the event“, Beispiel; `maxLength` von `description` wandert ins neue `TimetableEntryInput` (Leseform ohne Grenzen wie `Event`). `TimetableEntryInput`: gleiche Felder, `location` optional `anyOf: [$ref EventInputLocation, type: 'null']`. `EventInput.timetable.items` → `TimetableEntryInput`. Beispiele in `/events` (Z. 259) und `Event.timetable` um `location` ergänzen, einmal mit eigenem Ort (z. B. Posaunenchor in einer Kirche).
- `internal/adapter/publicapi/v1/api.gen.go` -- neu erzeugen, nie von Hand.
- `internal/core/event_query.go` -- `ListedEvent` um die Orte der Programmpunkte ergänzen; `listMatchingEvents` füllt sie aus der vorhandenen `locations`-Map (`locationsByID`), fehlender Ort → `ErrNotFound` wie Z. 174–177; `listedEventOf` bekommt sie mit.
- `internal/core/timetable.go` -- Konstante `TimetableFieldLocation = "location"` (AD-9-Spiegel, 5.3 liest damit).
- `internal/adapter/publicapi/v1/events.go` -- `timetableOf` braucht das `ListedEvent`; `eventLocationOf` für den Punkt wiederverwenden, `nil` → `null`.
- `internal/adapter/publicapi/v1/contract_test.go` -- `fieldSchemas[timetableFieldPrefix]` → `TimetableEntryInput`; `specLimits`-Pfad auf `TimetableEntryInput`; neuer Test: Lese- und Schreibform des Programmpunkts haben dieselben Feldnamen. `TestEventInputHasNoIdentifiers` deckt `timetable.location.*` über `fieldPaths` mit ab.
- `internal/adapter/publicapi/v1/events_test.go` -- `fullListedEvent` mit einem Punkt mit eigenem Ort, Erwartungen in `TestListEventsDeliversExactlyTheReadForm`, `…FormatsDatesTimesAndInstants`, `…GivesOutNoInternalValues` (Ortskennung des Punkts).
- `internal/core/event_query_test.go` -- Muster Z. 260 und Z. 631 für Kerntests.
- `README.md` -- Beispiel Z. 101: `"location": null` je Punkt, Satz zur Leseform.

## Nahtstellen

- `listMatchingEvents`/`listedEventOf` -- 2.3, 2.7 -- Filter, Ausblenden markierter Events, Sortierung bleiben; neu werden Punkt-Orte ergänzt -- Kerntests aktiv und Archiv.
- `eventOf`/`timetableOf` -- 2.3, 2.8 -- Leseform ohne `archived`, leere Liste nie `null`; neu `location` -- `TestListEventsDeliversExactlyTheReadForm`, `…EmptyValuesAsNull`.
- Spec-Beispiele und Doku -- 2.6 -- Beispiele passen zum Schema -- Contract-Tests, README.
- `EventInput` im Import-Schema -- 3.1 -- `TimetableEntryInput` wird per `$ref` mitgezogen, das Import-Lesen ignoriert `location` bis 5.3 (unbekannte Felder werden ignoriert) -- `TestImportSchemaIsValidJSONAndRefersOnlyToSchemasOfTheSpec`.
- Cache-Header -- 4.3 -- unverändert -- bestehende `cache_test.go`.

## Tasks & Acceptance

**Execution:**
- [x] `internal/adapter/publicapi/v1/contract_test.go` -- Tests zuerst: Schreibform `TimetableEntryInput`, gleiche Felder in beiden Formen, Grenzen -- AD-9
- [x] `api/v1/openapi.yaml` + `internal/core/timetable.go` -- Schemas, Beispiele, Konstante; `go generate ./...` -- Vertrag
- [x] `internal/core/event_query.go` + Test -- Orte der Punkte ergänzen, fehlender Ort → `ErrNotFound` -- Kern
- [x] `internal/adapter/publicapi/v1/events.go` + `events_test.go` -- Abbildung, Matrix-Fälle aktiv und Archiv -- Adapter
- [x] `README.md` -- Beispiel und Hinweis -- Doku

**Acceptance Criteria:**
- Given ein Event mit einem Punkt mit eigenem Ort und einem ohne, when ich `/v1/events` oder `/v1/archive/events` abrufe, then hat der erste `location` vollständig wie der Ort des Events und der zweite `location: null`.
- Given die Spec, when ich `TimetableEntry` lese, then ist `location` als `EventLocation` oder `null` beschrieben, mit Satz zur Bedeutung von `null` und Beispiel, und `EventInput.timetable` verweist auf `TimetableEntryInput` mit optionalem `location`.
- Given die Antwort, then enthält sie keine Ortskennung eines Programmpunkts.
- Given `bash scripts/check-coverage.sh`, then 100 % in den Pflichtpaketen, und `go generate ./...` ergibt keinen Diff.

## Implementation Notes

## Spec Change Log

## Review Triage Log

| Befund | Verdikt | Evidenz |
|---|---|---|
| Akzeptanztest `TestChristmasMarketIsFoundByPeriod` erwartet Punkte ohne `location` (Seam) | high, patch | `wantFirstAdventWeekend` vergleicht die ganze Antwort; `"location": null` ergänzt |
| Vertragstest prüft die Schemas von `location` nicht (Blind) | low, patch | Test vergleicht nur Namen; Typprüfung `EventLocation`/`EventInputLocation` ergänzt |
| Beispiel `Event.timetable` hat Christkind-Eintrag ersetzt statt ergänzt (Blind) | low, patch | Eintrag wiederhergestellt, Posaunenchor angehängt |
| Fehlender Punkt-Ort eines markierten Events lässt die Liste scheitern (Edge, Blind) | false | Gleiche Reihenfolge wie beim Event-Ort, gewollt und getestet (`TestPublicListsReportTheMissingLocationOfAMarkedEvent`) |
| Punkt-`LocationID` gleich Event-Ort liefert vollen Ort statt `null` (Edge, Blind) | false | Jedes Schreiben läuft über `SaveEvent`/`Canonicalize`, das den Verweis entfernt; kein anderer Schreibweg |
| Import ignoriert dokumentiertes `location` am Punkt (Edge, Blind) | low, rejected | Bewusst laut Design Notes; 5.3 folgt im selben Lauf |
| Kein Test, dass der Import `location` am Punkt ignoriert (Seam) | low, rejected | 5.3 ersetzt das Verhalten samt Tests |
| Spec-Beispiele nicht gegen Schemas geprüft (Seam) | low, rejected | Vorbestehend, keine Beispielprüfung im Projekt |
| `maxLength` aus der Leseform entfernt (Blind) | low, rejected | Gewollt (Leseform ohne Grenzen wie `Event`); lockert keine Antwort |
| README zeigt nur `location: null` (Blind) | low, rejected | Beispiel stammt laut README aus Produktion; Satz erklärt den eigenen Ort |
| Leere Map je Event, exportierte Map (Blind) | low, rejected | Kein benannter Schaden; Zugriff über `EntryLocation` dokumentiert |
| Test beider Listen in `archive_test.go` (Blind) | low, rejected | Geschmack, keine benannte Folge |
| Sprint-Status und Spec-Status abweichend (Blind) | false | Wird am Ende synchronisiert |
| Spec-Datei fehlt im Diff (Blind) | false | Absichtlich aus dem Review-Diff genommen, wird mitcommittet; 5.3-Spec bleibt draußen |

## Design Notes

- `ListedEvent` erhält `TimetableLocations map[string]Location` (eigene Orte der Punkte nach ID) und `EntryLocation(entry) (Location, bool)`; `false` heißt Ort des Events. Eine parallele Liste zum sortierten Ablaufplan wäre fehleranfällig, ein eigener Punkt-Typ würde `Event.Timetable` verdecken.
- `TimetableEntryInput.location` steht ab 5.2 in der Spec, wirkt im Import aber erst mit 5.3. Bis dahin ignoriert der Import das Feld wie jedes unbekannte; 5.3 folgt direkt.

## Verification

**Commands:**
- `go generate ./...` und `git status` -- expected: nur gewollte Änderungen an `api.gen.go`
- `go test ./...` (mit `CI` leer) -- expected: grün
- `bash scripts/check-coverage.sh` -- expected: 100 % in den drei Paketen
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- expected: keine Funde
