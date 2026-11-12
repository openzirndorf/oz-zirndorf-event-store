package admin

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/openzirndorf/oz-zirndorf-event-store/internal/core"
)

// Routes of the inline input for a new location on the event form (FR-15).
// htmx swaps their fragments into the event form, which itself is never
// sent or reloaded.
const (
	inlineLocationPath       = eventsPath + "/new-location"
	inlineLocationCancelPath = inlineLocationPath + "/cancel"
)

// Element ids the fragments rely on. The inline fields belong to a separate,
// empty form after the event form, so saving the event never sends them.
const (
	newLocationAreaID      = "new-location"
	locationChoiceID       = "location-choice"
	inlineLocationFormID   = "new-location-form"
	inlineLocationIDPrefix = newLocationAreaID + "-"
)

// The inline input of a timetable entry is told apart by the key of the
// entry, a query parameter of its routes. The key comes from rand.Text when
// the entry is rendered and ends up in element ids and CSS selectors, so a
// request may only name a key of that alphabet and length.
const (
	entryQueryParam = "entry"
	entryKeyLength  = 26
	entryKeyChars   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
)

// Element ids of a timetable entry, each the entry id plus a suffix. The
// forms of the inline inputs of the entries are appended to a container
// after the event form, so saving the event never sends their fields.
const (
	entryIDPrefix             = "entry-"
	entryNewLocationSuffix    = "-" + newLocationAreaID
	entryLocationChoiceSuffix = "-" + locationChoiceID
	entryLocationSelectSuffix = "-" + core.TimetableFieldLocationID
	inlineLocationFormSuffix  = "-form"
	inlineLocationFieldSep    = "-"
	entryLocationFormsID      = "entry-location-forms"
)

// Names of the fragment templates in new_location.html.
const (
	newLocationOpenTemplate   = "newLocationOpen"
	newLocationClosedTemplate = "newLocationClosed"
	newLocationSavedTemplate  = "newLocationSaved"
)

// msgLocationCreatedFormat confirms the new location, which is now selected.
const msgLocationCreatedFormat = "Ort „%s“ angelegt und ausgewählt."

// inlineLocationTarget is the location choice a new location is created
// for: the choice of the event or, with an entry key, that of the entry.
type inlineLocationTarget struct {
	entryKey string
}

// inlineLocationTargetOf reads the target of a request from its query. ok
// is false only for a malformed query or an entry parameter that is not
// exactly one valid key; other parameters are ignored.
func inlineLocationTargetOf(r *http.Request) (target inlineLocationTarget, ok bool) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return inlineLocationTarget{}, false
	}
	keys, present := query[entryQueryParam]
	if !present {
		return inlineLocationTarget{}, true
	}
	if len(keys) != 1 || !isEntryKey(keys[0]) {
		return inlineLocationTarget{}, false
	}
	return inlineLocationTarget{entryKey: keys[0]}, true
}

// isEntryKey reports whether key has the alphabet and length of rand.Text.
func isEntryKey(key string) bool {
	return len(key) == entryKeyLength && strings.Trim(key, entryKeyChars) == ""
}

func (t inlineLocationTarget) isEvent() bool {
	return t.entryKey == ""
}

func (t inlineLocationTarget) areaID() string {
	if t.isEvent() {
		return newLocationAreaID
	}
	return entryIDPrefix + t.entryKey + entryNewLocationSuffix
}

func (t inlineLocationTarget) formID() string {
	if t.isEvent() {
		return inlineLocationFormID
	}
	return t.areaID() + inlineLocationFormSuffix
}

func (t inlineLocationTarget) fieldIDPrefix() string {
	if t.isEvent() {
		return inlineLocationIDPrefix
	}
	return t.areaID() + inlineLocationFieldSep
}

// selectID returns the id of the select the new location is chosen in.
func (t inlineLocationTarget) selectID() string {
	if t.isEvent() {
		return core.EventFieldLocationID
	}
	return entryLocationSelectID(t.entryKey)
}

// routeURL returns path with the entry key of the target, if any.
func (t inlineLocationTarget) routeURL(path string) string {
	if t.isEvent() {
		return path
	}
	return path + "?" + url.Values{entryQueryParam: {t.entryKey}}.Encode()
}

// newLocationArea is the data of the inline input, open with its fields or
// closed with an optional notice.
type newLocationArea struct {
	ID        string
	OpenURL   string
	CancelURL string
	Fields    locationFields
	Conflict  *locationConflict
	Notice    string
	// LocationForm is the empty form of an entry, appended out of band when
	// its input opens.
	LocationForm *inlineLocationForm
	// RemovedFormID is the form of an entry, removed out of band when its
	// input closes.
	RemovedFormID string
}

// inlineLocationForm is the empty form that the fields of an inline input
// belong to, appended to the container with ContainerID.
type inlineLocationForm struct {
	ID          string
	Action      string
	Target      string
	ContainerID string
}

// newLocationOption is the saved location as an option, appended out of
// band to every location choice of the page except the one of the target.
type newLocationOption struct {
	Option           selectOption
	ExcludedSelectID string
}

// newLocationSaved is the answer to a saved location: the closed input,
// the choice of the target with the new location selected, swapped out of
// band, and the new option for the other choices. Either Choice (event) or
// EntryChoice (timetable entry) is set.
type newLocationSaved struct {
	Area        newLocationArea
	Choice      *locationChoice
	EntryChoice *entryLocationChoice
	NewOption   newLocationOption
}

// inlineLocationTarget returns the target of a request, or answers 400 and
// ok false when the request names none.
func (h *handler) inlineLocationTarget(w http.ResponseWriter, r *http.Request) (target inlineLocationTarget, ok bool) {
	target, ok = inlineLocationTargetOf(r)
	if !ok {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	}
	return target, ok
}

func (h *handler) openInlineLocation(w http.ResponseWriter, r *http.Request) {
	target, ok := h.inlineLocationTarget(w, r)
	if !ok {
		return
	}
	h.renderFragment(w, newLocationTemplate, newLocationOpenTemplate, http.StatusOK, target.openingArea())
}

func (h *handler) cancelInlineLocation(w http.ResponseWriter, r *http.Request) {
	target, ok := h.inlineLocationTarget(w, r)
	if !ok {
		return
	}
	h.renderFragment(w, newLocationTemplate, newLocationClosedTemplate, http.StatusOK, target.closingArea(""))
}

// createInlineLocation saves the location through the same use case and
// with the same messages as the location form: field messages (422), name
// conflict (409). On success it selects the new location in the choice of
// the target.
func (h *handler) createInlineLocation(w http.ResponseWriter, r *http.Request) {
	target, ok := h.inlineLocationTarget(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxLocationFormBytes)
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	values := inputFromForm(r.PostForm)
	saved, err := h.locations.SaveLocation(r.Context(), "", withDecimalPoints(values))

	area := target.openArea(values)
	var validation *core.ValidationError
	var conflict *core.LocationConflictError
	switch {
	case err == nil:
		h.renderSavedLocation(w, r, target, saved)
	case errors.As(err, &validation):
		area.Fields.Errors = fieldErrorMessages(validation.Fields, locationFieldMessages)
		h.renderFragment(w, newLocationTemplate, newLocationOpenTemplate, http.StatusUnprocessableEntity, area)
	case errors.As(err, &conflict):
		area.Conflict = locationConflictOf(conflict)
		h.renderFragment(w, newLocationTemplate, newLocationOpenTemplate, http.StatusConflict, area)
	default:
		h.failLocationRequest(w, r, err)
	}
}

// renderSavedLocation closes the input with a notice, replaces the location
// choice of the target, with the saved location selected, and offers the
// saved location in every other choice of the page.
func (h *handler) renderSavedLocation(w http.ResponseWriter, r *http.Request, target inlineLocationTarget, saved core.Location) {
	locations, err := h.locations.ListLocations(r.Context())
	if err != nil {
		h.failLocationRequest(w, r, err)
		return
	}
	answer := newLocationSaved{
		Area: target.closingArea(fmt.Sprintf(msgLocationCreatedFormat, saved.Name)),
		NewOption: newLocationOption{
			Option:           selectOption{Value: saved.ID, Label: saved.Name},
			ExcludedSelectID: target.selectID(),
		},
	}
	if target.isEvent() {
		choice := locationChoiceOf(locations, saved.ID, "")
		choice.OutOfBand = true
		answer.Choice = &choice
	} else {
		choice := entryLocationChoiceOf(target.entryKey, locations, saved.ID, "")
		choice.OutOfBand = true
		answer.EntryChoice = &choice
	}
	h.renderFragment(w, newLocationTemplate, newLocationSavedTemplate, http.StatusOK, answer)
}

// closedArea returns the closed input as the page renders it.
func (t inlineLocationTarget) closedArea() newLocationArea {
	return newLocationArea{
		ID:        t.areaID(),
		OpenURL:   t.routeURL(inlineLocationPath),
		CancelURL: t.routeURL(inlineLocationCancelPath),
	}
}

// openArea returns the open input filled with values.
func (t inlineLocationTarget) openArea(values core.LocationInput) newLocationArea {
	area := t.closedArea()
	area.Fields = newLocationFields(locationFields{
		IDPrefix: t.fieldIDPrefix(),
		FormID:   t.formID(),
		Values:   values,
	})
	return area
}

// openingArea returns the empty open input. The form of the event is part
// of the page; the form of an entry is appended with the input.
func (t inlineLocationTarget) openingArea() newLocationArea {
	area := t.openArea(core.LocationInput{})
	if !t.isEvent() {
		area.LocationForm = &inlineLocationForm{
			ID:          t.formID(),
			Action:      t.routeURL(inlineLocationPath),
			Target:      t.areaID(),
			ContainerID: entryLocationFormsID,
		}
	}
	return area
}

// closingArea returns the closed input with notice after it was open; the
// form of an entry goes with it.
func (t inlineLocationTarget) closingArea(notice string) newLocationArea {
	area := t.closedArea()
	area.Notice = notice
	if !t.isEvent() {
		area.RemovedFormID = t.formID()
	}
	return area
}
