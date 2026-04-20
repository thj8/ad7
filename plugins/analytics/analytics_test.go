package analytics_test

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

func TestAnalyticsOverview(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	userTok := testutil.MakeToken("user1", "user")

	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"title":"CompA","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	compID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	// Empty overview
	resp = testutil.DoRequest(t, env.Server.URL, "GET",
		fmt.Sprintf("/api/v1/competitions/%s/analytics/overview", compID), "", userTok)
	testutil.AssertStatus(t, resp, 200)
	body := testutil.DecodeJSON(t, resp)
	if body["total_users"] == nil {
		t.Fatal("expected total_users in overview")
	}

	// 401
	resp = testutil.DoRequest(t, env.Server.URL, "GET",
		fmt.Sprintf("/api/v1/competitions/%s/analytics/overview", compID), "", "")
	testutil.AssertStatus(t, resp, 401)
	resp.Body.Close()
}

func TestAnalyticsCategories(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	userTok := testutil.MakeToken("user1", "user")

	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"title":"CompCat","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	compID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	resp = testutil.DoRequest(t, env.Server.URL, "GET",
		fmt.Sprintf("/api/v1/competitions/%s/analytics/categories", compID), "", userTok)
	testutil.AssertStatus(t, resp, 200)
	body := testutil.DecodeJSON(t, resp)
	if body["categories"] == nil {
		t.Fatal("expected categories key")
	}
}

func TestAnalyticsUsers(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	userTok := testutil.MakeToken("user1", "user")

	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"title":"CompUser","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	compID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	// Create challenge and add to comp
	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/challenges",
		`{"title":"ChalUser","description":"D","score":100,"flag":"flag{user}"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	chalID := testutil.GetID(t, testutil.DecodeJSON(t, resp))
	testutil.DoRequest(t, env.Server.URL, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chalID), adminTok).Body.Close()

	// Submit correct flag
	testutil.DoRequest(t, env.Server.URL, "POST",
		fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID),
		`{"flag":"flag{user}"}`, userTok).Body.Close()

	// Check users analytics
	resp = testutil.DoRequest(t, env.Server.URL, "GET",
		fmt.Sprintf("/api/v1/competitions/%s/analytics/users", compID), "", userTok)
	testutil.AssertStatus(t, resp, 200)
	body := testutil.DecodeJSON(t, resp)
	users := body["users"].([]any)
	if len(users) == 0 {
		t.Fatal("expected at least 1 user in analytics")
	}
}

func TestAnalyticsChallenges(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	userTok := testutil.MakeToken("user1", "user")

	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"title":"CompChal","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	compID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	// Create challenge and add to comp
	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/challenges",
		`{"title":"ChalAna","description":"D","score":100,"flag":"flag{chal}"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	chalID := testutil.GetID(t, testutil.DecodeJSON(t, resp))
	testutil.DoRequest(t, env.Server.URL, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chalID), adminTok).Body.Close()

	// Submit correct flag
	testutil.DoRequest(t, env.Server.URL, "POST",
		fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID),
		`{"flag":"flag{chal}"}`, userTok).Body.Close()

	// Check challenges analytics
	resp = testutil.DoRequest(t, env.Server.URL, "GET",
		fmt.Sprintf("/api/v1/competitions/%s/analytics/challenges", compID), "", userTok)
	testutil.AssertStatus(t, resp, 200)
	body := testutil.DecodeJSON(t, resp)
	chals := body["challenges"].([]any)
	if len(chals) == 0 {
		t.Fatal("expected at least 1 challenge in analytics")
	}
}
