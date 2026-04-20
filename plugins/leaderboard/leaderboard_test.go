package leaderboard_test

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

func TestCompetitionLeaderboard(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	user1Tok := testutil.MakeToken("user1", "user")
	user2Tok := testutil.MakeToken("user2", "user")

	// Create competition
	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"title":"CompLB","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	compID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	// Create 2 challenges
	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/challenges",
		`{"title":"LB1","description":"D","score":100,"flag":"flag{lb1}"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	chal1ID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/challenges",
		`{"title":"LB2","description":"D","score":200,"flag":"flag{lb2}"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	chal2ID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	// Add both challenges to competition
	testutil.DoRequest(t, env.Server.URL, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chal1ID), adminTok).Body.Close()
	testutil.DoRequest(t, env.Server.URL, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chal2ID), adminTok).Body.Close()

	// Empty leaderboard
	resp = testutil.DoRequest(t, env.Server.URL, "GET", fmt.Sprintf("/api/v1/competitions/%s/leaderboard", compID), "", user1Tok)
	testutil.AssertStatus(t, resp, 200)
	body := testutil.DecodeJSON(t, resp)
	if body["leaderboard"] == nil {
		t.Fatal("expected leaderboard key")
	}

	// User1 solves chal1
	testutil.DoRequest(t, env.Server.URL, "POST",
		fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chal1ID),
		`{"flag":"flag{lb1}"}`, user1Tok).Body.Close()

	// User2 solves both
	testutil.DoRequest(t, env.Server.URL, "POST",
		fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chal1ID),
		`{"flag":"flag{lb1}"}`, user2Tok).Body.Close()
	testutil.DoRequest(t, env.Server.URL, "POST",
		fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chal2ID),
		`{"flag":"flag{lb2}"}`, user2Tok).Body.Close()

	// Check leaderboard - user2 should be rank 1 (300pts), user1 rank 2 (100pts)
	resp = testutil.DoRequest(t, env.Server.URL, "GET", fmt.Sprintf("/api/v1/competitions/%s/leaderboard", compID), "", user1Tok)
	testutil.AssertStatus(t, resp, 200)
	body = testutil.DecodeJSON(t, resp)
	lb := body["leaderboard"].([]any)
	if len(lb) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(lb))
	}
	first := lb[0].(map[string]any)
	if first["user_id"] != "user2" {
		t.Fatalf("expected user2 as rank 1, got %v", first["user_id"])
	}

	// 401 no token
	resp = testutil.DoRequest(t, env.Server.URL, "GET", fmt.Sprintf("/api/v1/competitions/%s/leaderboard", compID), "", "")
	testutil.AssertStatus(t, resp, 401)
	resp.Body.Close()

	// Invalid competition ID
	resp = testutil.DoRequest(t, env.Server.URL, "GET", "/api/v1/competitions/invalid/leaderboard", "", user1Tok)
	testutil.AssertStatus(t, resp, 400)
	resp.Body.Close()
}
