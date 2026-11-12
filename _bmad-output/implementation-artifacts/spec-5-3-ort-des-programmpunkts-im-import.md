---
title: '5.3: Ort des Programmpunkts im Import'
type: 'feature'
created: '2026-11-12'
status: 'done'
route: 'dispatch'
baseline_commit: '6942cd5f35afce6a14837ef134dd961189ad5c97'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-5-context.md'
  - '{project-root}/_bmad-output/planning-artifacts/sprint-change-proposal-2026-11-11.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Die Import-Datei kann für einen Programmpunkt keinen eigenen Ort mitgeben. Recherchierte Programme mit Konzert in der Kirche brauchen Nacharbeit im Admin, und ein erneuter Import verliert einen im Admin gesetzten Ort ohne lesbare Änderungsanzeige (Deferred aus 5.1).

**Approach:** `timetable[i].location` (Schema `TimetableEntryInput` aus 5.2) wird wie der Ort des Events aufgelöst: vorhandener Name → dieser Ort, neuer Name → `newLocation` einmal pro Name und Datei, mehrdeutig oder unvollständig → `error` am Programmpunkt. `location: null` heißt ausdrücklich Ort des Events. Vergleich, Änderungsanzeige und Commit berücksichtigen den Ort des Programmpunkts.

**Entscheidungen (Andreas, 2026-11-12):**
- Fehlt `location` am Programmpunkt, und das Event hat per `importKey` ein Ziel im Bestand, übernimmt der Punkt den gespeicherten eigenen Ort des passenden Bestands-Punkts. Passend heißt: gleiche Beschreibung, gleiches Datum, gleiche Zeiten nach Normalisierung; mehrere gleiche Punkte werden in Reihenfolge paarweise zugeordnet. Ohne passenden Punkt oder ohne Ziel gilt der Ort des Events. So bleibt eine alte Datei `unchanged`, auch wenn im Admin ein Ort gesetzt wurde (Deferred aus 5.1).
- `location: null` unterscheidet sich vom fehlenden Feld: Es entfernt einen gespeicherten Ort (`update`). Damit weicht 5.3 bewusst vom Satz „ohne `location` oder mit `location: null` gilt der Ort des Events“ in `epics.md` ab.

## Boundaries & Constraints

**Always:** Test-first, 100 % Abdeckung in `core` und `admin`. Schreiben nur über `CommitImport` → `SaveEvent` (AD-6), Ablaufplan als ganze Liste (AD-15). `formatVersion` bleibt `1`. Ein neuer Ort entsteht im Commit einmal pro Name, egal ob Event oder Programmpunkte ihn mitbringen. Meldungen deutsch, Code englisch.

**Never:** Keine Änderung an Datenbank, Migrationen, öffentlicher Leseform, `import-v1.schema.json`, Admin-Event-Formular, Duplikatprüfung, `effective*`. Keine Ortskennung in der Import-Datei (AD-14).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Vorhandener Ort | `location.name` eines Orts | Punkt diesem Ort zugeordnet | – |
| Neuer Ort | neuer Name, vollständig | `newLocation`, einmal pro Name und Datei, Positionen aller Events | – |
| Unvollständig | neuer Name ohne Adresse/Koordinaten | Eintrag `error`, Meldung „Programmpunkt N, Ort …“ | Feldpfad `timetable[i].location.*` |
| Mehrdeutig | Name mehrerer Orte | `error` am Programmpunkt | wie C1 |
| Ohne Ort, neu | `location` fehlt oder `null`, kein Ziel oder kein passender Bestands-Punkt | Ort des Events | – |
| Fehlt, Bestand mit Ort | `location` fehlt, passender Bestands-Punkt hat eigenen Ort | Ort bleibt, `unchanged` | – |
| `null`, Bestand mit Ort | `location: null`, passender Bestands-Punkt hat eigenen Ort | `update`, Anzeige „Kirche → Ort des Events“, Commit entfernt ihn | – |
| Anderer Ort | `importKey` vorhanden, Punkt mit anderem Ort | `update`, Anzeige nennt `timetable` mit Ortsnamen | – |
| Gleicher Ort | Ort wie im Bestand | `unchanged` | – |
| Ort = Event-Ort | Punkt bringt den Ort des Events mit | wie ohne Ort | – |
| Alte Datei | ohne `location`, Bestand ohne eigene Orte | `unchanged` | – |

</frozen-after-approval>

## Code Map

- `internal/core/timetable.go` -- `TimetableEntryInput.Location *LocationInput` (Import, analog `EventInput.Location`), in `normalized()` aufnehmen; `TimetableFieldLocation` gibt es aus 5.2.
- `internal/core/import.go` -- `readTimetable` (403) liest `location` per `nestedObject` und unterscheidet fehlend von `null` (z. B. Merker am `TimetableEntryInput`, nur Import); `readLocationInput` (373–388), `readCoordinate` (392) und `importLocationField` (453) mit Pfad-Präfix verallgemeinern (`location` bzw. `timetable[i].location`). `resolveImportLocation` (424) mit Präfix und „Pflicht ja/nein“: für Punkte heißt `nil` kein eigener Ort. `readImportEntry` (317) löst die Punkt-Orte auf, `importedEvent` (297) trägt sie je Index.
- `internal/core/import_classify.go` -- `classifyAgainstStore` (196) setzt Punkt-`LocationID` für vorhandene Orte und übernimmt bei fehlendem `location` die `LocationID` des passenden Punkts des Ziels (eine Funktion, vor `changesOf`; das Ergebnis steht im `entry.Input`, so nutzt der Commit dieselbe Kennung, `isStale` schützt vor Zwischenänderungen); für neue Orte nur in der Vorschau einen Schlüssel aus `NameKey`, der nie eine Ortskennung ist, damit `Canonicalize` (event.go 203) „neu und gleich dem neuen Event-Ort“ entfernt und `changesOf` (246) ihn vergleicht. `timetableText` (276) nennt den Ortsnamen. `collectNewLocations` (371) und `newLocationDiffers` (397) nehmen Punkt-Orte auf; `entry.NewLocation` gilt auch bei neuem Punkt-Ort.
- `internal/core/import_commit.go` -- `write` (342): nach Z. 349 `in.Timetable[i].LocationID` aus vorhandenem oder übernommenem Ort oder über `locationID` (364) mit gemeinsamer `createdLocations`; `firstLocations` (324) um Punkt-Orte. `isStale` (267) bleibt bei `NewLocation` und Fingerprint.
- `internal/adapter/admin/import.go` -- `importProblemMessage` (808) und `timetableFieldLabels` (223): Pfade `timetable[i].location.*` mit Label „Programmpunkt N, Ort: …“ und den Meldungen von Ort (`locationFieldMessages`, locations.go 115) und Mehrdeutigkeit (`msgImportAmbiguousLocation`, 92).
- `api/v1/openapi.yaml` -- Beschreibung von `TimetableEntryInput.location` (aus 5.2): Auflösung wie beim Event; `null` = Ort des Events; fehlt = bei erneutem Import eines Events mit `importKey` bleibt der Ort des gleichen Programmpunkts, sonst Ort des Events.
- Tests als Muster: `import_test.go` 90, 566; `import_classify_test.go` 100, 149, 309, 424; `import_commit_test.go` 114, 379, 361; admin `import_test.go` 157, 188, 215.

## Nahtstellen

- `readLocationInput`/`resolveImportLocation` -- 3.1, 3.2, C1 -- Ort des Events unverändert (Pflicht, gleiche Pfade und Meldungen) -- bestehende Tests grün.
- `changesOf`/`timetableText` -- 3.2 -- andere Felder unverändert; Deferred-Eintrag 5.1 (Re-Import ohne Ort) -- Klassifizierungstest je Matrixzeile.
- `collectNewLocations` -- 3.2 -- ein Eintrag pro Name über Events und Punkte -- Kerntest.
- `write`/`locationID`/`isStale` -- 3.3, 3.6 -- Ort einmal erstellt, `CreatedLocations` zählt mit, Stale-Erkennung unverändert -- Commit-Tests.
- `importProblemMessage` -- 3.1, 1.9 -- Meldungen am Programmpunkt -- Admin-Test.

## Tasks & Acceptance

**Execution:**
- [x] `internal/core/timetable.go`, `import.go` + Tests -- Lesen, Auflösen, Fehlerpfade -- Vorschau
- [x] `internal/core/import_classify.go` + Tests -- Vergleich, Änderungsanzeige, neue Orte -- Klassifizierung
- [x] `internal/core/import_commit.go` + Tests -- Kennungen setzen, Orte einmal erstellen -- Commit
- [x] `internal/core/import_classify.go` + Tests -- Übernahme des gespeicherten Orts bei fehlendem `location`, `null` entfernt ihn, Paarung gleicher Punkte -- Entscheidung B
- [x] `internal/adapter/admin/import.go` + Tests -- Labels, Meldungen -- Oberfläche
- [x] `api/v1/openapi.yaml` -- Beschreibung, `go generate ./...` -- Vertrag
- [x] `_bmad-output/implementation-artifacts/deferred-work.md` -- Eintrag 5.1 → `done` mit Verweis -- Pflege

**Acceptance Criteria:**
- Given eine Datei mit einem Weihnachtsmarkt-Punkt in einer neuen Kirche und einem zweiten Event mit derselben Kirche, when ich importiere, then entsteht die Kirche einmal und beide Punkte verweisen darauf.
- Given `bash scripts/check-coverage.sh`, then 100 %, und `go generate ./...` ergibt keinen Diff.

## Implementation Notes

## Spec Change Log

## Review Triage Log

| Befund | Verdikt | Evidenz |
|---|---|---|
| Fehler von `parseTimetable` in `withLocationKeys` verworfen (Blind) | low, patch | Regel „jeden Fehler behandeln oder weitergeben“; Fehler wird weitergegeben |
| Commit ohne Test: gleicher neuer Ort an Event und Programmpunkt; Punkt am neuen Event-Ort (Seam) | low, patch | Nur Vorschau getestet; zwei Commit-Tests ergänzt |
| `classifyAgainstStore` schreibt Klassifizierungsergebnis in `entry.Input`, Commit hängt daran (Blind) | low, patch | Kommentar ergänzt |
| Überschreiben eines Duplikatverdachts löscht Punkt-Orte ohne Anzeige (Blind, Edge, VG) | medium, defer | Entscheidung B nennt nur das Ziel per `importKey`; Überschreiben ersetzt seit 3.3 alle Felder ohne Anzeige; Deferred-Eintrag, Ziel Retro Epic 5 |
| Zwei neue Orte, einer währenddessen angelegt, nicht `stale` (Edge) | low, rejected | Commit scheitert laut als Konflikt (`TestCommitImportReportsANewLocationNameTakenOnWriteAsConflict`); Fix bräuchte Schlüsselmenge je Eintrag |
| Label ohne Rückfall bei unbekanntem Unterfeld von `timetable[i].location` (Blind, Edge) | false | Der Kern meldet nur bekannte Ortsfelder; unbekannte Felder werden ignoriert |
| Deutscher Text „Ort des Events“ im Kern (Blind) | low, rejected | Kern führt bereits deutsche Inhalte (Event-Typ-Labels, `event.go:49`); Matrix verlangt die Anzeige |
| `nameOf` zeigt „Ort des Events“ bei unbekannter Kennung (Blind) | false | Alle Orte sind geladen, FK `RESTRICT` sichert Punkt-Orte |
| Import-Felder am gemeinsamen `TimetableEntryInput`, Zeigervergleich (Blind) | low, rejected | Muster wie `EventInput.Location` laut Code Map; `EventInputOf` setzt die Felder nie |
| Parallele Listen statt Struct, Parameterreihenfolge `readCoordinate` (Blind) | low, rejected | Geschmack, keine benannte Folge |
| Admin-Tests für Hinweise, mehrzeilige Änderung, Neuer-Ort-Marker (Blind) | low, rejected | Labels und Meldungen getestet; Vorlagen generisch und unverändert |
| Test „ALTE VESTE“ prüft den Hinweis nicht (Blind) | low, rejected | Hinweis-Pfad durch `TestPreviewImportListsANewTimetableLocationOnce…` abgedeckt |
| Abgleichregel und README nicht genauer dokumentiert (Blind) | low, rejected | Regel steht in `TimetableEntryInput.location`; README beschreibt die Leseform |
| Tracking-Artefakte, Spec untracked (Blind) | false | Wird am Ende synchronisiert und mitcommittet |
| Fingerprint ändert sich nach dem Deploy (VG) | false | Fingerprints werden nicht gespeichert; wie in 5.1 bewertet |

## Design Notes

- Der Vorschau-Schlüssel für neue Orte macht `Canonicalize`, `changesOf` und `EventFingerprint` ohne Sonderzweig nutzbar; er wird im Commit immer durch die echte Kennung ersetzt und erreicht `SaveEvent` nie.

## Verification

**Commands:**
- `go test ./...` (mit `CI` leer) -- expected: grün
- `go test -p 1 ./internal/adapter/postgres/... ./cmd/eventstore/...` mit `EVENTSTORE_TEST_DATABASE_URL` -- expected: grün
- `bash scripts/check-coverage.sh` -- expected: 100 %
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- expected: keine Funde
