package admin

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// timetableEntryPath answers with one empty timetable entry, which htmx
// appends to the entries of the event form (FR-4).
const timetableEntryPath = eventsPath + "/timetable-entry"

// Form fields of the timetable entries. Every entry sends the five fields,
// so the n-th value of each list belongs to the n-th entry.
const (
	timetableFormPrefix      = core.EventFieldTimetable + "."
	timetableFormDescription = timetableFormPrefix + core.TimetableFieldDescription
	timetableFormDate        = timetableFormPrefix + core.TimetableFieldDate
	timetableFormStartTime   = timetableFormPrefix + core.TimetableFieldStartTime
	timetableFormEndTime     = timetableFormPrefix + core.TimetableFieldEndTime
	timetableFormLocationID  = timetableFormPrefix + core.TimetableFieldLocationID
	timetableEntryFragment   = "timetableEntry"
)

// German texts of the timetable.
const (
	msgOutsideEvent       = "Der Programmpunkt liegt außerhalb des Event-Zeitraums."
	msgTimetableProblems  = "Bitte die markierten Programmpunkte prüfen."
	msgEntryEndsNextDay   = "Liegt die End-Uhrzeit vor der Beginn-Uhrzeit, endet der Programmpunkt am Folgetag."
	msgEntryTimeFormat    = " ist keine gültige Uhrzeit (Format HH:MM)."
	msgEntryStartTimeName = "Die Beginn-Uhrzeit"
	msgEntryEndTimeName   = "Die End-Uhrzeit"
	msgEntryLocationGone  = "Den gewählten Ort gibt es nicht mehr. Bitte einen anderen Ort auswählen."
)

// errUnevenTimetable means the timetable lists of a form differ in length,
// which no form of the admin sends.
var errUnevenTimetable = errors.New("timetable fields differ in number")

// timetableFieldMessages are the German messages for the problems the core
// reports for a field within a timetable entry.
var timetableFieldMessages = map[core.FieldError]string{
	{Field: core.TimetableFieldDescription, Problem: core.ProblemMissing}:       "Bitte eine Beschreibung angeben.",
	{Field: core.TimetableFieldDate, Problem: core.ProblemMissing}:              "Bitte ein Datum angeben.",
	{Field: core.TimetableFieldDate, Problem: core.ProblemInvalidFormat}:        "Das Datum ist kein gültiges Datum (Format JJJJ-MM-TT).",
	{Field: core.TimetableFieldDate, Problem: core.ProblemOutsideEvent}:         msgOutsideEvent,
	{Field: core.TimetableFieldStartTime, Problem: core.ProblemInvalidFormat}:   msgEntryStartTimeName + msgEntryTimeFormat,
	{Field: core.TimetableFieldStartTime, Problem: core.ProblemNonexistentTime}: msgNonexistentTime,
	{Field: core.TimetableFieldStartTime, Problem: core.ProblemOutsideEvent}:    msgOutsideEvent,
	{Field: core.TimetableFieldEndTime, Problem: core.ProblemInvalidFormat}:     msgEntryEndTimeName + msgEntryTimeFormat,
	{Field: core.TimetableFieldEndTime, Problem: core.ProblemNonexistentTime}:   msgNonexistentTime,
	{Field: core.TimetableFieldEndTime, Problem: core.ProblemNotAfterStart}:     msgEndNotAfterStart,
	{Field: core.TimetableFieldEndTime, Problem: core.ProblemOutsideEvent}:      msgOutsideEvent,
	{Field: core.TimetableFieldLocationID, Problem: core.ProblemNotFound}:       msgEntryLocationGone,
}

// timetableEntryView is the data of one entry on the event form: the
// values as entered, the locations to choose from with the own location of
// the entry selected, and the messages per field within the entry.
type timetableEntryView struct {
	Values    core.TimetableEntryInput
	Locations []selectOption
	Errors    map[string]string
	// UnknownTimeHint and EndsNextDayHint are shown at the time fields.
	UnknownTimeHint string
	EndsNextDayHint string
}

// showTimetableEntry answers with an empty entry whose location choice
// offers every stored location.
func (h *handler) showTimetableEntry(w http.ResponseWriter, r *http.Request) {
	locations, err := h.locations.ListLocations(r.Context())
	if err != nil {
		h.failEventRequest(w, r, err)
		return
	}
	view := newTimetableEntryView(core.TimetableEntryInput{}, locations, nil)
	h.renderFragment(w, timetableEntryTemplate, timetableEntryFragment, http.StatusOK, view)
}

func newTimetableEntryView(values core.TimetableEntryInput, locations []core.Location, messages map[string]string) timetableEntryView {
	return timetableEntryView{
		Values:          values,
		Locations:       entryLocationOptions(locations, values.LocationID),
		Errors:          messages,
		UnknownTimeHint: msgUnknownTime,
		EndsNextDayHint: msgEntryEndsNextDay,
	}
}

// entryLocationOptions returns one option per location with selectedID
// selected. The option for the location of the event, with an empty value,
// comes before them in the template.
func entryLocationOptions(locations []core.Location, selectedID string) []selectOption {
	options := make([]selectOption, 0, len(locations))
	for _, location := range locations {
		options = append(options, selectOption{Value: location.ID, Label: location.Name, Selected: location.ID == selectedID})
	}
	return options
}

// timetableFromForm reads the entries in the order of the form. An entry
// whose texts are all blank and that takes place at the location of the
// event is dropped, so an added but unused entry is no error. Lists of
// different length are an error.
func timetableFromForm(form url.Values) ([]core.TimetableEntryInput, error) {
	descriptions, dates := form[timetableFormDescription], form[timetableFormDate]
	startTimes, endTimes := form[timetableFormStartTime], form[timetableFormEndTime]
	locationIDs := form[timetableFormLocationID]
	count := len(descriptions)
	if len(dates) != count || len(startTimes) != count || len(endTimes) != count || len(locationIDs) != count {
		return nil, errUnevenTimetable
	}
	var entries []core.TimetableEntryInput
	for i := range count {
		entry := core.TimetableEntryInput{
			Description: descriptions[i], Date: dates[i], StartTime: startTimes[i], EndTime: endTimes[i], LocationID: locationIDs[i],
		}
		if !isBlankEntry(entry) {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func isBlankEntry(entry core.TimetableEntryInput) bool {
	return strings.TrimSpace(entry.Description+entry.Date+entry.StartTime+entry.EndTime+entry.LocationID) == ""
}

// splitTimetableProblems separates the problems of timetable entries from
// those of the other event fields and returns the German messages of each
// entry by its index.
func splitTimetableProblems(fields []core.FieldError) ([]core.FieldError, map[int]map[string]string) {
	var eventFields []core.FieldError
	byEntry := map[int]map[string]string{}
	for _, field := range fields {
		index, name, ok := core.SplitTimetableField(field.Field)
		if !ok {
			eventFields = append(eventFields, field)
			continue
		}
		if byEntry[index] == nil {
			byEntry[index] = map[string]string{}
		}
		byEntry[index][name] = fieldErrorMessage(core.FieldError{Field: name, Problem: field.Problem, Limit: field.Limit}, timetableFieldMessages)
	}
	return eventFields, byEntry
}

// timetableViews returns the entries of the form with their messages.
func timetableViews(entries []core.TimetableEntryInput, locations []core.Location, errorsByEntry map[int]map[string]string) []timetableEntryView {
	views := make([]timetableEntryView, 0, len(entries))
	for i, entry := range entries {
		views = append(views, newTimetableEntryView(entry, locations, errorsByEntry[i]))
	}
	return views
}
