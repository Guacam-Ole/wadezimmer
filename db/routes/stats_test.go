package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

// The stats queries are raw SQL, so a renamed column only fails at runtime.
// Run them against the schema built from the real migrations.
func TestStatsRunsAgainstRealSchema(t *testing.T) {
	app := newRemoteRealSchemaApp(t)

	actors, err := app.FindCollectionByNameOrId("activitypub_actors")
	if err != nil {
		t.Fatal(err)
	}
	for i, local := range []bool{true, true, false} {
		actor := core.NewRecord(actors)
		actor.Set("iri", "https://example.org/actor/"+string(rune('a'+i)))
		actor.Set("preferred_username", "user"+string(rune('a'+i)))
		actor.Set("is_local", local)
		if err := app.SaveNoValidate(actor); err != nil {
			t.Fatal(err)
		}
	}

	event := &core.RequestEvent{App: app}
	event.Request = httptest.NewRequest(http.MethodGet, "/stats", nil)
	recorder := httptest.NewRecorder()
	event.Response = recorder

	if err := Stats(event); err != nil {
		t.Fatalf("Stats() error = %v", err)
	}

	var body struct {
		Totals statsTotals `json:"totals"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Totals.TotalUsers != 2 {
		t.Fatalf("totalUsers = %d, want 2 (local actors only)", body.Totals.TotalUsers)
	}
}
