# Epic 5 Context: Programmpunkte finden an einem eigenen Ort statt

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Ein Programmpunkt eines Ablaufplans kann an einem anderen Ort stattfinden als sein Event, etwa ein Konzert des Weihnachtsmarkts in der Kirche statt am Marktplatz. Andreas wählt den Ort im Admin oder bringt ihn per Import mit, die Karten-App erhält ihn über die öffentliche API. Ohne eigenen Ort gilt der Ort des Events. Hinweis: Die Epic-Definition steht in `epics.md` an zwei Stellen (Epic-Liste und Abschnitt am Dateiende); maßgeblich ist der Abschnitt mit den Stories.

## Stories

- Story 5.1: Eigenen Ort für einen Programmpunkt im Admin wählen
- Story 5.2: Ort des Programmpunkts in der öffentlichen API
- Story 5.3: Ort des Programmpunkts im Import

## Requirements & Constraints

- Ein eigener Ort ist ein Verweis auf einen gepflegten Ort (Name, Adresse, Koordinaten, Ortsgenauigkeit), keine eigene Adresse am Programmpunkt.
- Ohne eigenen Ort findet der Programmpunkt am Ort des Events statt; zieht das Event um, ziehen solche Punkte mit.
- Wird als eigener Ort der Ort des Events gewählt, gilt das wie "kein eigener Ort". `Canonicalize` entfernt den Verweis, damit "Ort des Events" genau eine Darstellung hat.
- Ein Ort, auf den nur Programmpunkte verweisen, kann nicht gelöscht werden; die Meldung nennt, dass Programmpunkte ihn nutzen.
- Die öffentliche API liefert pro Programmpunkt den vollständigen Ort (wie beim Event) oder `location: null`. Keine interne Ortskennung in der Antwort. Die Änderung ist rückwärtskompatibel, alte Abnehmer funktionieren weiter.
- Duplikatprüfung, `effective*`, Sortierung und `RecomputeDerived` ändern ihre Regeln nicht. Der Ort eines Programmpunkts geht in keine davon ein. Keine Prüfung räumlicher Nähe zum Event.
- Import: Programmpunkt-Orte werden wie der Ort des Events aufgelöst (vorhandener Name wird zugeordnet, sonst `newLocation` einmal pro Name und Datei; ohne Adresse oder Koordinaten `error` mit Hinweis auf den Programmpunkt; mehrdeutige Namen `error`). Fehlendes `location` oder `null` heißt Ort des Events.
- Ein anderer Ort im Import gegenüber dem Bestand ergibt `update` mit `timetable` in der Änderungsanzeige; gleicher Ort ergibt `unchanged`. Ein erneuter Import alter Dateien bleibt `unchanged` (SM-2).
- Kein neuer Ort direkt aus dem Programmpunkt heraus; neue Orte entstehen über die Ortsverwaltung oder beim Event.

## Technical Decisions

- Migration `00009` (nur vorwärts, expand/contract nach AD-17): `timetable_entries.location_id uuid NULL`, Fremdschlüssel auf `locations (id) ON DELETE RESTRICT`, Index. `NULL` = Ort des Events; bestehende Daten bleiben unverändert. Keine Fachlogik in SQL.
- Kern: `TimetableEntry`/`TimetableEntryInput` bekommen einen optionalen Ort: interne Kennung im Admin, mitgebrachter Ort im Import (analog `core.EventInput`). Unbekannte Ortskennung führt zu `ErrValidation` mit deutscher Meldung am betroffenen Punkt. `DeleteLocation` zählt Programmpunkte mit (zusätzlich zum FK-Schutz, AD-6).
- Der Ablaufplan wird weiter nur als ganze Liste über `SaveEvent` ersetzt (AD-15); Schreiben nur über Kern-Anwendungsfälle.
- Spec (`api/v1/openapi.yaml`, einzige Quelle für Feldnamen, AD-9): `TimetableEntry` (Lesen, `location` als `EventLocation` oder `null`, mit Beschreibung und Beispiel) und neues `TimetableEntryInput` (Schreiben, `location` optional). `EventInput.timetable` verweist darauf. Das Import-Schema bindet `TimetableEntryInput` per `$ref` ein; `formatVersion` bleibt `1`.
- Generierter Code (sqlc, oapi-codegen) wird neu erzeugt und mitcommittet, nie von Hand geändert. Der AD-9-Vertragstest deckt die neuen Felder ab.
- Test-first; 100 % Abdeckung für `internal/core`, `publicapi/v1` und `admin`.

## UX & Interaction Patterns

- Event-Formular: Jeder Programmpunkt hat eine Ortsauswahl, Voreinstellung "Ort des Events", dazu alle vorhandenen Orte. Hinzufügen und Entfernen laufen weiter ohne Neuladen (htmx).
- Nach dem Speichern ist der gewählte Ort beim erneuten Öffnen ausgewählt; die Event-Ansicht zeigt ihn am Programmpunkt.
- Admin-Oberfläche und Meldungen auf Deutsch, Code und Fehlermeldungen im Code englisch.

## Cross-Story Dependencies

- Reihenfolge 5.1 → 5.2 → 5.3; 5.2 und 5.3 bauen auf dem Datenmodell und Kern aus 5.1 auf.
- Berührte Nahtstellen früherer Epics: `SaveEvent`/`Canonicalize`, Ablaufplan-Formular, `DeleteLocation` und Löschrückfrage (Epic 1); Leseform, Mapping, Spec-Beispiele und Cache-Header (Epic 2, 4.3); Ortsauflösung, Klassifizierung, `unchanged`-Vergleich, `isStale`, Commit und Import-Format-Doku (Epic 3).
- Abnahme nach 5.3 mit einem Weihnachtsmarkt-Programmpunkt in der Kirche; danach optional Retrospektive.
