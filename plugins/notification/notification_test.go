package notification_test

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

func TestNotificationCRUD(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	userTok := testutil.MakeToken("user1", "user")

	// Create competition
	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"title":"CompNotif","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	compID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	notifPath := fmt.Sprintf("/api/v1/admin/competitions/%s/notifications", compID)

	// --- Create ---
	resp = testutil.DoRequest(t, env.Server.URL, "POST", notifPath,
		`{"title":"Notice 1","message":"Message body"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	resp.Body.Close()

	// Create second notification
	resp = testutil.DoRequest(t, env.Server.URL, "POST", notifPath,
		`{"title":"Notice 2","message":"Second body"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	resp.Body.Close()

	// Create with missing fields → 400
	resp = testutil.DoRequest(t, env.Server.URL, "POST", notifPath,
		`{"title":"No message"}`, adminTok)
	testutil.AssertStatus(t, resp, 400)
	resp.Body.Close()

	// --- List ---
	resp = testutil.DoRequest(t, env.Server.URL, "GET",
		fmt.Sprintf("/api/v1/competitions/%s/notifications", compID), "", userTok)
	testutil.AssertStatus(t, resp, 200)
	body := testutil.DecodeJSON(t, resp)
	notifs := body["notifications"].([]any)
	if len(notifs) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(notifs))
	}
	// Verify order: most recent first (Notice 2 first)
	first := notifs[0].(map[string]any)
	if first["title"] != "Notice 2" {
		t.Fatalf("expected Notice 2 first, got %v", first["title"])
	}
	notifID := first["res_id"].(string)

	// --- Update ---
	resp = testutil.DoRequest(t, env.Server.URL, "PUT",
		fmt.Sprintf("/api/v1/admin/notifications/%s", notifID),
		`{"title":"Updated Title"}`, adminTok)
	testutil.AssertStatus(t, resp, 204)
	resp.Body.Close()

	// Verify update
	resp = testutil.DoRequest(t, env.Server.URL, "GET",
		fmt.Sprintf("/api/v1/competitions/%s/notifications", compID), "", userTok)
	testutil.AssertStatus(t, resp, 200)
	body = testutil.DecodeJSON(t, resp)
	notifs = body["notifications"].([]any)
	first = notifs[0].(map[string]any)
	if first["title"] != "Updated Title" {
		t.Fatalf("expected Updated Title, got %v", first["title"])
	}
	if first["message"] != "Second body" {
		t.Fatalf("expected message unchanged, got %v", first["message"])
	}

	// Update non-existent → 404
	resp = testutil.DoRequest(t, env.Server.URL, "PUT",
		"/api/v1/admin/notifications/00000000000000000000000000000000",
		`{"title":"X"}`, adminTok)
	testutil.AssertStatus(t, resp, 404)
	resp.Body.Close()

	// --- Delete ---
	resp = testutil.DoRequest(t, env.Server.URL, "DELETE",
		fmt.Sprintf("/api/v1/admin/notifications/%s", notifID), "", adminTok)
	testutil.AssertStatus(t, resp, 204)
	resp.Body.Close()

	// Verify deletion
	resp = testutil.DoRequest(t, env.Server.URL, "GET",
		fmt.Sprintf("/api/v1/competitions/%s/notifications", compID), "", userTok)
	testutil.AssertStatus(t, resp, 200)
	body = testutil.DecodeJSON(t, resp)
	notifs = body["notifications"].([]any)
	if len(notifs) != 1 {
		t.Fatalf("expected 1 notification after delete, got %d", len(notifs))
	}

	// Delete already deleted → 404
	resp = testutil.DoRequest(t, env.Server.URL, "DELETE",
		fmt.Sprintf("/api/v1/admin/notifications/%s", notifID), "", adminTok)
	testutil.AssertStatus(t, resp, 404)
	resp.Body.Close()

	// --- Auth checks ---
	// Non-admin create → 403
	resp = testutil.DoRequest(t, env.Server.URL, "POST", notifPath,
		`{"title":"X","message":"Y"}`, userTok)
	testutil.AssertStatus(t, resp, 403)
	resp.Body.Close()

	// No token → 401
	resp = testutil.DoRequest(t, env.Server.URL, "GET",
		fmt.Sprintf("/api/v1/competitions/%s/notifications", compID), "", "")
	testutil.AssertStatus(t, resp, 401)
	resp.Body.Close()
}
