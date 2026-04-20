package topthree_test

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"ad7/internal/testutil"
)

var env *testutil.TestEnv

func TestMain(m *testing.M) {
	env = testutil.NewTestEnv(m)
	defer env.Close()
	os.Exit(m.Run())
}

func getTopThree(t *testing.T, compID, token string) map[string]any {
	t.Helper()
	resp := testutil.DoRequest(t, env.Server.URL, "GET",
		fmt.Sprintf("/api/v1/topthree/competitions/%s", compID), "", token)
	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("topthree status %d: %s", resp.StatusCode, raw)
	}
	body := testutil.DecodeJSON(t, resp)
	if _, ok := body["challenges"]; !ok {
		t.Fatalf("no challenges key: %v", body)
	}
	return body
}

func TestTopThreeEventDriven(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	user1Tok := testutil.MakeToken("user1", "user")
	user2Tok := testutil.MakeToken("user2", "user")
	user3Tok := testutil.MakeToken("user3", "user")
	user4Tok := testutil.MakeToken("user4", "user")

	// Create competition
	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"title":"CompTT","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	compID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	// Create challenge
	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/challenges",
		`{"title":"TT1","description":"D","score":100,"flag":"flag{tt1}"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	chalID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	// Add challenge to competition
	testutil.DoRequest(t, env.Server.URL, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chalID), adminTok).Body.Close()

	// Empty topthree
	body := getTopThree(t, compID, user1Tok)
	challenges := body["challenges"].([]any)
	if len(challenges) != 1 {
		t.Fatalf("expected 1 challenge, got %d", len(challenges))
	}
	first := challenges[0].(map[string]any)
	ttRaw := first["top_three"]
	var tt []any
	if ttRaw != nil {
		tt = ttRaw.([]any)
	}
	if len(tt) != 0 {
		t.Fatalf("expected 0 top_three entries, got %d", len(tt))
	}

	// User1 solves → should be rank 1
	resp = testutil.DoRequest(t, env.Server.URL, "POST",
		fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID),
		`{"flag":"flag{tt1}"}`, user1Tok)
	testutil.AssertStatus(t, resp, 200)
	resp.Body.Close()
	time.Sleep(200 * time.Millisecond)

	body = getTopThree(t, compID, user1Tok)
	challenges = body["challenges"].([]any)
	tt = challenges[0].(map[string]any)["top_three"].([]any)
	if len(tt) != 1 {
		t.Fatalf("expected 1 top_three entry, got %d", len(tt))
	}
	if tt[0].(map[string]any)["user_id"] != "user1" {
		t.Fatalf("expected user1, got %v", tt[0].(map[string]any)["user_id"])
	}

	// User2 solves → rank 2
	testutil.DoRequest(t, env.Server.URL, "POST",
		fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID),
		`{"flag":"flag{tt1}"}`, user2Tok).Body.Close()
	time.Sleep(200 * time.Millisecond)

	// User3 solves → rank 3
	testutil.DoRequest(t, env.Server.URL, "POST",
		fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID),
		`{"flag":"flag{tt1}"}`, user3Tok).Body.Close()
	time.Sleep(200 * time.Millisecond)

	// Verify 3 entries
	body = getTopThree(t, compID, user1Tok)
	tt = body["challenges"].([]any)[0].(map[string]any)["top_three"].([]any)
	if len(tt) != 3 {
		t.Fatalf("expected 3 top_three entries, got %d", len(tt))
	}

	// User4 solves → should NOT enter top three
	testutil.DoRequest(t, env.Server.URL, "POST",
		fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID),
		`{"flag":"flag{tt1}"}`, user4Tok).Body.Close()
	time.Sleep(200 * time.Millisecond)

	body = getTopThree(t, compID, user1Tok)
	tt = body["challenges"].([]any)[0].(map[string]any)["top_three"].([]any)
	if len(tt) != 3 {
		t.Fatalf("expected still 3 entries after user4, got %d", len(tt))
	}
}

func TestTopThreeAuth(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")

	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"title":"CompAuth","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	compID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	// No token → 401
	resp = testutil.DoRequest(t, env.Server.URL, "GET",
		fmt.Sprintf("/api/v1/topthree/competitions/%s", compID), "", "")
	testutil.AssertStatus(t, resp, 401)
	resp.Body.Close()

	// Invalid competition ID → 400
	userTok := testutil.MakeToken("user1", "user")
	resp = testutil.DoRequest(t, env.Server.URL, "GET",
		"/api/v1/topthree/competitions/invalid", "", userTok)
	testutil.AssertStatus(t, resp, 400)
	resp.Body.Close()
}

func TestTopThreeDuplicateUser(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	user1Tok := testutil.MakeToken("user1", "user")

	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"title":"CompDup","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	compID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/challenges",
		`{"title":"DupChal","description":"D","score":100,"flag":"flag{dup}"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	chalID := testutil.GetID(t, testutil.DecodeJSON(t, resp))
	testutil.DoRequest(t, env.Server.URL, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chalID), adminTok).Body.Close()

	// Submit twice as same user
	testutil.DoRequest(t, env.Server.URL, "POST",
		fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID),
		`{"flag":"flag{dup}"}`, user1Tok).Body.Close()
	time.Sleep(200 * time.Millisecond)

	// Second submission should not create duplicate entry
	testutil.DoRequest(t, env.Server.URL, "POST",
		fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID),
		`{"flag":"flag{dup}"}`, user1Tok).Body.Close()
	time.Sleep(200 * time.Millisecond)

	body := getTopThree(t, compID, user1Tok)
	tt := body["challenges"].([]any)[0].(map[string]any)["top_three"].([]any)
	if len(tt) != 1 {
		t.Fatalf("expected 1 entry (no duplicate), got %d", len(tt))
	}
}
