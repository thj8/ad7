package hints_test

import (
	"fmt"
	"os"
	"testing"

	"ad7/internal/testutil"
)

var env *testutil.TestEnv

func TestMain(m *testing.M) {
	env = testutil.NewTestEnv(m)
	defer env.Close()
	os.Exit(m.Run())
}

func TestHintsCRUD(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	userTok := testutil.MakeToken("user1", "user")

	// Create a challenge first
	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/challenges",
		`{"title":"HintChal","description":"D","score":100,"flag":"flag{h}"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	chalID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	hintsCreatePath := fmt.Sprintf("/api/v1/admin/challenges/%s/hints", chalID)

	// --- Create hint ---
	resp = testutil.DoRequest(t, env.Server.URL, "POST", hintsCreatePath,
		`{"content":"This is a hint"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	resp.Body.Close()

	// Create second hint
	resp = testutil.DoRequest(t, env.Server.URL, "POST", hintsCreatePath,
		`{"content":"Secret hint"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	resp.Body.Close()

	// Create with empty content → 400
	resp = testutil.DoRequest(t, env.Server.URL, "POST", hintsCreatePath,
		`{"content":""}`, adminTok)
	testutil.AssertStatus(t, resp, 400)
	resp.Body.Close()

	// --- List hints ---
	resp = testutil.DoRequest(t, env.Server.URL, "GET",
		fmt.Sprintf("/api/v1/challenges/%s/hints", chalID), "", userTok)
	testutil.AssertStatus(t, resp, 200)
	body := testutil.DecodeJSON(t, resp)
	hints := body["hints"].([]any)
	if len(hints) != 2 {
		t.Fatalf("expected 2 hints, got %d", len(hints))
	}
	first := hints[0].(map[string]any)
	hintID := first["res_id"].(string)

	// --- Update hint: make invisible ---
	resp = testutil.DoRequest(t, env.Server.URL, "PUT",
		fmt.Sprintf("/api/v1/admin/hints/%s", hintID),
		`{"is_visible":false}`, adminTok)
	testutil.AssertStatus(t, resp, 204)
	resp.Body.Close()

	// Verify: should only see 1 hint now
	resp = testutil.DoRequest(t, env.Server.URL, "GET",
		fmt.Sprintf("/api/v1/challenges/%s/hints", chalID), "", userTok)
	testutil.AssertStatus(t, resp, 200)
	body = testutil.DecodeJSON(t, resp)
	hints = body["hints"].([]any)
	if len(hints) != 1 {
		t.Fatalf("expected 1 visible hint, got %d", len(hints))
	}

	// Update non-existent → 404
	resp = testutil.DoRequest(t, env.Server.URL, "PUT",
		"/api/v1/admin/hints/00000000000000000000000000000000",
		`{"content":"x"}`, adminTok)
	testutil.AssertStatus(t, resp, 404)
	resp.Body.Close()

	// --- Delete hint ---
	resp = testutil.DoRequest(t, env.Server.URL, "DELETE",
		fmt.Sprintf("/api/v1/admin/hints/%s", hintID), "", adminTok)
	testutil.AssertStatus(t, resp, 204)
	resp.Body.Close()

	// Delete non-existent → 404
	resp = testutil.DoRequest(t, env.Server.URL, "DELETE",
		"/api/v1/admin/hints/00000000000000000000000000000000", "", adminTok)
	testutil.AssertStatus(t, resp, 404)
	resp.Body.Close()

	// --- Auth checks ---
	// Non-admin create → 403
	resp = testutil.DoRequest(t, env.Server.URL, "POST", hintsCreatePath,
		`{"content":"hack"}`, userTok)
	testutil.AssertStatus(t, resp, 403)
	resp.Body.Close()

	// No token → 401
	resp = testutil.DoRequest(t, env.Server.URL, "GET",
		fmt.Sprintf("/api/v1/challenges/%s/hints", chalID), "", "")
	testutil.AssertStatus(t, resp, 401)
	resp.Body.Close()
}
