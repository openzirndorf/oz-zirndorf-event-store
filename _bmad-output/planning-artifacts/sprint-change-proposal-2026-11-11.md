---
title: "Sprint Change Proposal: Eigener Ort für Programmpunkte"
status: approved
created: 2026-11-11
author: Andreas (mit Dev-Agent, bmad-correct-course)
scope: moderate
---

# Sprint Change Proposal: Eigener Ort für Programmpunkte

## 1. Problem

**Auslöser:** Neue Anforderung von Andreas (2026-11-11): Programmpunkte eines Ablaufplans sollen eine eigene, vom Event abweichende Adresse mit Koordinaten bekommen können. Ohne eigene Angabe gilt der Ort des Events.

**Beleg:** Heute verweist nur das Event auf einen Ort (FR-7). Jeder Programmpunkt erbt ihn zwangsläufig (`internal/core/timetable.go`, `TimetableEntry` ohne Ort; Tabelle `timetable_entries` aus Migration `00005` ohne `location_id`). Beim Weihnachtsmarkt steht das Programm im Ablaufplan („Christkind, Stadtkapelle“, Entscheidung vom 2026-10-03). Ein Konzert oder eine Andacht in der Kirche erscheint auf der Karte trotzdem am Marktplatz.

**Kategorie:** Neue Anforderung des Stakeholders, Erweiterung des MVP.

**Entscheidungen (Andreas, 2026-11-11):**

- Der eigene Ort ist ein **Verweis auf einen gepflegten Ort** (Name, Adresse, Koordinaten, Ortsgenauigkeit), keine eigene Adresse am Programmpunkt. Das folgt FR-7.
- Die öffentliche API liefert bei einem Programmpunkt ohne eigenen Ort `location: null`; `null` heißt „findet am Ort des Events statt“.
- Wählt der Admin als eigenen Ort den Ort des Events, gilt das wie „kein eigener Ort“ (`Canonicalize`). Zieht das Event später um, ziehen solche Punkte mit.

## 2. Auswirkungen

| # | Punkt | Status | Ergebnis |
| --- | --- | --- | --- |
| 1.1–1.3 | Auslöser und Belege | [x] | Anforderung 2026-11-11, Code-Stellen oben |
| 2.1 | Aktuelles Epic | [N/A] | Epics 1–4 sind fertig, kein laufendes Epic |
| 2.2 | Epic-Änderung | [!] | Neues Epic 5 mit drei Stories |
| 2.3–2.5 | Weitere Epics, Reihenfolge | [x] | Epics 1–4 bleiben `done`. Reihenfolge 5.1 → 5.2 → 5.3 |
| 3.1 | PRD | [!] | Glossar „Ablaufplan“, FR-4, FR-7, FR-10, FR-15, FR-16 |
| 3.2 | Architektur | [!] | AD-15, AD-14, AD-6, AD-10, Structural Seed, Capability-Map; keine neue AD |
| 3.3 | UX | [N/A] | Kein UX-Dokument. Das Event-Formular bekommt je Programmpunkt eine Ortsauswahl |
| 3.4 | Sonstige Artefakte | [x] | `epics.md`, `sprint-status.yaml`. `openapi.yaml`, `import-v1.schema.json`, Migration `00009` und README ändern die Stories |
| 4.1 | Direkte Anpassung | [x] Viable | Aufwand mittel, Risiko gering |
| 4.2 | Rollback | [x] Not viable | Nichts zurückzunehmen |
| 4.3 | MVP-Review | [x] Not viable | MVP ist erreicht und bleibt unberührt |

**Technische Auswirkung:**

- **Migration `00009` (nur vorwärts, AD-17):** `timetable_entries.location_id uuid NULL` mit Fremdschlüssel auf `locations (id) ON DELETE RESTRICT` und Index. Bestehende Programmpunkte bekommen `NULL`, also den Ort des Events. Kein Contract-Schritt nötig.
- **Kern:** `TimetableEntry`/`TimetableEntryInput` mit optionalem Ort (interne Kennung im Admin, mitgebrachter Ort im Import). `Canonicalize` entfernt einen Verweis auf den Ort des Events. `DeleteLocation` zählt Programmpunkte mit. Duplikatprüfung, `effective*`, Sortierung und `RecomputeDerived` ändern ihre Regeln nicht.
- **Spec:** `TimetableEntry` (Lesen, `location` als `EventLocation | null`) und neues `TimetableEntryInput` (Schreiben, `location` optional). Rückwärtskompatibel nach NFR-2. `formatVersion` des Imports bleibt `1`.
- **Admin:** Ortsauswahl je Programmpunkt, Anzeige in der Event-Ansicht, Meldung beim Löschschutz.
- **Import:** Ortsauflösung, `newLocation`, `unchanged`-Vergleich und Änderungsanzeige berücksichtigen den Ort der Programmpunkte.

## 3. Empfohlener Weg

Direkte Anpassung von PRD, Spine und `epics.md` mit einem neuen Epic 5 aus drei Stories, geschnitten nach Adaptern: 5.1 Kern, Datenbank und Admin; 5.2 öffentliche API; 5.3 Import. Aufwand mittel, Risiko gering: Die Migration fügt nur eine nullable Spalte hinzu, die API wächst rückwärtskompatibel, und alle Muster (Ort-Referenz, mitgebrachter Ort, `Canonicalize`) gibt es schon.

**Erwogen und verworfen:**

- **Eigene Adresse und Koordinaten direkt am Programmpunkt:** Kein Ort anlegen nötig, aber Kopie statt Referenz (gegen FR-7), ohne Ortsname und Ortsgenauigkeit, ohne Gruppierung auf der Karte und ohne Löschschutz.
- **Immer aufgelöster Ort in der API:** Für Abnehmer etwas einfacher, aber eine Abweichung wäre nur über den Vergleich der Ortsnamen erkennbar, und jede Antwort trüge den Ort doppelt.
- **Verweis auf den Event-Ort als bewusste Bindung speichern:** Hätte „Ort des Events“ zwei Darstellungen gegeben und beim Import Scheinunterschiede erzeugt.
- **Neuen Ort direkt aus dem Programmpunkt anlegen:** Nicht nötig für den Nutzen; neue Orte entstehen weiter über die Ortsverwaltung oder beim Event (1.8). Kann später als eigene Story kommen.

## 4. Änderungen

### PRD (`prds/prd-oz-zirndorf-event-store-2026-10-01/prd.md`)

#### V1a – Frontmatter

`updated: 2026-11-11`

#### V1b – Glossar, „Ablaufplan“

ALT:

> **Ablaufplan** — Geordnete Liste von Programmpunkten eines Events, jeder mit Beschreibung, Beginn und optional Ende.

NEU:

> **Ablaufplan** — Geordnete Liste von Programmpunkten eines Events, jeder mit Beschreibung, Beginn, optional Ende und optional einem eigenen Ort. Ohne eigenen Ort findet ein Programmpunkt am Ort des Events statt.

#### V1c – FR-4

ALT:

> Ein Event kann einen Ablaufplan mit beliebig vielen Programmpunkten haben, jeder mit Beschreibung, Beginn und optionalem Ende.
>
> **Konsequenzen (testbar):**
> - Programmpunkte werden chronologisch nach Beginn sortiert ausgeliefert.

NEU:

> Ein Event kann einen Ablaufplan mit beliebig vielen Programmpunkten haben, jeder mit Beschreibung, Beginn, optionalem Ende und optionalem eigenem Ort.
>
> **Konsequenzen (testbar):**
> - Programmpunkte werden chronologisch nach Beginn sortiert ausgeliefert.
> - Ein Programmpunkt ohne eigenen Ort findet am Ort des Events statt. Ändert sich der Ort des Events, gilt der neue Ort auch für diese Programmpunkte.
> - Ein eigener Ort ist ein Verweis auf einen gepflegten Ort (FR-6, FR-7), keine eigene Adresse.
> - Wird als eigener Ort der Ort des Events gewählt, gilt das wie „kein eigener Ort“.

#### V1d – FR-7

ALT:

> - Ein Ort, auf den noch Events (auch archivierte) verweisen, kann nicht gelöscht werden.

NEU:

> - Ein Ort, auf den noch Events (auch archivierte) oder Programmpunkte verweisen, kann nicht gelöscht werden.

#### V1e – FR-10, neue Konsequenz nach der zweiten

> - Jeder Programmpunkt enthält seinen eigenen Ort vollständig (wie den Ort des Events) oder `null`, wenn der Ort des Events gilt.

#### V1f – FR-15, neue Konsequenz nach der zweiten

> - Für jeden Programmpunkt kann der Admin einen vorhandenen Ort wählen. Voreingestellt ist „Ort des Events“.

#### V1g – FR-16, dritte Konsequenz

ALT:

> - Jedes Event in der Import-Datei bringt seinen Ort mit, mindestens mit Namen. Ein mitgebrachter Ort mit demselben Namen wie ein vorhandener wird dem vorhandenen zugeordnet. Die Import-Datei enthält keine internen Kennungen.

NEU:

> - Jedes Event in der Import-Datei bringt seinen Ort mit, mindestens mit Namen. Ein mitgebrachter Ort mit demselben Namen wie ein vorhandener wird dem vorhandenen zugeordnet. Die Import-Datei enthält keine internen Kennungen. Ein Programmpunkt kann ebenso einen eigenen Ort mitbringen; dafür gelten dieselben Regeln.

### Architektur (`ARCHITECTURE-SPINE.md`)

#### V2a – Frontmatter

`updated: '2026-11-11'`

#### V2b – AD-15

**Binds** um `FR-7` ergänzen. In **Rule** nach „…nach dem Zeitmodell aus AD-3.“ einfügen:

> Optional verweist ein Eintrag auf einen eigenen Ort (`locationId`, intern). Ohne Verweis findet er am Ort des Events statt. Ein Verweis auf den Ort des Events wird von `Canonicalize` entfernt, damit „Ort des Events“ genau eine Darstellung hat. Der Verweis ist eine Ort-Referenz wie beim Event (FR-7). Der Ablaufplan prüft keine räumliche Nähe zum Event.

#### V2c – AD-14

In **Rule** nach dem Satz zur Leseform („… Einen Archivstatus enthält sie nicht (AD-5).“) einfügen:

> Ein Eintrag in `timetable` hat in beiden Formen `description`, `date`, `startTime`, `endTime` und `location`. In der Schreibform ist `location` ein mitgebrachter Ort wie beim Event (`name` Pflicht) oder fehlt bzw. ist `null`. In der Leseform ist `location` der vollständige Ort oder `null`, wenn der Ort des Events gilt. Die Spec führt dafür zwei Schemas, `TimetableEntry` (Lesen) und `TimetableEntryInput` (Schreiben). Im Kern trägt der Eintrag wie `core.EventInput` entweder die interne Ortskennung (Admin) oder den mitgebrachten Ort (Import).

#### V2d – AD-6, Löschschutz

ALT:

> Der Löschschutz für Orte (FR-7) liegt im Kern und wird zusätzlich durch einen Fremdschlüssel `ON DELETE RESTRICT` abgesichert.

NEU:

> Der Löschschutz für Orte (FR-7) liegt im Kern. Er zählt Verweise von Events und von Programmpunkten und wird zusätzlich durch je einen Fremdschlüssel `ON DELETE RESTRICT` abgesichert (`events.location_id`, `timetable_entries.location_id`).

#### V2e – AD-10, Upload (Satz an Punkt 1 anhängen)

> Mitgebrachte Orte von Programmpunkten werden genauso aufgelöst wie der Ort des Events: über `name_key` dem vorhandenen Ort zugeordnet oder als `newLocation` angezeigt, einmal pro Name und Datei. Ein Unterschied im Ort eines Programmpunkts zählt beim `unchanged`-Vergleich und in der Änderungsanzeige von `update` als Änderung des Feldes `timetable`.

#### V2f – Structural Seed, ER-Diagramm

Beziehung ergänzen:

```
  LOCATIONS ||--o{ TIMETABLE_ENTRIES : "optional (ON DELETE RESTRICT)"
```

Feld in `TIMETABLE_ENTRIES` ergänzen:

```
    uuid location_id "NULL = Ort des Events"
```

#### V2g – Capability → Architecture Map, Zeile FR-1–FR-5

Spalte AD um `AD-11` ergänzen (Ortsauflösung im Import).

### Epics (`epics.md`)

#### V3a – Requirements Inventory

- FR-4: „… und optionalem eigenem Ort. Ohne eigenen Ort gilt der Ort des Events.“
- FR-7: „… auf den noch Events oder Programmpunkte verweisen …“
- AD-6, AD-10, AD-14, AD-15 je um einen Satz nach V2 ergänzen.
- Neu unter „Ergänzende Festlegungen“: **ENT-26 (→ AD-15, AD-14) Ort eines Programmpunkts:** optionaler Verweis auf einen Ort; `null` = Ort des Events, in Lese- und Schreibform. `Canonicalize` entfernt einen Verweis auf den Ort des Events. Der Löschschutz zählt Programmpunkte mit.
- Einleitungssätze (Overview und „Ergänzende Festlegungen“) um „ENT-26 mit dem Sprint Change Proposal vom 2026-11-11“ ergänzen.

#### V3b – FR Coverage Map

```
FR-4: Epic 1 - Ablaufplan; eigener Ort je Programmpunkt in Epic 5
FR-7: Epic 1 - Ortsreferenz und Löschschutz; Programmpunkte in Epic 5
FR-10: Epic 2 - Kartengerechte Antwort mit vollständigem Ort; Ort des Programmpunkts in Epic 5 (Story 5.2)
FR-16: Epic 3 - Versioniertes Import-Format; Ort des Programmpunkts in Epic 5 (Story 5.3)
```

#### V3c – Epic List, neu nach Epic 4

```
### Epic 5: Programmpunkte finden an einem eigenen Ort statt
**FRs covered:** FR-4, FR-7, FR-10, FR-15, FR-16 (Erweiterung), AD-6, AD-14, AD-15, ENT-26
```

#### V3d – Neues Epic 5 am Ende von `epics.md`

## Epic 5: Programmpunkte finden an einem eigenen Ort statt

Ein Programmpunkt kann an einem anderen Ort stattfinden als sein Event, zum Beispiel ein Konzert des Weihnachtsmarkts in der Kirche. Andreas wählt den Ort im Admin oder bringt ihn per Import mit, die Karten-App bekommt ihn über die API. Ohne eigenen Ort gilt der Ort des Events.

**Hinweis:** Sprint Change Proposal 2026-11-11. Reihenfolge 5.1 → 5.2 → 5.3; 5.2 und 5.3 bauen auf dem Datenmodell aus 5.1 auf.

### Story 5.1: Eigenen Ort für einen Programmpunkt im Admin wählen

Als Admin,
möchte ich für einen Programmpunkt einen anderen Ort als den des Events wählen,
damit zum Beispiel das Konzert in der Kirche nicht am Marktplatz eingezeichnet wird.

**Deckt ab:** FR-4, FR-7, FR-15, AD-6, AD-14, AD-15, ENT-26

**Datenmodell:** Migration `00009`: Spalte `timetable_entries.location_id uuid NULL` mit Fremdschlüssel auf `locations (id) ON DELETE RESTRICT` und Index. `NULL` = Ort des Events.

**Acceptance Criteria:**

**Angenommen** das Event-Formular
**Wenn** ich einen Programmpunkt hinzufüge
**Dann** hat er eine Ortsauswahl mit „Ort des Events“ als Voreinstellung und allen vorhandenen Orten
**Und** Hinzufügen und Entfernen gehen weiter ohne Neuladen (htmx)

**Angenommen** ein Programmpunkt mit gewähltem eigenem Ort
**Wenn** ich speichere und das Event wieder öffne
**Dann** ist dieser Ort ausgewählt, und die Event-Ansicht zeigt ihn am Programmpunkt

**Angenommen** ein Programmpunkt, für den ich den Ort des Events auswähle
**Wenn** ich speichere
**Dann** speichert der Kern keinen Verweis (`Canonicalize`), und beim nächsten Öffnen steht „Ort des Events“

**Angenommen** ein Programmpunkt ohne eigenen Ort
**Wenn** ich den Ort des Events ändere
**Dann** gilt für den Programmpunkt der neue Ort

**Angenommen** eine Ortskennung im Formular, die es nicht gibt
**Wenn** ich speichere
**Dann** lehnt der Kern das Speichern mit einer deutschen Meldung am betroffenen Punkt ab (`ErrValidation`)

**Angenommen** ein Ort, auf den nur Programmpunkte verweisen
**Wenn** ich ihn löschen will
**Dann** lehnt `DeleteLocation` das ab, und die Meldung nennt, dass Programmpunkte ihn nutzen

**Außerdem gilt:**
- Der Ablaufplan wird weiter nur als ganze Liste über `SaveEvent` ersetzt (AD-15).
- Duplikatprüfung, `effective*` und Sortierung bleiben unberührt. Der Ort eines Programmpunkts geht in keine davon ein.
- Kein neuer Ort direkt aus dem Programmpunkt heraus; neue Orte entstehen wie bisher über die Ortsverwaltung oder beim Event (1.8).
- sqlc-Code wird neu erzeugt und mitcommittet, nicht von Hand geändert.
- Nahtstellen: `SaveEvent` und `Canonicalize` (1.7, 1.9), Ablaufplan-Formular (1.9), `DeleteLocation` und Löschrückfrage (1.11), `RecomputeDerived` liest Programmpunkte (2.5), Ablaufplan-Ersetzen im Import-Commit (3.3).

### Story 5.2: Ort des Programmpunkts in der öffentlichen API

Als Karten-App,
möchte ich bei jedem Programmpunkt erfahren, ob er an einem eigenen Ort stattfindet,
damit ich ihn an der richtigen Stelle zeigen kann.

**Deckt ab:** FR-4, FR-10, NFR-2, NFR-3, AD-8, AD-9, AD-14, ENT-26

**Acceptance Criteria:**

**Angenommen** ein Event mit einem Programmpunkt mit eigenem Ort und einem ohne
**Wenn** ich `/v1/events` oder `/v1/archive/events` abrufe
**Dann** hat der erste `location` vollständig wie der Ort des Events (`name`, `address`, `latitude`, `longitude`, `precision`, `note`)
**Und** der zweite hat `location: null`

**Angenommen** die Spec
**Wenn** ich `TimetableEntry` lese
**Dann** ist `location` als `EventLocation` oder `null` beschrieben, mit dem Satz, dass `null` „findet am Ort des Events statt“ heißt, und mit Beispiel
**Und** `EventInput.timetable` verweist auf ein eigenes Schema `TimetableEntryInput`, dessen `location` optional ist

**Angenommen** ein Abnehmer, der `location` am Programmpunkt nicht kennt
**Wenn** er die Antwort liest
**Dann** funktioniert er weiter (rückwärtskompatible Ergänzung nach NFR-2)

**Außerdem gilt:**
- Keine internen Werte in der Antwort (FR-10): keine Ortskennung am Programmpunkt.
- `api.gen.go` wird mit `go generate ./...` neu erzeugt, nicht von Hand geändert.
- Der AD-9-Vertragstest deckt die neuen Felder ab.
- Nahtstellen: Leseform und Mapping (2.3, 2.8), Spec-Beispiele und Doku (2.6), Cache-Header (4.3).

### Story 5.3: Ort des Programmpunkts im Import

Als Admin,
möchte ich in der Import-Datei für einen Programmpunkt einen eigenen Ort mitgeben,
damit recherchierte Programme mit ihren Orten ohne Nacharbeit im Bestand landen.

**Deckt ab:** FR-4, FR-16, FR-17, AD-9, AD-10, AD-11, AD-14, ENT-26

**Acceptance Criteria:**

**Angenommen** ein Programmpunkt mit `location` mit dem Namen eines vorhandenen Orts
**Wenn** ich die Datei hochlade
**Dann** wird er diesem Ort zugeordnet

**Angenommen** ein Programmpunkt mit `location` mit einem neuen Namen und vollständigen Angaben
**Wenn** ich die Datei hochlade
**Dann** zeigt die Vorschau den Ort als `newLocation`, einmal pro Name und Datei, auch wenn ihn mehrere Events oder Programmpunkte mitbringen
**Und** ohne Adresse oder Koordinaten ist der Eintrag `error` mit Hinweis auf den Programmpunkt

**Angenommen** ein Programmpunkt ohne `location` oder mit `location: null`
**Wenn** ich importiere
**Dann** gilt der Ort des Events

**Angenommen** ein vorhandenes Event mit `importKey`, dessen Programmpunkt in der Datei einen anderen Ort hat
**Wenn** ich hochlade
**Dann** ist der Eintrag `update`, und die Änderungsanzeige nennt `timetable`
**Und** derselbe Ort wie im Bestand ergibt `unchanged`

**Angenommen** eine Datei aus der Zeit vor dieser Story
**Wenn** ich sie erneut importiere
**Dann** ändert sich nichts (`unchanged`), denn fehlendes `location` heißt Ort des Events

**Außerdem gilt:**
- Das Import-Schema bindet `TimetableEntryInput` per `$ref` ein (AD-9); `formatVersion` bleibt `1`, weil das Feld optional ist.
- Mehrdeutige Ortsnamen sind `error` wie beim Ort des Events (C1).
- Nahtstellen: Ortsauflösung und Klassifizierung (3.2, C1), `unchanged`-Vergleich und Änderungsanzeige (3.2), `isStale` und Commit (3.3, 3.6), Import-Format-Doku (3.1).

### Sprint-Status (`implementation-artifacts/sprint-status.yaml`)

#### V4

Nach dem Block von Epic 4 einfügen, `last_updated` setzen:

```yaml
  epic-5: backlog
  5-1-eigenen-ort-für-einen-programmpunkt-im-admin-wählen: backlog
  5-2-ort-des-programmpunkts-in-der-öffentlichen-api: backlog
  5-3-ort-des-programmpunkts-im-import: backlog
  epic-5-retrospective: optional
```

## 5. Übergabe

**Umfang:** Moderate. Planungsartefakte werden angepasst und ein neues Epic kommt in den Backlog; kein Neuplan nötig.

| Rolle | Aufgabe |
| --- | --- |
| Andreas mit Dev-Agent (`bmad-correct-course`) | V1–V4 in PRD, Spine, `epics.md` und `sprint-status.yaml` einarbeiten, per Pull Request (nie direkt auf `main`) |
| Dev-Agent (`bmad-build`) | Stories 5.1, 5.2, 5.3 nacheinander umsetzen, test-first, 100 % Abdeckung in `core`, `publicapi/v1` und `admin` |
| Andreas | Review und Merge je Story; nach 5.3 Abnahme mit einem Weihnachtsmarkt-Programmpunkt in der Kirche, dann Retrospektive Epic 5 (optional) |

**Erfolgskriterien:**

- Ein Programmpunkt mit eigenem Ort erscheint in `/v1/events` mit vollständigem Ort, einer ohne mit `location: null`.
- Ein Ort, den nur ein Programmpunkt nutzt, lässt sich nicht löschen.
- Ein zweiter Import der Testsammlung erzeugt weiterhin 0 neue Events und nur `unchanged` (SM-2).
- Migration `00009` läuft auf dem Produktionsbestand ohne Datenänderung; alle bestehenden Programmpunkte liefern `location: null`.
