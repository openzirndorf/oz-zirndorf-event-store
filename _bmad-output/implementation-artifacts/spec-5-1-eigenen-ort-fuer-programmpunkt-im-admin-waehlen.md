---
title: '5.1: Eigenen Ort für einen Programmpunkt im Admin wählen'
type: 'feature'
created: '2026-11-11'
status: 'done'
route: 'dispatch'
baseline_commit: 'f101d63ef4d3d56e3e47e46d180b60d061d9bae1'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-5-context.md'
  - '{project-root}/_bmad-output/planning-artifacts/sprint-change-proposal-2026-11-11.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Ein Programmpunkt gehört immer zum Ort seines Events. Ein Konzert des Weihnachtsmarkts in der Kirche lässt sich nicht an der Kirche zeigen.

**Approach:** Ein Programmpunkt verweist optional auf einen gepflegten Ort (`timetable_entries.location_id`, `NULL` = Ort des Events). Das Event-Formular bietet pro Programmpunkt eine Ortsauswahl, der Kern validiert und kanonisiert den Verweis, `DeleteLocation` schützt genutzte Orte. Öffentliche API (5.2) und Import (5.3) sind nicht Teil dieser Story.

## Boundaries & Constraints

**Always:** Test-first, 100 % Abdeckung in `core` und `admin`. Migration `00009` nur vorwärts (AD-17, nullable, keine Fachlogik in SQL). Ablaufplan nur als ganze Liste über `SaveEvent` (AD-15). Der Verweis auf den Ort des Events wird in `Canonicalize` entfernt. Meldungen deutsch, Code und Fehler englisch. sqlc-Code neu erzeugen und mitcommitten.

**Never:** Keine Änderung an `openapi.yaml`, Import-Schema, `publicapi/v1`, Duplikatprüfung, `effective*`, Sortierung und `RecomputeDerived`. Kein neuer Ort aus dem Programmpunkt heraus. Keine Prüfung räumlicher Nähe. Keine Änderung am Import-Lesen und -Klassifizieren (5.3), auch wenn `EventFingerprint` sich ändert.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Eigener Ort | Punkt mit `locationId` einer vorhandenen Ortskennung | gespeichert, beim Öffnen ausgewählt | – |
| Ort des Events | Punkt ohne `locationId` | kein Verweis gespeichert, „Ort des Events“ gewählt | – |
| Gleicher Ort wie Event | `locationId` = Ort des Events | `Canonicalize` entfernt ihn, wie „Ort des Events“ | – |
| Event zieht um | Punkt ohne eigenen Ort | Punkt gilt am neuen Ort des Events | – |
| Unbekannte Kennung | `locationId` ohne Ort | `ErrValidation`, `timetable[i].locationId` = `notFound`, deutsche Meldung am Punkt (422) | Formular bleibt mit Eingaben |
| Ort währenddessen gelöscht | FK-Verletzung beim Speichern | wie unbekannte Kennung, am betroffenen Punkt | – |
| Ort löschen | nur Programmpunkte verweisen | abgelehnt (409), Meldung nennt Programmpunkte | `LocationInUseError` |

</frozen-after-approval>

## Code Map

- `internal/adapter/postgres/migrations/00005_timetable_entries.sql`, `00008_*.sql` -- Muster für `00009_timetable_entry_location.sql` (nicht ändern).
- `internal/adapter/postgres/queries/events.sql` -- Timetable-Queries um `location_id` erweitern, neue Query `CountTimetableEntriesByLocation`; `db/` mit sqlc 1.31.1 neu erzeugen.
- `internal/adapter/postgres/events.go` -- `timetableEntryFromRow`, `withTimetable` (`CreateTimetableEntryParams`), `CountByLocation` als Muster; FK-Verletzung beim Eintrag wie beim Event auf `ErrConflict` abbilden.
- `internal/core/timetable.go` -- `TimetableEntry`/`TimetableEntryInput` um `LocationID`, `TimetableFieldLocationID = "locationId"`, `normalized()`, `timetableEntryFields`, `parseTimetableEntry` (nur Form; Existenz prüft der Service).
- `internal/core/event.go` -- `Canonicalize` (Verweis auf Event-Ort entfernen), `EventInputOf` (Ort mitgeben), `EventFingerprint`.
- `internal/core/event_service.go` -- `saveEvent`: Orte der Punkte per `LocationRepo.Get` prüfen, Probleme je Index; bei `ErrConflict` Orte neu prüfen; `EventRepo` um `CountTimetableEntriesByLocation`.
- `internal/core/location_service.go`, `errors.go` -- `deleteLocation` zählt Punkte, `LocationInUseError.TimetableEntryCount`.
- `internal/adapter/admin/timetable.go`, `templates/timetable_entry.html`, `events.go`, `locations.go` -- Formularfeld `timetable.locationId`, `timetableFromForm` (fünfte parallele Liste), Ortsauswahl je Punkt (`locationChoiceOf`/`selectOption` wiederverwenden, Leer-Option „Ort des Events“), Fragment-Handler braucht die Ortsliste, `timetableFieldMessages`, `locationInUseMessage`.
- Test-Fakes: `fakeEventRepo` (`core/event_service_test.go`), `memoryEventRepo` (`admin/events_test.go`), Postgres-Tests `timetable_test.go`, `delete_test.go`.

## Nahtstellen

- `SaveEvent`/`newEvent`/`Canonicalize` -- 1.7, 1.9 -- Eingabe-Normalisierung und Sortierung bleiben, Verweis auf Event-Ort fällt weg -- Kerntest je Fall der Matrix.
- Ablaufplan-Formular, Re-Render nach Validierungs-/Duplikatfehler -- 1.9, 1.10 -- gewählter Ort bleibt erhalten, Fehler am Punkt -- Admin-Test (422 und Duplikatwarnung).
- `timetableFromForm` und Fragment „Programmpunkt hinzufügen“ -- 1.9 -- vier Listen werden fünf; ein leerer Punkt (nur Ort „Ort des Events“) ist weiter leer -- `isBlankEntry`-Test.
- `DeleteLocation`, Löschrückfrage -- 1.11 -- Meldung bei Programmpunkten; Löschen von Event-Orten unverändert -- Kern- und Admin-Test.
- `RecomputeDerived`, Ablaufplan-Ersetzen im Import-Commit -- 2.5, 3.3 -- lesen/schreiben `TimetableEntry` mit; Ort bleibt erhalten -- Postgres-Rundlauf-Test.
- `EventFingerprint` -- 3.6 -- Ort geht ein, alte Importdateien ohne Ort bleiben `unchanged` -- Kerntest.

## Tasks & Acceptance

**Execution:**
- [x] `internal/adapter/postgres/migrations/00009_timetable_entry_location.sql` -- `location_id uuid NULL`, FK `ON DELETE RESTRICT`, Index -- Datenmodell
- [x] `internal/adapter/postgres/queries/events.sql` + `db/` -- Queries erweitern, Zählquery, neu erzeugen -- Speicherung
- [x] `internal/adapter/postgres/events.go` + Tests -- Lesen/Schreiben, Zählen, FK-Verletzung → `ErrConflict` -- Persistenz
- [x] `internal/core/timetable.go`, `event.go` + Tests -- Feld, Canonicalize, Input-Rückgabe, Fingerprint -- Kern
- [x] `internal/core/event_service.go`, `location_service.go`, `errors.go` + Tests -- Ortsprüfung, Löschschutz -- Anwendungsfälle
- [x] `internal/adapter/admin/*` + Templates + Tests -- Ortsauswahl, Meldungen, Löschmeldung -- Oberfläche

**Acceptance Criteria:**
- Given das Event-Formular, when ich einen Programmpunkt hinzufüge, then hat er eine Ortsauswahl mit „Ort des Events“ vorbelegt und allen Orten, und Hinzufügen/Entfernen laufen ohne Neuladen.
- Given ein gespeicherter Punkt mit eigenem Ort, when ich das Event öffne, then ist der Ort ausgewählt.
- Given ein Punkt ohne eigenen Ort, when der Ort des Events wechselt, then gilt der neue Ort für den Punkt.
- Given ein Ort, auf den nur Programmpunkte verweisen, when ich ihn lösche, then lehnt der Kern ab und die Meldung nennt Programmpunkte.
- Given `bash scripts/check-coverage.sh`, then bleibt die Abdeckung bei 100 %, und die CI-Neuerzeugung von sqlc und oapi-codegen ergibt keinen Diff.

## Implementation Notes

## Spec Change Log

## Review Triage Log

| Befund | Verdikt | Evidenz |
|---|---|---|
| Re-Import löscht Programmpunkt-Ort (Blind, Seam) | medium, defer | `changesOf` vergleicht den gespeicherten Ort; Import-Änderung vom Frozen-Block ausgeschlossen; Deferred-Eintrag, Ziel Story 5.3 (Andreas, 2026-11-11) |
| Formular ohne `locationId`-Listen → 400 (Edge) | low, rejected | Nur ein offenes Formular während des Deploys, Einzelnutzer; Fix wäre ein zusätzlicher Zweig |
| `locationInUseMessage` bei zwei Nullen (Edge) | false | Der Kern liefert `LocationInUseError` nur bei Zähler > 0 |
| Option bei unbekanntem Ort nicht gewählt (Edge, Blind) | low, rejected | Meldung am Punkt nennt den Ort; Fix wäre ein zusätzlicher Zweig |
| Rückfall auf `locationId notFound` bei fremdem Konflikt, Neuprüfung außerhalb der Transaktion (Edge, Blind) | low, rejected | Verhalten war vorher dasselbe; Race bewusst hingenommen |
| Fehlerhafte UUID in `locations.Get` (Edge) | low, rejected | Gleicher Pfad wie beim Event-Ort, vorbestehend |
| Entfernen nur bei gesetzter Event-Ort-Kennung (Edge) | false | Gewollt; Import-Ort ist Thema von 5.3 |
| Fingerprint ändert sich beim Deploy (Edge) | false | Fingerprints werden nicht gespeichert, nur zwischen Vorschau und Commit verglichen |
| FK-Verletzung → `ErrConflict` am Punkt (Edge) | low, rejected | Folgt der Regel beim Event |
| Testname/Kommentar `TestEntryLocationDeletedWhileSaving…`, `goneLocationRepo` (Verification-Gap) | low, patch | Redundanten Test entfernt, Kommentar korrigiert |
| Kommentare in `timetable.go` verstümmelt (Verification-Gap, Blind) | low, patch | Umbruch und Satz korrigiert |
| Vergleich ohne `LocationID` → Reihenfolge hängt von Eingabe ab (Seam) | low, patch | Vergleich um `LocationID` ergänzt, Test `TestCanonicalizeOrdersEntriesThatDifferOnlyByOwnLocation…` |
| Duplikatwarnung ohne Test für gewählten Ort (Seam) | low, patch | `TestDuplicateWarningKeepsTheOwnLocationOfAnEntrySelected` |
| `writeRefusedError` im Import-Commit (Seam) | low, rejected | Passt weiter auf `ErrConflict`, Commit wickelt den Fehler ein |
| `RecomputeDerived` gegen Postgres ohne Test (Seam) | low, rejected | `UpdateDerived` und `Update` sind mit Postgres getestet |
| Zeitzeile vor/hinter Ortsauswahl, Label „Ort“, Optionslisten je Punkt, Test-Fragilität, `msgManyTimetableEntry`, Down-Migration (Blind) | low/false, rejected | Geschmack ohne benanntes Problem; Migrationen sind nur vorwärts (AD-17) |
| Sprint-Status/Spec-Status abweichend (Blind) | false | Wird am Ende synchronisiert |

## Design Notes

- `CountTimetableEntriesByLocation` ist ein eigener Repo-Port neben `CountByLocation`: `LocationInUseError` trägt beide Zahlen, die Meldung nennt Events und Programmpunkte getrennt. Der FK `RESTRICT` bleibt als letzte Sperre (`ErrConflict`).
- Der Feldname `locationId` am Programmpunkt ist nur intern (Admin-Formular). Die öffentliche Spec nennt in 5.2 `location` und führt keine Ortskennung (FR-10).

## Verification

**Commands:**
- `go test ./...` -- expected: grün (mit `CI` leer, ohne Datenbank)
- `go test -p 1 ./internal/adapter/postgres/... ./cmd/eventstore/...` mit `EVENTSTORE_TEST_DATABASE_URL` -- expected: grün
- `bash scripts/check-coverage.sh` -- expected: 100 % in den drei Paketen
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- expected: keine Funde
