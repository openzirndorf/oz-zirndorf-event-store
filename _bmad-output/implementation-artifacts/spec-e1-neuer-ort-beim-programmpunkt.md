---
title: 'E1: Neuen Ort direkt beim Programmpunkt anlegen'
type: 'feature'
created: '2026-11-12'
status: 'done'
baseline_commit: 'c8f9678fb885d1b963faf54cbd92ff42ade302a8'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/spec-1-8-neuer-ort-beim-anlegen-eines-events.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-5-1-eigenen-ort-fuer-programmpunkt-im-admin-waehlen.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Seit 5.1 hat jeder Programmpunkt eine Ortsauswahl, aber „Neuer Ort“ gibt es nur am Ort des Events (1.8). Fehlt der Ort eines Programmpunkts, muss der Admin das Formular verlassen.

**Approach:** Jeder Programmpunkt bekommt unter seiner Ortsauswahl dieselbe Inline-Eingabe „Neuer Ort“ wie das Event (gleiche Felder, Karte, Meldungen, `SaveLocation`). Nach dem Speichern ist der neue Ort in der Auswahl dieses Programmpunkts gewählt; alle anderen Eingaben bleiben im Browser unberührt.

**Entscheidung (Andreas, 2026-11-12):** Nach dem Anlegen eines Orts, am Event wie am Programmpunkt, bekommen alle anderen Ortsauswahlen der Seite den neuen Ort als Option, ohne ihre Wahl zu ändern. Bis zum Neuladen steht er dort am Listenende.

## Boundaries & Constraints

**Always:** Wiederverwendung von `createInlineLocation`, `locationFields`, `newLocationClosed`/`newLocationOpen`, `locationFieldMessages`, `withDecimalPoints`. Die Inline-Felder gehören nie zum Event-Formular (Attribut `form` auf ein eigenes Formular außerhalb, kein verschachteltes `<form>`). Gültiges HTML, eindeutige IDs je Programmpunkt. „Neuer Ort“ am Event behält sein Markup, seine IDs und seine Antwort aus 1.8, ergänzt nur um die neue Option für die übrigen Auswahlen; bestehende Tests aus 1.8 bleiben grün. Ohne JavaScript führt „Neuer Ort“ auf `/admin/locations/new`. Test-first, 100 % Abdeckung für `internal/adapter/admin`. Texte deutsch, Code englisch.

**Never:** Keine Änderung an Kern, Postgres, Migrationen, `openapi.yaml`, Import. Kein Neuladen oder Senden des Event-Formulars, keine Event-Werte in den Ort-Anfragen. Kein neues Skript, kein CDN, keine Ortsbearbeitung inline.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Öffnen | „Neuer Ort“ an Programmpunkt K | Ortsfelder mit Karte im Programmpunkt K, andere Punkte und Event-Ort unverändert | – |
| Gültig | vollständige Ortseingabe | 200: Eingabe geschlossen, Hinweis „Ort … angelegt und ausgewählt.“, Auswahl von K neu mit neuem Ort gewählt | – |
| Ungültig | PLZ `9051` | 422: Felder mit Werten und Meldungen aus 1.4 in K | je Feld deutsche Meldung |
| Name existiert | Name eines vorhandenen Orts | 409: Meldung mit Link (neuer Tab) in K | – |
| Andere Auswahlen | Ort an K oder am Event angelegt | Event-Auswahl und alle übrigen Punkte haben die neue Option, ihre Wahl bleibt | – |
| Abbrechen | „Abbrechen“ in K | geschlossen, nichts gespeichert | – |
| Zwei offen | Eingaben an Event und K gleichzeitig offen | jede speichert nur ihre eigenen Felder | – |
| Kennung manipuliert | Programmpunkt-Kennung im Request ungültig | 400 | nichts gespeichert |
| Fehler sonst | `SaveLocation`/`ListLocations` scheitert | 500, geloggt, nichts eingetauscht | wie 1.8 |

</frozen-after-approval>

## Code Map

- `internal/adapter/admin/new_location.go` -- `createInlineLocation`, `openInlineLocation`, `cancelInlineLocation`, `renderSavedLocation`, `openNewLocationArea`, `newLocationArea`/`newLocationSaved`; um Ziel (Event oder Programmpunkt-Kennung) erweitern. Event-Konstanten `newLocationAreaID`, `locationChoiceID`, `inlineLocationFormID`, `inlineLocationIDPrefix` bleiben.
- `internal/adapter/admin/templates/new_location.html` -- `newLocationClosed`/`newLocationOpen` mit Bereichs-ID, Ziel-URL und Formular-ID aus den Daten statt fest; neues Fragment für die Programmpunkt-Auswahl.
- `internal/adapter/admin/templates/timetable_entry.html`, `timetable.go` -- `timetableEntryView` um Kennung und geschlossenen Bereich; Auswahl in eigenem Fragment mit ID; `newTimetableEntryView`, `timetableViews`, `showTimetableEntry`.
- `internal/adapter/admin/templates/event_form.html` -- leerer Container nach dem Event-Formular für die Ort-Formulare der Programmpunkte.
- `internal/adapter/admin/templates.go` -- `timetableEntryTemplate` braucht `new_location.html`; `newLocationTemplate` braucht das Auswahl-Fragment.
- `internal/adapter/admin/static/location-map.js` -- initialisiert jedes `[data-location-map]` nach `htmx:load`; nicht ändern.
- Tests: `new_location_test.go`, `timetable_test.go` (Hilfen `memoryLocationRepo`, Fragment-Requests mit Session).

## Nahtstellen

- `createInlineLocation`/`renderSavedLocation` -- 1.8 -- Event-Ziel liefert weiter `#new-location` und OOB `#location-choice`, neu zusätzlich die Option für die Programmpunkt-Auswahlen -- bestehende Tests in `new_location_test.go` plus neuer Test.
- `newLocationClosed` im Event-Formular -- 1.8 -- Link-Fallback und `hx-get` bleiben -- `TestEventFormOffersNewLocationInlineWithLinkFallback`.
- Fragment „Programmpunkt hinzufügen“ und Re-Render nach 422/409 -- 1.9, 1.10, 5.1 -- Punkt behält gewählten Ort und Fehler, bekommt geschlossenes „Neuer Ort“ -- Admin-Tests in `timetable_test.go`.
- `timetableFromForm` -- 1.9, 5.1 -- Inline-Felder der Punkte erreichen das Event-Formular nie; fünf Listen bleiben gleich lang -- Test wie `TestSavingEventWithExtraLocationFieldsCreatesNoLocation`.

## Tasks & Acceptance

**Execution:**
- [x] `internal/adapter/admin/new_location_test.go`, `timetable_test.go` -- fehlschlagende Tests je Matrix-Fall für das Programmpunkt-Ziel, Markup-Tests für Kennung, IDs, `form`-Attribute, Link-Fallback -- Test-first
- [x] `internal/adapter/admin/new_location.go` -- Ziel aus Query-Parameter lesen und gegen das Kennungsformat prüfen, Bereich/Formular/Auswahl je Ziel; Erfolg tauscht die Auswahl des Ziels -- Verhalten
- [x] `internal/adapter/admin/timetable.go` -- Kennung je gerendertem Programmpunkt, Auswahl-Fragment-Daten -- Daten
- [x] `internal/adapter/admin/templates/*.html`, `templates.go` -- Fragmente parametrisieren, Container, Programmpunkt-Template, `data-location-options` an Event- und Programmpunkt-Auswahl, Out-of-band-Option in der Erfolgsantwort beider Ziele -- Oberfläche

**Acceptance Criteria:**
- Given ein Event-Formular mit mehreren Programmpunkten, when ich an einem davon „Neuer Ort“ öffne und speichere, then ist der Ort nur in dessen Auswahl gewählt und die übrigen Eingaben sind unverändert.
- Given offene, ungespeicherte Ortseingaben an Programmpunkten, when ich das Event speichere, then enthält die Anfrage keine Ortsfelder und das Event wird wie in 5.1 verarbeitet.
- Given `bash scripts/check-coverage.sh`, golangci-lint v2.14.0 und `CI= go test ./...`, then grün und 100 %.

## Implementation Notes

## Spec Change Log

## Review Triage Log

| Befund | Verdikt | Evidenz | Route |
|---|---|---|---|
| Entfernter Programmpunkt lässt sein leeres Formular in `#entry-location-forms` zurück (Blind, Seam, Edge) | low | Formular ohne Felder, nichts verweist mehr darauf; Abhilfe bräuchte zusätzliches Skript am Entfernen-Knopf | reject |
| Doppelklick auf „Neuer Ort“ hängt zwei Formulare mit gleicher ID an (Blind, Edge) | false | `form=` löst auf das erste, gleich gebaute Formular auf; `hx-swap-oob="delete"` sucht per `querySelectorAll` und entfernt beide | reject |
| Neue Option steht am Listenende (Blind) | false | So entschieden im Frozen-Block (Andreas, 2026-11-12) | reject |
| htmx-Verhalten (OOB über Selektor, `<option>` im Wrapper, `delete`, `form=`) ohne Browsertest (Blind) | medium | Kein JS-/Browser-Test-Setup im Repo, schon bei 1.8 zurückgestellt | defer |
| `TestSavingEventWithExtraLocationFieldsAtEntriesCreatesNoLocation` kann nicht aus dem genannten Grund scheitern (Blind) | low | Gleiches Muster wie der Test aus 1.8; die Bindung ans eigene Formular prüft `assertAllFieldsBelongTo` | reject |
| Container-Test ankert am ersten `</form>` (Blind) | false | Das Event-Formular ist das erste Formular der Seite, das Layout hat keins | reject |
| IDs und URLs doppelt in Go und Template (Blind) | low | Bestehendes Muster (1.8 Triage #7) | reject |
| Abbrechen sendet immer `delete` (Blind) | false | „Abbrechen“ gibt es nur im offenen Zustand, das Öffnen hat das Formular angehängt; Abbrechen am Event ohne OOB prüft `TestCancellingNewLocationShowsClosedArea` | reject |
| 400-Block dreifach kopiert (Blind) | low | Clean-Code-Policy, direkte Extraktion | patch |
| Eine Testmeldung deckt zwei Fehlerfälle (Blind) | low | `TestUnknownEntryLocation…` verknüpft beide Bedingungen | patch |
| Neue Kennungen bei jedem Rendern verlieren offene Eingaben (Blind) | false | Das Event-Formular wird nur als ganze Seite neu gerendert, offene Eingaben gehen dabei wie in 1.8 ohnehin verloren | reject |
| `isEntryKey` nur per Kommentar an `rand.Text` gebunden (Blind) | low | Test `isEntryKey(rand.Text())` fehlt | patch |
| Spec nicht im Review-Diff (Blind) | false | Der Diff umfasst bewusst nur Code | reject |
| Re-Render nach 409 ohne Test für „Neuer Ort“ am Punkt (Seam) | low | Nur 422 geprüft | patch |
| Erster Ort überhaupt an einem Punkt angelegt: Event-Auswahl zeigt weiter „Noch keine Orte angelegt.“ (Edge) | low | Tritt einmal je Installation auf; Abhilfe wäre ein weiterer OOB-Zweig | reject |
| Ort am Event ohne Programmpunkt: `htmx:oobErrorNoTarget` in der Konsole (Edge) | low | Nur Konsolenmeldung, kein sichtbarer Effekt; Abhilfe wäre ein Zweig | reject |
| Kommentar von `inlineLocationTargetOf` verspricht mehr, als der Code prüft (Edge) | low | Andere Parameter werden angenommen | patch |
| `<option>` im `<div>` der Antwort ist kein gültiges HTML (Edge) | low | Betrifft nur das Fragment, das htmx parst; die Seite bleibt gültig; ein Umbau auf `<template>` wäre ohne Browsertest riskant | reject |

## Design Notes

htmx 2.0.11 sammelt bei POST die Werte von `elt.form || closest('form')`. Damit die Felder eines Programmpunkts nicht ins Event-Formular gehen, zeigt ihr `form`-Attribut auf ein eigenes leeres Formular je Kennung K. Es entsteht beim Öffnen per Out-of-band-Anhängen an einen Container außerhalb des Event-Formulars und wird beim Abbrechen und nach dem Speichern per `hx-swap-oob="delete"` entfernt:

```html
<div id="entry-K-new-location">…Felder mit form="entry-K-new-location-form"…</div>
<div hx-swap-oob="beforeend:#entry-location-forms"><form id="entry-K-new-location-form"
  hx-post="/admin/events/new-location?entry=K" hx-target="#entry-K-new-location" hx-swap="outerHTML" hx-sync="this:drop" novalidate></form></div>
```

Nach dem Speichern hängt ein zweiter Out-of-band-Teil die Option an alle übrigen Auswahlen an (htmx sucht das Ziel per `querySelectorAll`, bei `beforeend` werden die Kinder eingefügt); die Auswahl des Ziels wird ausgenommen, da sie neu gerendert wird:

```html
<div hx-swap-oob="beforeend:select[data-location-options]:not(#entry-K-locationId)"><option value="ID">Name</option></div>
```

K kommt aus `crypto/rand.Text()` beim Rendern eines Programmpunkts und wird im Request gegen dessen Alphabet und Länge geprüft, da es in IDs und CSS-Selektoren landet. Die Route bleibt dieselbe, das Ziel steht im Query-Parameter `entry`.

## Verification

**Commands:**
- `CI= go test ./...` -- expected: grün
- `bash scripts/check-coverage.sh` -- expected: 100 % in den drei Paketen
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- expected: keine Funde

**Manual checks (if no CLI):**
- Browser: Event mit zwei Programmpunkten, an Punkt 2 „Neuer Ort“, ungültig, dann gültig speichern; Ort an Punkt 2 gewählt, Event-Felder unverändert; Event speichern.
