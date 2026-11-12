package admin

import (
	"context"
	"crypto/rand"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/openzirndorf/oz-zirndorf-event-store/internal/core"
)

// listFailingLocations saves through the real core but cannot list, so the
// failure after a successful save is observable.
type listFailingLocations struct {
	LocationUseCases
	err error
}

func (f listFailingLocations) ListLocations(context.Context) ([]core.Location, error) {
	return nil, f.err
}

// fieldTags matches every input, select and textarea tag of a fragment.
var fieldTags = regexp.MustCompile(`<(input|select|textarea)\b[^>]*>`)

// inlineFormAttribute ties a field to the separate location form.
const inlineFormAttribute = `form="` + inlineLocationFormID + `"`

func (ts *testServer) htmxGet(path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set(htmxRequestHeader, htmxRequestTrue)
	return ts.do(withCookie(req, ts.validCookie()))
}

func (ts *testServer) htmxPost(path string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(htmxRequestHeader, htmxRequestTrue)
	return ts.do(withCookie(req, ts.validCookie()))
}

// assertFragment checks that a response is a fragment, not a whole page.
func assertFragment(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	assertBodyLacks(t, rec, "<html", "<head", "<!doctype")
}

// assertAllFieldsBelongToInlineForm checks that no location field would be
// sent with the event form.
func assertAllFieldsBelongToInlineForm(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	tags := fieldTags.FindAllString(rec.Body.String(), -1)
	if len(tags) != len(hallForm()) {
		t.Errorf("fragment has %d fields, want %d", len(tags), len(hallForm()))
	}
	for _, tag := range tags {
		if !strings.Contains(tag, inlineFormAttribute) {
			t.Errorf("field %s is not tied to the location form", tag)
		}
	}
}

func TestEventFormOffersNewLocationInlineWithLinkFallback(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.get(newEventPath)

	assertStatusCode(t, rec, http.StatusOK)
	assertBodyContains(t, rec,
		`<div id="`+locationChoiceID+`">`,
		`<div id="`+newLocationAreaID+`">`,
		`<a href="`+newLocationPath+`" hx-get="`+inlineLocationPath+`" hx-target="#`+newLocationAreaID+`" hx-swap="outerHTML">Neuer Ort</a>`,
	)
	assertBodyContains(t, rec, mapAssets...)
	assertBodyLacks(t, rec, inlineFormAttribute, `role="status"`)
}

func TestEventFormHasEmptyLocationFormAfterEventForm(t *testing.T) {
	ts := newTestServer(t)

	body := ts.get(newEventPath).Body.String()

	locationForm := `<form id="` + inlineLocationFormID + `" hx-post="` + inlineLocationPath +
		`" hx-target="#` + newLocationAreaID + `" hx-swap="outerHTML" hx-sync="this:drop" novalidate></form>`
	eventFormEnd := strings.Index(body, "</form>")
	if position := strings.Index(body, locationForm); position < 0 || position < eventFormEnd {
		t.Errorf("location form at %d, event form ends at %d, want an empty location form after it", position, eventFormEnd)
	}
	if got := strings.Count(body, "<form"); got != 2 {
		t.Errorf("page has %d forms, want the event form and the location form", got)
	}
}

func TestEventFormWithoutLocationsStillOffersNewLocation(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.get(newEventPath)

	assertBodyContains(t, rec, msgNoLocations, `hx-get="`+inlineLocationPath+`"`)
}

func TestLayoutSwapsConflictAndValidationResponses(t *testing.T) {
	ts := newTestServer(t)
	pages := map[string]*httptest.ResponseRecorder{
		"login":      ts.do(httptest.NewRequest(http.MethodGet, loginPath, nil)),
		"event form": ts.get(newEventPath),
	}
	for name, rec := range pages {
		t.Run(name, func(t *testing.T) {
			assertBodyContains(t, rec, `<meta name="htmx-config" content='{"responseHandling":[`+
				`{"code":"204","swap":false},{"code":"[23]..","swap":true},`+
				`{"code":"404|409|422","swap":true,"error":false},`+
				`{"code":"[45]..","swap":false,"error":true},{"code":"...","swap":false}]}'>`)
		})
	}
}

func TestOpeningNewLocationShowsEmptyFieldsWithMap(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.htmxGet(inlineLocationPath)

	assertStatusCode(t, rec, http.StatusOK)
	assertFragment(t, rec)
	assertAllFieldsBelongToInlineForm(t, rec)
	assertBodyContains(t, rec,
		`<div id="`+newLocationAreaID+`">`,
		`<label for="new-location-name">Name</label>`,
		`id="new-location-street" name="`+core.LocationFieldStreet+`"`,
		`id="new-location-note" name="`+core.LocationFieldNote+`"`,
		`<div id="new-location-location-map" class="location-map" data-location-map`+
			` data-latitude-field="new-location-latitude" data-longitude-field="new-location-longitude"`+
			` aria-label="Karte zum Setzen der Koordinaten" hidden></div>`,
		`<option value="building">Gebäude</option>`,
		`<button type="submit" form="`+inlineLocationFormID+`">Ort speichern</button>`,
		`<button type="button" hx-get="`+inlineLocationCancelPath+`" hx-target="#`+newLocationAreaID+`" hx-swap="outerHTML">Abbrechen</button>`,
	)
	assertBodyLacks(t, rec, `id="name"`, `id="note"`, `id="location-map"`, `value="49`)
}

func TestCancellingNewLocationShowsClosedArea(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.htmxGet(inlineLocationCancelPath)

	assertStatusCode(t, rec, http.StatusOK)
	assertFragment(t, rec)
	assertBodyContains(t, rec, `<div id="`+newLocationAreaID+`">`, `hx-get="`+inlineLocationPath+`"`, ">Neuer Ort</a>")
	assertBodyLacks(t, rec, inlineFormAttribute, `role="status"`, `hx-swap-oob`)
	if len(ts.locations.locations) != 0 {
		t.Error("cancelling stored a location")
	}
}

func TestSavingValidNewLocationSelectsItInTheChoice(t *testing.T) {
	ts := newTestServer(t)
	park := ts.seed(t, "Bibertpark")
	form := hallForm()
	form.Set(core.LocationFieldLatitude, "49,44")
	form.Set(core.LocationFieldLongitude, "10,95")

	rec := ts.htmxPost(inlineLocationPath, form)

	assertStatusCode(t, rec, http.StatusOK)
	assertFragment(t, rec)
	hall, err := ts.locations.FindByNameKey(context.Background(), core.NormalizeKey(hallName))
	if err != nil {
		t.Fatalf("location not stored: %v", err)
	}
	if hall.Latitude != 49.44 || hall.Longitude != 10.95 {
		t.Errorf("coordinates = %v, %v, want 49.44, 10.95", hall.Latitude, hall.Longitude)
	}
	assertBodyContains(t, rec,
		`<div id="`+newLocationAreaID+`">`,
		`<p class="hint" role="status">Ort „Paul-Metz-Halle“ angelegt und ausgewählt.</p>`,
		`hx-get="`+inlineLocationPath+`"`,
		`<div id="`+locationChoiceID+`" hx-swap-oob="true">`,
		`<option value="`+hall.ID+`" selected>`+hallName+`</option>`,
		`<option value="`+park.ID+`">Bibertpark</option>`,
		`name="`+core.EventFieldLocationID+`"`,
	)
	assertBodyLacks(t, rec, inlineFormAttribute, msgNoLocations)
	body := html.UnescapeString(rec.Body.String())
	if area, choice := strings.Index(body, `id="`+newLocationAreaID+`"`), strings.Index(body, `id="`+locationChoiceID+`"`); area < 0 || area > choice {
		t.Errorf("positions = %d, %d, want the area first and the choice out of band", area, choice)
	}
}

func TestSavingInvalidNewLocationShowsSameMessagesAndKeepsInput(t *testing.T) {
	ts := newTestServer(t)
	form := hallForm()
	form.Set(core.LocationFieldPostalCode, "9051")
	form.Set(core.LocationFieldLatitude, "")
	form.Set(core.LocationFieldLongitude, "10,95")

	rec := ts.htmxPost(inlineLocationPath, form)

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertFragment(t, rec)
	assertAllFieldsBelongToInlineForm(t, rec)
	assertBodyContains(t, rec,
		msgPostalCodeFormat, "Bitte eine Breite angeben.",
		`value="`+hallName+`"`, `value="9051"`, `value="10,95"`, `<option value="building" selected>Gebäude</option>`,
		"Eingang hinten", "Ort speichern",
	)
	assertBodyLacks(t, rec, locationChoiceID, "hx-swap-oob")
	if len(ts.locations.locations) != 0 {
		t.Error("an invalid location was stored")
	}
}

func TestSavingNewLocationWithExistingNameShowsConflictLinkInNewTab(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	form := hallForm()
	form.Set(core.LocationFieldName, " paul-metz-halle ")

	rec := ts.htmxPost(inlineLocationPath, form)

	assertStatusCode(t, rec, http.StatusConflict)
	assertFragment(t, rec)
	assertAllFieldsBelongToInlineForm(t, rec)
	assertBodyContains(t, rec,
		`<p class="error" role="alert">`+msgNameConflict+` <a href="`+locationPath(hall.ID)+`" target="_blank" rel="noopener">`+hallName+`</a></p>`,
		`value=" paul-metz-halle "`,
	)
	assertBodyLacks(t, rec, locationChoiceID, "hx-swap-oob")
	if len(ts.locations.locations) != 1 {
		t.Errorf("stored %d locations, want 1", len(ts.locations.locations))
	}
}

func TestUnexpectedNewLocationFieldProblemFallsBackToGenericMessage(t *testing.T) {
	ts := newTestServer(t)
	ts.handler.locations = failingLocations{err: &core.ValidationError{Fields: []core.FieldError{
		{Field: core.LocationFieldName, Problem: core.ProblemOutOfRange},
	}}}

	rec := ts.htmxPost(inlineLocationPath, hallForm())

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertBodyContains(t, rec, msgFieldInvalid)
}

func TestNewLocationRequestsWithoutSessionGetHXRedirect(t *testing.T) {
	ts := newTestServer(t)
	requests := []*http.Request{
		httptest.NewRequest(http.MethodGet, inlineLocationPath, nil),
		httptest.NewRequest(http.MethodGet, inlineLocationCancelPath, nil),
		httptest.NewRequest(http.MethodPost, inlineLocationPath, strings.NewReader(hallForm().Encode())),
	}
	for _, req := range requests {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set(htmxRequestHeader, htmxRequestTrue)

		rec := ts.do(req)

		assertStatusCode(t, rec, http.StatusUnauthorized)
		if got := rec.Header().Get(htmxRedirectHeader); got != loginPath {
			t.Errorf("%s %s: %s = %q, want %q", req.Method, req.URL.Path, htmxRedirectHeader, got, loginPath)
		}
	}
	if len(ts.locations.locations) != 0 {
		t.Error("a location was stored without session")
	}
}

func TestSavingNewLocationAnswersFailuresWithServerErrorAndLog(t *testing.T) {
	tests := map[string]func(*testServer) LocationUseCases{
		"save fails": func(*testServer) LocationUseCases { return failingLocations{err: errStorageDown} },
		"list fails after save": func(ts *testServer) LocationUseCases {
			return listFailingLocations{LocationUseCases: ts.handler.locations, err: errStorageDown}
		},
	}
	for name, locations := range tests {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t)
			ts.handler.locations = locations(ts)

			rec := ts.htmxPost(inlineLocationPath, hallForm())

			assertFailurePage(t, rec, http.StatusInternalServerError, locationFailurePage(msgRequestFailed))
			// The failure page is a whole page, which htmx does not swap in; it
			// holds no fragment of the event form.
			assertBodyLacks(t, rec, `id="`+locationChoiceID+`"`, `id="`+newLocationAreaID+`"`)
			if !strings.Contains(ts.logs.String(), logMsgLocationsFailed) || !strings.Contains(ts.logs.String(), errStorageDown.Error()) {
				t.Errorf("log %q does not record the failure", ts.logs.String())
			}
		})
	}
}

func TestSavingNewLocationPastTheDeadlineAnswersServiceUnavailable(t *testing.T) {
	ts := newTestServer(t)
	ts.handler.locations = failingLocations{err: errRequestTimedOut}

	rec := ts.htmxPost(inlineLocationPath, hallForm())

	assertFailurePage(t, rec, http.StatusServiceUnavailable, locationFailurePage(msgRequestTimedOut))
	assertLoggedAsWarningOnly(t, ts, logMsgLocationsFailed)
}

func TestOversizedNewLocationIsRejected(t *testing.T) {
	ts := newTestServer(t)
	form := hallForm()
	form.Set(core.LocationFieldNote, strings.Repeat("x", maxLocationFormBytes))

	rec := ts.htmxPost(inlineLocationPath, form)

	assertStatusCode(t, rec, http.StatusBadRequest)
	if len(ts.locations.locations) != 0 {
		t.Error("an oversized location was stored")
	}
}

func TestSavingEventWithExtraLocationFieldsCreatesNoLocation(t *testing.T) {
	ts := newTestServer(t)
	park := ts.seed(t, "Bibertpark")
	form := marketForm(park.ID)
	for field, values := range hallForm() {
		if form.Get(field) == "" {
			form[field] = values
		}
	}

	event := ts.seedEvent(t, form)

	if event.LocationID != park.ID || event.Note != "Mit Fahrgeschäften" {
		t.Errorf("stored = %+v, want the market at the park with its own note", event)
	}
	if len(ts.locations.locations) != 1 {
		t.Errorf("stored %d locations, want 1", len(ts.locations.locations))
	}
}

func TestFragmentRenderFailureAnswersInternalServerErrorAndLogs(t *testing.T) {
	ts := newTestServer(t)
	rec := httptest.NewRecorder()

	ts.handler.renderFragment(rec, newLocationTemplate, "missing", http.StatusOK, nil)

	assertStatusCode(t, rec, http.StatusInternalServerError)
	if !strings.Contains(ts.logs.String(), logMsgRenderFailed) {
		t.Errorf("log %q does not record the render failure", ts.logs.String())
	}
}

func TestSavingNewLocationOverTheLimitShowsItInline(t *testing.T) {
	ts := newTestServer(t)
	form := hallForm()
	form.Set(core.LocationFieldStreet, strings.Repeat("s", core.MaxStreetLength+1))
	form.Set(core.LocationFieldNote, strings.Repeat("ü", core.MaxLocationNoteLength))

	rec := ts.htmxPost(inlineLocationPath, form)

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertBodyContains(t, rec, "Höchstens 200 Zeichen.", form.Get(core.LocationFieldStreet), form.Get(core.LocationFieldNote))
}

// entryKeyPattern matches the key of a timetable entry in its element ids.
var entryKeyPattern = regexp.MustCompile(`id="entry-([A-Z2-7]{26})-location-choice"`)

// entryKeysIn returns the keys of the timetable entries in markup, in order.
func entryKeysIn(markup string) []string {
	var keys []string
	for _, match := range entryKeyPattern.FindAllStringSubmatch(markup, -1) {
		keys = append(keys, match[1])
	}
	return keys
}

// entryKeyOfNewEntry renders an added timetable entry and returns its key.
func (ts *testServer) entryKeyOfNewEntry(t *testing.T) string {
	t.Helper()
	keys := entryKeysIn(ts.htmxGet(timetableEntryPath).Body.String())
	if len(keys) != 1 {
		t.Fatalf("entry keys = %q, want one", keys)
	}
	return keys[0]
}

// entryIDs are the element ids of the location choice and the inline input
// of one timetable entry.
type entryIDs struct {
	area, form, choice, selectID string
}

func entryIDsOf(key string) entryIDs {
	entry := "entry-" + key
	return entryIDs{
		area:     entry + "-new-location",
		form:     entry + "-new-location-form",
		choice:   entry + "-location-choice",
		selectID: entry + "-locationId",
	}
}

func entryQuery(key string) string {
	return "?entry=" + key
}

// assertAllFieldsBelongTo checks that every field of a fragment is tied to
// the form with formID, so neither the event form nor another input sends it.
func assertAllFieldsBelongTo(t *testing.T, rec *httptest.ResponseRecorder, formID string) {
	t.Helper()
	tags := fieldTags.FindAllString(rec.Body.String(), -1)
	if len(tags) != len(hallForm()) {
		t.Errorf("fragment has %d fields, want %d", len(tags), len(hallForm()))
	}
	for _, tag := range tags {
		if !strings.Contains(tag, `form="`+formID+`"`) {
			t.Errorf("field %s is not tied to %s", tag, formID)
		}
	}
}

func TestEveryKeyFromRandTextIsAnEntryKey(t *testing.T) {
	for range 100 {
		if key := rand.Text(); !isEntryKey(key) {
			t.Fatalf("isEntryKey(%q) = false, want true", key)
		}
	}
}

func TestEventFormHasContainerForEntryLocationFormsAfterEventForm(t *testing.T) {
	ts := newTestServer(t)

	body := ts.get(newEventPath).Body.String()

	eventFormEnd := strings.Index(body, "</form>")
	if position := strings.Index(body, `<div id="entry-location-forms"></div>`); position < 0 || position < eventFormEnd {
		t.Errorf("container at %d, event form ends at %d, want an empty container after it", position, eventFormEnd)
	}
}

func TestEventLocationChoiceTakesNewLocations(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.get(newEventPath)

	assertBodyContains(t, rec, `<select id="`+core.EventFieldLocationID+`" name="`+core.EventFieldLocationID+`" required data-location-options>`)
}

func TestOpeningNewLocationAtEntryShowsFieldsOfThatEntryAndAppendsItsForm(t *testing.T) {
	ts := newTestServer(t)
	key := ts.entryKeyOfNewEntry(t)
	ids := entryIDsOf(key)

	rec := ts.htmxGet(inlineLocationPath + entryQuery(key))

	assertStatusCode(t, rec, http.StatusOK)
	assertFragment(t, rec)
	assertAllFieldsBelongTo(t, rec, ids.form)
	assertBodyContains(t, rec,
		`<div id="`+ids.area+`">`,
		`<label for="`+ids.area+`-name">Name</label>`,
		`<div id="`+ids.area+`-location-map" class="location-map" data-location-map`+
			` data-latitude-field="`+ids.area+`-latitude" data-longitude-field="`+ids.area+`-longitude"`,
		`<button type="submit" form="`+ids.form+`">Ort speichern</button>`,
		`<button type="button" hx-get="`+inlineLocationCancelPath+entryQuery(key)+`" hx-target="#`+ids.area+`" hx-swap="outerHTML">Abbrechen</button>`,
		`<div hx-swap-oob="beforeend:#entry-location-forms"><form id="`+ids.form+`" hx-post="`+inlineLocationPath+entryQuery(key)+
			`" hx-target="#`+ids.area+`" hx-swap="outerHTML" hx-sync="this:drop" novalidate></form></div>`,
	)
	assertBodyLacks(t, rec, `id="`+newLocationAreaID+`"`, inlineFormAttribute, `id="new-location-name"`)
}

func TestOpeningNewLocationAtEventAppendsNoForm(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.htmxGet(inlineLocationPath)

	assertBodyLacks(t, rec, "hx-swap-oob", "<form")
}

func TestCancellingNewLocationAtEntryClosesItAndRemovesItsForm(t *testing.T) {
	ts := newTestServer(t)
	key := ts.entryKeyOfNewEntry(t)
	ids := entryIDsOf(key)

	rec := ts.htmxGet(inlineLocationCancelPath + entryQuery(key))

	assertStatusCode(t, rec, http.StatusOK)
	assertFragment(t, rec)
	assertBodyContains(t, rec,
		`<div id="`+ids.area+`">`,
		`<a href="`+newLocationPath+`" hx-get="`+inlineLocationPath+entryQuery(key)+`" hx-target="#`+ids.area+`" hx-swap="outerHTML">Neuer Ort</a>`,
		`<div id="`+ids.form+`" hx-swap-oob="delete"></div>`,
	)
	assertBodyLacks(t, rec, `form="`, `role="status"`)
	if len(ts.locations.locations) != 0 {
		t.Error("cancelling stored a location")
	}
}

func TestSavingValidNewLocationAtEntrySelectsItThereAndOffersItEverywhereElse(t *testing.T) {
	ts := newTestServer(t)
	park := ts.seed(t, "Bibertpark")
	key := ts.entryKeyOfNewEntry(t)
	ids := entryIDsOf(key)

	rec := ts.htmxPost(inlineLocationPath+entryQuery(key), hallForm())

	assertStatusCode(t, rec, http.StatusOK)
	assertFragment(t, rec)
	hall, err := ts.locations.FindByNameKey(context.Background(), core.NormalizeKey(hallName))
	if err != nil {
		t.Fatalf("location not stored: %v", err)
	}
	assertBodyContains(t, rec,
		`<div id="`+ids.area+`">`,
		`<p class="hint" role="status">Ort „Paul-Metz-Halle“ angelegt und ausgewählt.</p>`,
		`hx-get="`+inlineLocationPath+entryQuery(key)+`"`,
		`<div id="`+ids.choice+`" hx-swap-oob="true">`,
		`<select id="`+ids.selectID+`" name="`+timetableFormLocationID+`" data-location-options>`,
		`<option value="">Ort des Events</option>`,
		`<option value="`+hall.ID+`" selected>`+hallName+`</option>`,
		`<option value="`+park.ID+`">Bibertpark</option>`,
		`<div hx-swap-oob="beforeend:select[data-location-options]:not(#`+ids.selectID+`)"><option value="`+hall.ID+`">`+hallName+`</option></div>`,
		`<div id="`+ids.form+`" hx-swap-oob="delete"></div>`,
	)
	assertBodyLacks(t, rec, `id="`+locationChoiceID+`"`, `id="`+newLocationAreaID+`"`, `form="`)
	body := html.UnescapeString(rec.Body.String())
	if area, choice := strings.Index(body, `id="`+ids.area+`"`), strings.Index(body, `id="`+ids.choice+`"`); area < 0 || area > choice {
		t.Errorf("positions = %d, %d, want the area first and the choice out of band", area, choice)
	}
}

func TestSavingValidNewLocationAtEventOffersItToTheOtherChoices(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.htmxPost(inlineLocationPath, hallForm())

	assertStatusCode(t, rec, http.StatusOK)
	hall, err := ts.locations.FindByNameKey(context.Background(), core.NormalizeKey(hallName))
	if err != nil {
		t.Fatalf("location not stored: %v", err)
	}
	assertBodyContains(t, rec,
		`<div hx-swap-oob="beforeend:select[data-location-options]:not(#`+core.EventFieldLocationID+`)"><option value="`+hall.ID+`">`+hallName+`</option></div>`,
	)
	assertBodyLacks(t, rec, `hx-swap-oob="delete"`)
}

func TestSavingInvalidNewLocationAtEntryShowsMessagesInThatEntry(t *testing.T) {
	ts := newTestServer(t)
	key := ts.entryKeyOfNewEntry(t)
	ids := entryIDsOf(key)
	form := hallForm()
	form.Set(core.LocationFieldPostalCode, "9051")

	rec := ts.htmxPost(inlineLocationPath+entryQuery(key), form)

	assertStatusCode(t, rec, http.StatusUnprocessableEntity)
	assertFragment(t, rec)
	assertAllFieldsBelongTo(t, rec, ids.form)
	assertBodyContains(t, rec, `<div id="`+ids.area+`">`, msgPostalCodeFormat, `value="9051"`, `id="`+ids.area+`-postalCode"`)
	// The form of the entry is already on the page and stays there.
	assertBodyLacks(t, rec, "hx-swap-oob", "<form")
	if len(ts.locations.locations) != 0 {
		t.Error("an invalid location was stored")
	}
}

func TestSavingNewLocationAtEntryWithExistingNameShowsConflictLinkInThatEntry(t *testing.T) {
	ts := newTestServer(t)
	hall := ts.seed(t, hallName)
	key := ts.entryKeyOfNewEntry(t)
	ids := entryIDsOf(key)

	rec := ts.htmxPost(inlineLocationPath+entryQuery(key), hallForm())

	assertStatusCode(t, rec, http.StatusConflict)
	assertAllFieldsBelongTo(t, rec, ids.form)
	assertBodyContains(t, rec,
		`<div id="`+ids.area+`">`,
		`<p class="error" role="alert">`+msgNameConflict+` <a href="`+locationPath(hall.ID)+`" target="_blank" rel="noopener">`+hallName+`</a></p>`,
	)
	assertBodyLacks(t, rec, "hx-swap-oob")
}

func TestNewLocationRequestsWithManipulatedEntryKeyAreBadRequests(t *testing.T) {
	valid := strings.Repeat("A", 26)
	queries := map[string]string{
		"empty":         "?entry=",
		"lower case":    "?entry=" + strings.ToLower(valid),
		"too short":     "?entry=" + valid[1:],
		"too long":      "?entry=" + valid + "A",
		"outside":       "?entry=" + valid[1:] + "1",
		"markup":        "?entry=" + url.QueryEscape(`"><b>`+valid[5:]),
		"twice":         "?entry=" + valid + "&entry=" + valid,
		"invalid query": "?entry=%zz",
	}
	for name, query := range queries {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t)
			responses := []*httptest.ResponseRecorder{
				ts.htmxGet(inlineLocationPath + query),
				ts.htmxGet(inlineLocationCancelPath + query),
				ts.htmxPost(inlineLocationPath+query, hallForm()),
			}
			for _, rec := range responses {
				assertStatusCode(t, rec, http.StatusBadRequest)
			}
			if len(ts.locations.locations) != 0 {
				t.Error("a location was stored for a manipulated entry key")
			}
		})
	}
}

func TestSavingNewLocationAtEntryAnswersFailuresWithServerErrorAndLog(t *testing.T) {
	tests := map[string]func(*testServer) LocationUseCases{
		"save fails": func(*testServer) LocationUseCases { return failingLocations{err: errStorageDown} },
		"list fails after save": func(ts *testServer) LocationUseCases {
			return listFailingLocations{LocationUseCases: ts.handler.locations, err: errStorageDown}
		},
	}
	for name, locations := range tests {
		t.Run(name, func(t *testing.T) {
			ts := newTestServer(t)
			key := ts.entryKeyOfNewEntry(t)
			ts.handler.locations = locations(ts)

			rec := ts.htmxPost(inlineLocationPath+entryQuery(key), hallForm())

			assertFailurePage(t, rec, http.StatusInternalServerError, locationFailurePage(msgRequestFailed))
			assertBodyLacks(t, rec, `id="`+entryIDsOf(key).choice+`"`, `id="`+entryIDsOf(key).area+`"`)
			if !strings.Contains(ts.logs.String(), logMsgLocationsFailed) {
				t.Errorf("log %q does not record the failure", ts.logs.String())
			}
		})
	}
}

func TestTwoOpenInputsUseTheirOwnForms(t *testing.T) {
	ts := newTestServer(t)
	key := ts.entryKeyOfNewEntry(t)

	atEvent := ts.htmxGet(inlineLocationPath)
	atEntry := ts.htmxGet(inlineLocationPath + entryQuery(key))

	assertAllFieldsBelongToInlineForm(t, atEvent)
	assertAllFieldsBelongTo(t, atEntry, entryIDsOf(key).form)
}

func TestSavingEventWithExtraLocationFieldsAtEntriesCreatesNoLocation(t *testing.T) {
	ts := newTestServer(t)
	park := ts.seed(t, "Bibertpark")
	form := festForm(park.ID, bieranstich, disco)
	for field, values := range hallForm() {
		if form.Get(field) == "" {
			form[field] = values
		}
	}

	event := ts.seedEvent(t, form)

	if got := storedDescriptions(event); !slices.Equal(got, []string{"Bieranstich", "Disco"}) {
		t.Errorf("timetable = %q, want both entries", got)
	}
	if len(ts.locations.locations) != 1 {
		t.Errorf("stored %d locations, want 1", len(ts.locations.locations))
	}
}
