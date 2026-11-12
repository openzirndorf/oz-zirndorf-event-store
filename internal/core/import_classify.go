package core

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// ImportClass says what importing an entry would do.
type ImportClass string

// Classes of an import entry. An entry has exactly one; error goes before
// update and unchanged, these before duplicateSuspect, and that before new.
const (
	// ImportClassNew creates a new event.
	ImportClassNew ImportClass = "new"
	// ImportClassUpdate changes the stored event with the import key of
	// the entry.
	ImportClassUpdate ImportClass = "update"
	// ImportClassUnchanged matches the stored event with the import key of
	// the entry in every field.
	ImportClassUnchanged ImportClass = "unchanged"
	// ImportClassDuplicateSuspect may duplicate a stored event or another
	// entry of the file; a person decides.
	ImportClassDuplicateSuspect ImportClass = "duplicateSuspect"
	// ImportClassError has problems and is never imported.
	ImportClassError ImportClass = "error"
)

// ImportClasses returns every import class in display order.
func ImportClasses() []ImportClass {
	return []ImportClass{
		ImportClassNew, ImportClassUpdate, ImportClassUnchanged, ImportClassDuplicateSuspect, ImportClassError,
	}
}

// ImportChange is one field an update would change, both values as text:
// the location by its name, the timetable as one field with one line per
// entry.
type ImportChange struct {
	Field string
	Old   string
	New   string
}

// ImportCandidate is what an entry may duplicate: a stored event or
// another entry of the file.
type ImportCandidate struct {
	// EventID is the stored event, empty for an entry of the file.
	EventID string
	// Position is the entry of the file, 0 for a stored event.
	Position  int
	Title     string
	StartDate LocalDate
	// Fingerprint is the EventFingerprint of the stored event, empty for an
	// entry of the file.
	Fingerprint string
	// CanOverwrite reports whether the entry may overwrite the stored
	// event: false when both have an import key and they differ, and for an
	// entry of the file.
	CanOverwrite bool
}

// ImportHintKind says what an import hint points out.
type ImportHintKind string

// Kinds of import hints.
const (
	// ImportHintDuplicate names a stored event or entry the entry may
	// duplicate, where its import key already decides its target.
	ImportHintDuplicate ImportHintKind = "duplicate"
	// ImportHintLocationDiffers names a given field of a stored location
	// that differs from the stored value; the stored location stays as it
	// is.
	ImportHintLocationDiffers ImportHintKind = "locationDiffers"
	// ImportHintNewLocationDiffers names a field of a new location that
	// differs from the first entry bringing it, whose details count.
	ImportHintNewLocationDiffers ImportHintKind = "newLocationDiffers"
)

// ImportHint points out something about an entry that does not change its
// class.
type ImportHint struct {
	Kind ImportHintKind
	// Field is the import path of the location field of a location hint.
	Field string
	// Candidate is what a duplicate hint names.
	Candidate ImportCandidate
}

// ImportNewLocation is a location the import would create, with the
// positions of the entries that bring it along.
type ImportNewLocation struct {
	Name      string
	Positions []int
}

// The text form of a timetable in an ImportChange: one line per entry,
// date, times and description separated by a space, the end time after a
// dash.
const (
	timetableLineSeparator  = "\n"
	timetablePartSeparator  = " "
	timetableRangeSeparator = "–"
	// timetableLocationSeparator precedes the location of an entry where a
	// line names it.
	timetableLocationSeparator = " · "
	// timetableAtEventLocation names the location of an entry without own
	// location.
	timetableAtEventLocation = "Ort des Events"
)

// importClassifier classifies the entries of an import file against the
// stored events and locations, read once before.
type importClassifier struct {
	events EventRepo
	// locationsByKey holds the stored locations by NormalizeKey of their
	// name; more than one share a key where RecomputeNameKeys left a
	// collision.
	locationsByKey map[string][]Location
	// locationNames holds the names of the stored locations by ID.
	locationNames map[string]string
	// eventsByImportKey holds the stored events that have an import key.
	eventsByImportKey map[string]Event
	// eventsByID holds every stored event by ID.
	eventsByID map[string]Event
}

// newImportClassifier reads all locations and events that repos store,
// archived ones included.
func newImportClassifier(ctx context.Context, repos Repos) (*importClassifier, error) {
	locations, err := repos.Locations.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list locations for import: %w", err)
	}
	events, err := repos.Events.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list events for import: %w", err)
	}
	classifier := &importClassifier{
		events:            repos.Events,
		locationsByKey:    make(map[string][]Location, len(locations)),
		locationNames:     make(map[string]string, len(locations)),
		eventsByImportKey: make(map[string]Event),
		eventsByID:        make(map[string]Event, len(events)),
	}
	for _, location := range locations {
		key := NormalizeKey(location.Name)
		classifier.locationsByKey[key] = append(classifier.locationsByKey[key], location)
		classifier.locationNames[location.ID] = location.Name
	}
	for _, event := range events {
		classifier.eventsByID[event.ID] = event
		if event.ImportKey != "" {
			classifier.eventsByImportKey[event.ImportKey] = event
		}
	}
	return classifier, nil
}

// classify sets the class of every entry read: import keys given twice,
// then the stored events, then the other entries of the file. It returns
// the new locations of the valid entries. imported holds the parsed event
// of each entry at the same index.
func (c *importClassifier) classify(ctx context.Context, entries []ImportEntry, imported []importedEvent) ([]ImportNewLocation, error) {
	markImportKeysGivenTwice(entries)
	for index := range entries {
		if entries[index].Class == ImportClassError {
			continue
		}
		if err := c.classifyAgainstStore(ctx, &entries[index], imported[index]); err != nil {
			return nil, err
		}
	}
	markDuplicatesInFile(entries, imported)
	return collectNewLocations(entries, imported), nil
}

// markImportKeysGivenTwice makes every entry an error whose normalized
// import key another entry of the file gives too.
func markImportKeysGivenTwice(entries []ImportEntry) {
	counts := make(map[string]int)
	for _, entry := range entries {
		if key := normalizeText(entry.Input.ImportKey); key != "" {
			counts[key]++
		}
	}
	for index := range entries {
		entry := &entries[index]
		if counts[normalizeText(entry.Input.ImportKey)] > 1 {
			entry.Problems = append(entry.Problems, FieldError{Field: EventFieldImportKey, Problem: ProblemDuplicateInFile})
			entry.Class = ImportClassError
		}
	}
}

// classifyAgainstStore compares a valid entry with the stored event of its
// import key, if there is one, and looks for stored duplicates. A new
// location has no stored events, so it has no stored duplicates either.
func (c *importClassifier) classifyAgainstStore(ctx context.Context, entry *ImportEntry, imported importedEvent) error {
	if !imported.isNewLocation() {
		entry.Hints = append(entry.Hints, storedLocationHints(EventFieldLocation, *entry.Input.Location, imported.location)...)
	}
	entry.Hints = append(entry.Hints, storedEntryLocationHints(entry.Input.Timetable, imported.entryLocations)...)
	target, found := c.eventsByImportKey[normalizeText(entry.Input.ImportKey)]
	// The resolved and kept own location IDs are classification output:
	// the commit writes them as they stand in entry.Input.
	entry.Input.Timetable = withEntryLocationIDs(entry.Input.Timetable, imported.entryLocations, target.Timetable)
	compared, err := imported.withLocationKeys(entry.Input.Timetable)
	if err != nil {
		return err
	}
	if found {
		compared.event.ID = target.ID
		entry.TargetID = target.ID
		entry.TargetFingerprint = EventFingerprint(target)
		entry.Changes = changesOf(target, c.namesFor(imported), compared)
		entry.Class = ImportClassUnchanged
		if entry.Changes != nil {
			entry.Class = ImportClassUpdate
		}
	}
	if imported.isNewLocation() {
		return nil
	}
	candidates, err := FindDuplicateCandidates(ctx, c.events, compared.event)
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		// The candidates come without timetable; the stored event read
		// before has it.
		stored := c.eventsByID[candidate.ID]
		addDuplicate(entry, ImportCandidate{
			EventID: candidate.ID, Title: candidate.Title, StartDate: candidate.Times.StartDate,
			Fingerprint: EventFingerprint(stored), CanOverwrite: !hasOtherImportKey(stored, *entry),
		})
	}
	return nil
}

// addDuplicate records that entry may duplicate candidate: as hint when
// its import key already gives it a target, otherwise as candidate of a
// duplicate suspect.
func addDuplicate(entry *ImportEntry, candidate ImportCandidate) {
	if entry.TargetID != "" {
		entry.Hints = append(entry.Hints, ImportHint{Kind: ImportHintDuplicate, Candidate: candidate})
		return
	}
	entry.Candidates = append(entry.Candidates, candidate)
	entry.Class = ImportClassDuplicateSuspect
}

// previewLocationKeyPrefix starts the key by which the preview refers to a
// new location. It is followed by the name key, and no ID starts with it.
const previewLocationKeyPrefix = "new:"

// previewLocationKey returns how the preview refers to location: by its ID
// when stored, by its name key when new, so Canonicalize and changesOf
// treat a new location like a stored one. The key of a new location never
// reaches SaveEvent; the commit replaces it with the created ID.
func previewLocationKey(location Location) string {
	if location.ID != "" {
		return location.ID
	}
	return previewLocationKeyPrefix + location.NameKey
}

// withLocationKeys returns the imported event with its location and the
// own locations of timetable, its entries as classified, given by
// previewLocationKey; an entry without own location in the file keeps the
// LocationID it has. The entries were checked when read, so parsing them
// again without bounds only carries their own locations into the parsed
// timetable; a problem it still finds is returned as *ValidationError.
func (i importedEvent) withLocationKeys(timetable []TimetableEntryInput) (importedEvent, error) {
	keyed := i
	keyed.event.LocationID = previewLocationKey(i.location)
	inputs := make([]TimetableEntryInput, 0, len(timetable))
	for index, entry := range timetable {
		if own := i.entryLocations[index]; own.Name != "" {
			entry.LocationID = previewLocationKey(own)
		}
		inputs = append(inputs, entry.normalized())
	}
	parsed, problems := parseTimetable(inputs, nil)
	if problems != nil {
		return importedEvent{}, fmt.Errorf("parse checked import timetable: %w", &ValidationError{Fields: problems})
	}
	keyed.event.Timetable = parsed
	return keyed, nil
}

// timetableEntryMatch is what a timetable entry of the file shares with
// the stored entry whose own location it keeps when it brings none.
type timetableEntryMatch struct {
	description string
	date        string
	startTime   string
	endTime     string
}

func timetableEntryMatchOf(entry TimetableEntryInput) timetableEntryMatch {
	normalized := entry.normalized()
	return timetableEntryMatch{
		description: normalized.Description, date: normalized.Date,
		startTime: normalized.StartTime, endTime: normalized.EndTime,
	}
}

// withEntryLocationIDs returns timetable with the stored own location of
// each entry: the stored location it names, or, when it brings no location
// and does not give null, the own location of the matching entry of
// stored, the timetable of its target. Equal entries match in order, the
// stored ones sorted. A new location stays without ID.
func withEntryLocationIDs(timetable []TimetableEntryInput, own []Location, stored []TimetableEntry) []TimetableEntryInput {
	storedLocationIDs := make(map[timetableEntryMatch][]string)
	for _, entry := range sortedTimetable(stored) {
		match := timetableEntryMatch{
			description: entry.Description, date: dateText(entry.Date),
			startTime: timeText(entry.StartTime), endTime: timeText(entry.EndTime),
		}
		storedLocationIDs[match] = append(storedLocationIDs[match], entry.LocationID)
	}
	resolved := slices.Clone(timetable)
	for index := range resolved {
		entry := &resolved[index]
		match := timetableEntryMatchOf(*entry)
		var storedLocationID string
		if ids := storedLocationIDs[match]; len(ids) > 0 {
			storedLocationID, storedLocationIDs[match] = ids[0], ids[1:]
		}
		switch {
		case own[index].Name != "":
			entry.LocationID = own[index].ID
		case !entry.AtEventLocation:
			entry.LocationID = storedLocationID
		}
	}
	return resolved
}

// importLocationNames names the locations an entry refers to: stored ones
// by ID, the new ones it brings by previewLocationKey.
type importLocationNames struct {
	stored   map[string]string
	newNames map[string]string
}

// nameOf returns the name of the location with ID or preview key key.
func (n importLocationNames) nameOf(key string) string {
	if name, ok := n.stored[key]; ok {
		return name
	}
	return n.newNames[key]
}

// namesFor returns the names of the stored locations and of the new ones
// imported brings along.
func (c *importClassifier) namesFor(imported importedEvent) importLocationNames {
	names := importLocationNames{stored: c.locationNames, newNames: make(map[string]string)}
	for _, location := range append([]Location{imported.location}, imported.entryLocations...) {
		if location.Name != "" && location.ID == "" {
			names.newNames[previewLocationKey(location)] = location.Name
		}
	}
	return names
}

// changesOf compares the canonical forms of the stored event and the
// imported one, which refers to its locations by previewLocationKey. It
// returns every changed field in the order of EventInput, none when they
// are equal.
func changesOf(stored Event, names importLocationNames, imported importedEvent) []ImportChange {
	before := EventInputOf(stored).Canonicalize()
	after := EventInputOf(imported.event).Canonicalize()
	var changes []ImportChange
	add := func(field, old, updated string) {
		if old != updated {
			changes = append(changes, ImportChange{Field: field, Old: old, New: updated})
		}
	}
	add(EventFieldTitle, before.Title, after.Title)
	add(EventFieldType, before.Type, after.Type)
	if before.LocationID != after.LocationID {
		changes = append(changes, ImportChange{Field: EventFieldLocation, Old: names.nameOf(before.LocationID), New: imported.location.Name})
	}
	add(EventFieldStartDate, before.StartDate, after.StartDate)
	add(EventFieldStartTime, before.StartTime, after.StartTime)
	add(EventFieldEndDate, before.EndDate, after.EndDate)
	add(EventFieldEndTime, before.EndTime, after.EndTime)
	add(EventFieldAllDay, strconv.FormatBool(before.AllDay), strconv.FormatBool(after.AllDay))
	add(EventFieldSourceDescription, before.Source.Description, after.Source.Description)
	add(EventFieldSourceURL, before.Source.URL, after.Source.URL)
	add(EventFieldNote, before.Note, after.Note)
	if !slices.Equal(before.Timetable, after.Timetable) {
		withLocations := hasOwnLocation(before.Timetable) || hasOwnLocation(after.Timetable)
		changes = append(changes, ImportChange{
			Field: EventFieldTimetable,
			Old:   timetableText(before.Timetable, names, withLocations),
			New:   timetableText(after.Timetable, names, withLocations),
		})
	}
	return changes
}

// hasOwnLocation reports whether an entry of timetable has an own location.
func hasOwnLocation(timetable []TimetableEntryInput) bool {
	return slices.ContainsFunc(timetable, func(entry TimetableEntryInput) bool { return entry.LocationID != "" })
}

// timetableText returns the text form of a timetable, one line per entry
// such as "2026-10-17 19:00–22:00 Vorband". withLocations appends the name
// of each entry's location, timetableAtEventLocation for none, such as
// "2026-10-17 19:00 Vorband · Kirche St. Rochus".
func timetableText(entries []TimetableEntryInput, names importLocationNames, withLocations bool) string {
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		times := entry.StartTime
		if entry.EndTime != "" {
			times += timetableRangeSeparator + entry.EndTime
		}
		parts := slices.DeleteFunc([]string{entry.Date, times, entry.Description}, func(part string) bool { return part == "" })
		line := strings.Join(parts, timetablePartSeparator)
		if withLocations {
			line += timetableLocationSeparator + cmp.Or(names.nameOf(entry.LocationID), timetableAtEventLocation)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, timetableLineSeparator)
}

// storedLocationHints names every given street, postal code, city or
// coordinate of the location object at path that differs from the stored
// location.
func storedLocationHints(path string, given LocationInput, stored Location) []ImportHint {
	var hints []ImportHint
	add := func(field string, differs bool) {
		hints = appendLocationHint(hints, ImportHintLocationDiffers, importLocationField(path, field), differs)
	}
	add(LocationFieldStreet, givenTextDiffers(given.Street, stored.Street))
	add(LocationFieldPostalCode, givenTextDiffers(given.PostalCode, stored.PostalCode))
	add(LocationFieldCity, givenTextDiffers(given.City, stored.City))
	add(LocationFieldLatitude, givenCoordinateDiffers(given.Latitude, stored.Latitude, maxLatitude))
	add(LocationFieldLongitude, givenCoordinateDiffers(given.Longitude, stored.Longitude, maxLongitude))
	return hints
}

// storedEntryLocationHints returns the storedLocationHints of every
// timetable entry that names a stored location.
func storedEntryLocationHints(timetable []TimetableEntryInput, own []Location) []ImportHint {
	var hints []ImportHint
	for index, location := range own {
		if location.ID != "" {
			path := TimetableField(index, TimetableFieldLocation)
			hints = append(hints, storedLocationHints(path, *timetable[index].Location, location)...)
		}
	}
	return hints
}

// givenTextDiffers reports whether text is given and differs from stored.
func givenTextDiffers(text, stored string) bool {
	text = normalizeText(text)
	return text != "" && text != stored
}

// givenCoordinateDiffers reports whether text is given and is no valid
// coordinate within limit or another one than stored.
func givenCoordinateDiffers(text string, stored, limit float64) bool {
	if normalizeText(text) == "" {
		return false
	}
	value, problem := parseCoordinate(text, limit)
	return problem != "" || value != stored
}

// appendLocationHint appends a hint of kind at the import path field when
// differs.
func appendLocationHint(hints []ImportHint, kind ImportHintKind, field string, differs bool) []ImportHint {
	if !differs {
		return hints
	}
	return append(hints, ImportHint{Kind: kind, Field: field})
}

// fileDuplicateKey is what two entries of a file share when one may
// duplicate the other: the title key, the start date and the location by
// NormalizeKey of its name.
type fileDuplicateKey struct {
	titleKey    string
	startDate   LocalDate
	locationKey string
}

func fileDuplicateKeyOf(imported importedEvent) fileDuplicateKey {
	return fileDuplicateKey{
		titleKey:    imported.event.TitleKey,
		startDate:   imported.event.Times.StartDate,
		locationKey: NormalizeKey(imported.location.Name),
	}
}

// markDuplicatesInFile records for every valid entry the other valid
// entries of the file with the same duplicate key, in file order.
func markDuplicatesInFile(entries []ImportEntry, imported []importedEvent) {
	positions := make(map[fileDuplicateKey][]int)
	for index, entry := range entries {
		if entry.Class != ImportClassError {
			key := fileDuplicateKeyOf(imported[index])
			positions[key] = append(positions[key], index)
		}
	}
	for index := range entries {
		if entries[index].Class == ImportClassError {
			continue
		}
		for _, other := range positions[fileDuplicateKeyOf(imported[index])] {
			if other != index {
				addDuplicate(&entries[index], ImportCandidate{
					Position: entries[other].Position, Title: entries[other].Title, StartDate: imported[other].event.Times.StartDate,
				})
			}
		}
	}
}

// broughtNewLocation is a new location an entry brings along: the
// location of the event or the own location of a timetable entry.
type broughtNewLocation struct {
	// path is the import path of the location object.
	path     string
	location Location
	// input is the location as read.
	input *LocationInput
}

// newLocationsOf returns the new locations a valid entry brings along: that
// of the event first, then those of its timetable entries in file order.
func newLocationsOf(entry ImportEntry, imported importedEvent) []broughtNewLocation {
	var brought []broughtNewLocation
	if imported.isNewLocation() {
		brought = append(brought, broughtNewLocation{path: EventFieldLocation, location: imported.location, input: entry.Input.Location})
	}
	for index, location := range imported.entryLocations {
		if location.Name != "" && location.ID == "" {
			brought = append(brought, broughtNewLocation{
				path: TimetableField(index, TimetableFieldLocation), location: location, input: entry.Input.Timetable[index].Location,
			})
		}
	}
	return brought
}

// collectNewLocations returns the new locations of the valid entries, from
// their events and timetable entries, once per NormalizeKey of the name
// with the positions bringing it. The first one's details count; a later
// one whose details differ gets a hint for each differing field.
func collectNewLocations(entries []ImportEntry, imported []importedEvent) []ImportNewLocation {
	var newLocations []ImportNewLocation
	var firsts []Location
	indexOf := make(map[string]int)
	for index := range entries {
		entry := &entries[index]
		if entry.Class == ImportClassError {
			continue
		}
		for _, brought := range newLocationsOf(*entry, imported[index]) {
			entry.NewLocation = true
			location := brought.location
			first, seen := indexOf[location.NameKey]
			if !seen {
				indexOf[location.NameKey] = len(newLocations)
				newLocations = append(newLocations, ImportNewLocation{Name: location.Name, Positions: []int{entry.Position}})
				firsts = append(firsts, location)
				continue
			}
			if positions := newLocations[first].Positions; positions[len(positions)-1] != entry.Position {
				newLocations[first].Positions = append(positions, entry.Position)
			}
			entry.Hints = append(entry.Hints, newLocationHints(brought.path, firsts[first], location)...)
		}
	}
	return newLocations
}

// newLocationHints names every field of the location object at path in
// which a later new location differs from the first one.
func newLocationHints(path string, first, later Location) []ImportHint {
	differences := []struct {
		field   string
		differs bool
	}{
		{LocationFieldName, first.Name != later.Name},
		{LocationFieldStreet, first.Street != later.Street},
		{LocationFieldPostalCode, first.PostalCode != later.PostalCode},
		{LocationFieldCity, first.City != later.City},
		{LocationFieldLatitude, first.Latitude != later.Latitude},
		{LocationFieldLongitude, first.Longitude != later.Longitude},
		{LocationFieldPrecision, first.Precision != later.Precision},
		{LocationFieldNote, first.Note != later.Note},
	}
	var hints []ImportHint
	for _, difference := range differences {
		hints = appendLocationHint(hints, ImportHintNewLocationDiffers, importLocationField(path, difference.field), difference.differs)
	}
	return hints
}
