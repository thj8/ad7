package integration_test

import (
	"fmt"
	"net/http"
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

// --- Tests ---

func TestListChallenges(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")

	// 200 empty list
	resp := testutil.DoRequest(t, env.Server.URL, "GET", "/api/v1/challenges", "", adminTok)
	testutil.AssertStatus(t, resp, 200)
	body := testutil.DecodeJSON(t, resp)
	if body["challenges"] == nil {
		t.Fatal("expected challenges key")
	}

	// create a challenge and verify flag is not in list response
	testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/challenges",
		`{"title":"FlagTest","description":"D","score":100,"flag":"flag{secret}"}`, adminTok).Body.Close()
	resp = testutil.DoRequest(t, env.Server.URL, "GET", "/api/v1/challenges", "", adminTok)
	testutil.AssertStatus(t, resp, 200)
	body = testutil.DecodeJSON(t, resp)
	for _, c := range body["challenges"].([]any) {
		ch := c.(map[string]any)
		if _, hasFlag := ch["flag"]; hasFlag {
			t.Fatal("flag must not appear in challenge list response")
		}
	}

	// 401 no token
	resp = testutil.DoRequest(t, env.Server.URL, "GET", "/api/v1/challenges", "", "")
	testutil.AssertStatus(t, resp, 401)
	resp.Body.Close()
}

func TestGetChallenge(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	userTok := testutil.MakeToken("user1", "user")

	// create a challenge first
	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/challenges",
		`{"title":"T1","description":"D","score":100,"flag":"flag{x}"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	b := testutil.DecodeJSON(t, resp)
	id := testutil.GetID(t, b)

	// 200 found
	resp = testutil.DoRequest(t, env.Server.URL, "GET", fmt.Sprintf("/api/v1/challenges/%s", id), "", userTok)
	testutil.AssertStatus(t, resp, 200)
	b = testutil.DecodeJSON(t, resp)
	if b["title"] != "T1" {
		t.Fatalf("expected title T1, got %v", b["title"])
	}
	if _, hasFlag := b["flag"]; hasFlag {
		t.Fatal("flag must not be in response")
	}

	// 404 not found
	resp = testutil.DoRequest(t, env.Server.URL, "GET", "/api/v1/challenges/00000000000000000000000000000000", "", userTok)
	testutil.AssertStatus(t, resp, 404)
	resp.Body.Close()

	// 401
	resp = testutil.DoRequest(t, env.Server.URL, "GET", fmt.Sprintf("/api/v1/challenges/%s", id), "", "")
	testutil.AssertStatus(t, resp, 401)
	resp.Body.Close()
}

func TestAdminCreateChallenge(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	userTok := testutil.MakeToken("user1", "user")
	body := `{"title":"T3","description":"D","score":200,"flag":"flag{z}"}`

	// 201 happy path
	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/challenges", body, adminTok)
	testutil.AssertStatus(t, resp, 201)
	b := testutil.DecodeJSON(t, resp)
	if b["id"] == nil {
		t.Fatal("expected id in response")
	}

	// 403 non-admin
	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/challenges", body, userTok)
	testutil.AssertStatus(t, resp, 403)
	resp.Body.Close()

	// 401
	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/challenges", body, "")
	testutil.AssertStatus(t, resp, 401)
	resp.Body.Close()
}

func TestAdminUpdateChallenge(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	userTok := testutil.MakeToken("user1", "user")

	// create
	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/challenges",
		`{"title":"T4","description":"D","score":100,"flag":"flag{u}"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	b := testutil.DecodeJSON(t, resp)
	id := testutil.GetID(t, b)
	path := fmt.Sprintf("/api/v1/admin/challenges/%s", id)

	// 204 update
	resp = testutil.DoRequest(t, env.Server.URL, "PUT", path,
		`{"title":"T4-updated","description":"D","score":150,"flag":"flag{u}","is_enabled":true}`, adminTok)
	testutil.AssertStatus(t, resp, 204)
	resp.Body.Close()

	// 404
	resp = testutil.DoRequest(t, env.Server.URL, "PUT", "/api/v1/admin/challenges/00000000000000000000000000000000",
		`{"title":"x","description":"D","score":100,"flag":"flag{x}","is_enabled":true}`, adminTok)
	testutil.AssertStatus(t, resp, 404)
	resp.Body.Close()

	// 403
	resp = testutil.DoRequest(t, env.Server.URL, "PUT", path,
		`{"title":"x","description":"D","score":100,"flag":"flag{x}","is_enabled":true}`, userTok)
	testutil.AssertStatus(t, resp, 403)
	resp.Body.Close()

	// 401
	resp = testutil.DoRequest(t, env.Server.URL, "PUT", path,
		`{"title":"x","description":"D","score":100,"flag":"flag{x}","is_enabled":true}`, "")
	testutil.AssertStatus(t, resp, 401)
	resp.Body.Close()
}

func TestAdminDeleteChallenge(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	userTok := testutil.MakeToken("user1", "user")

	// create
	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/challenges",
		`{"title":"T5","description":"D","score":100,"flag":"flag{d}"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	b := testutil.DecodeJSON(t, resp)
	id := testutil.GetID(t, b)
	path := fmt.Sprintf("/api/v1/admin/challenges/%s", id)

	// 403
	resp = testutil.DoRequest(t, env.Server.URL, "DELETE", path, "", userTok)
	testutil.AssertStatus(t, resp, 403)
	resp.Body.Close()

	// 401
	resp = testutil.DoRequest(t, env.Server.URL, "DELETE", path, "", "")
	testutil.AssertStatus(t, resp, 401)
	resp.Body.Close()

	// 204 delete
	resp = testutil.DoRequest(t, env.Server.URL, "DELETE", path, "", adminTok)
	testutil.AssertStatus(t, resp, 204)
	resp.Body.Close()

	// verify soft-deleted challenge is no longer accessible
	resp = testutil.DoRequest(t, env.Server.URL, "GET", fmt.Sprintf("/api/v1/challenges/%s", id), "", userTok)
	testutil.AssertStatus(t, resp, 404)
	resp.Body.Close()
}

func TestAdminListSubmissions(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	userTok := testutil.MakeToken("user1", "user")

	// create competition + challenge + submit in comp
	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"title":"CompSub","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	compID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/challenges",
		`{"title":"T6","description":"D","score":100,"flag":"flag{s}"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	chalID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	// add challenge to competition
	testutil.DoRequest(t, env.Server.URL, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chalID), adminTok).Body.Close()

	// submit in competition
	testutil.DoRequest(t, env.Server.URL, "POST", fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID),
		`{"flag":"flag{s}"}`, userTok)

	// 200 unfiltered
	resp = testutil.DoRequest(t, env.Server.URL, "GET", fmt.Sprintf("/api/v1/admin/competitions/%s/submissions", compID), "", adminTok)
	testutil.AssertStatus(t, resp, 200)
	b := testutil.DecodeJSON(t, resp)
	subs := b["submissions"].([]any)
	if len(subs) == 0 {
		t.Fatal("expected at least 1 submission")
	}

	// 200 filtered by user_id
	resp = testutil.DoRequest(t, env.Server.URL, "GET", fmt.Sprintf("/api/v1/admin/competitions/%s/submissions?user_id=user1", compID), "", adminTok)
	testutil.AssertStatus(t, resp, 200)
	resp.Body.Close()

	// 200 filtered by challenge_id
	resp = testutil.DoRequest(t, env.Server.URL, "GET", fmt.Sprintf("/api/v1/admin/competitions/%s/submissions?challenge_id=%s", compID, chalID), "", adminTok)
	testutil.AssertStatus(t, resp, 200)
	resp.Body.Close()

	// 403
	resp = testutil.DoRequest(t, env.Server.URL, "GET", fmt.Sprintf("/api/v1/admin/competitions/%s/submissions", compID), "", userTok)
	testutil.AssertStatus(t, resp, 403)
	resp.Body.Close()

	// 401
	resp = testutil.DoRequest(t, env.Server.URL, "GET", fmt.Sprintf("/api/v1/admin/competitions/%s/submissions", compID), "", "")
	testutil.AssertStatus(t, resp, 401)
	resp.Body.Close()
}

func TestCompetitions(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	userTok := testutil.MakeToken("user1", "user")

	// Create competition
	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"title":"CTF Round 1","description":"First round","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	b := testutil.DecodeJSON(t, resp)
	compID := testutil.GetID(t, b)

	// List active competitions
	resp = testutil.DoRequest(t, env.Server.URL, "GET", "/api/v1/competitions", "", userTok)
	testutil.AssertStatus(t, resp, 200)
	b = testutil.DecodeJSON(t, resp)
	comps := b["competitions"].([]any)
	if len(comps) == 0 {
		t.Fatal("expected at least 1 competition")
	}

	// Get competition detail
	resp = testutil.DoRequest(t, env.Server.URL, "GET", fmt.Sprintf("/api/v1/competitions/%s", compID), "", userTok)
	testutil.AssertStatus(t, resp, 200)
	b = testutil.DecodeJSON(t, resp)
	if b["title"] != "CTF Round 1" {
		t.Fatalf("expected title 'CTF Round 1', got %v", b["title"])
	}

	// Update competition
	resp = testutil.DoRequest(t, env.Server.URL, "PUT", fmt.Sprintf("/api/v1/admin/competitions/%s", compID),
		`{"title":"CTF Round 1 Updated","description":"Updated","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z","is_active":true}`, adminTok)
	testutil.AssertStatus(t, resp, 204)
	resp.Body.Close()

	// Delete competition
	resp = testutil.DoRequest(t, env.Server.URL, "DELETE", fmt.Sprintf("/api/v1/admin/competitions/%s", compID), "", adminTok)
	testutil.AssertStatus(t, resp, 204)
	resp.Body.Close()

	// 404 after delete
	resp = testutil.DoRequest(t, env.Server.URL, "GET", fmt.Sprintf("/api/v1/competitions/%s", compID), "", userTok)
	testutil.AssertStatus(t, resp, 404)
	resp.Body.Close()

	// 403 non-admin create
	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"title":"X","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, userTok)
	testutil.AssertStatus(t, resp, 403)
	resp.Body.Close()

	// 400 missing title
	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	testutil.AssertStatus(t, resp, 400)
	resp.Body.Close()
}

func TestCompetitionChallenges(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	userTok := testutil.MakeToken("user1", "user")

	// Create competition
	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"title":"Comp","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	compID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	// Create challenge
	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/challenges",
		`{"title":"Chal1","description":"D","score":100,"flag":"flag{x}"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	chalID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	// Add challenge to competition
	resp = testutil.DoRequest(t, env.Server.URL, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chalID), adminTok)
	testutil.AssertStatus(t, resp, 201)
	resp.Body.Close()

	// List competition challenges
	resp = testutil.DoRequest(t, env.Server.URL, "GET", fmt.Sprintf("/api/v1/competitions/%s/challenges", compID), "", userTok)
	testutil.AssertStatus(t, resp, 200)
	b := testutil.DecodeJSON(t, resp)
	chals := b["challenges"].([]any)
	if len(chals) != 1 {
		t.Fatalf("expected 1 challenge, got %d", len(chals))
	}
	// verify flag not leaked in competition challenge list
	for _, c := range chals {
		ch := c.(map[string]any)
		if _, hasFlag := ch["flag"]; hasFlag {
			t.Fatal("flag must not appear in competition challenge list response")
		}
	}

	// Remove challenge from competition
	resp = testutil.DoRequest(t, env.Server.URL, "DELETE", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges/%s", compID, chalID), "", adminTok)
	testutil.AssertStatus(t, resp, 204)
	resp.Body.Close()

	// Verify empty
	resp = testutil.DoRequest(t, env.Server.URL, "GET", fmt.Sprintf("/api/v1/competitions/%s/challenges", compID), "", userTok)
	testutil.AssertStatus(t, resp, 200)
	b = testutil.DecodeJSON(t, resp)
	chals = b["challenges"].([]any)
	if len(chals) != 0 {
		t.Fatalf("expected 0 challenges, got %d", len(chals))
	}
}

func TestSubmitInCompetition(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	userTok := testutil.MakeToken("user1", "user")

	// Create competition + challenge + add to comp
	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"title":"Comp","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	compID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/challenges",
		`{"title":"Chal","description":"D","score":200,"flag":"flag{comp}"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	chalID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	resp = testutil.DoRequest(t, env.Server.URL, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chalID), adminTok)
	testutil.AssertStatus(t, resp, 201)
	resp.Body.Close()

	submitPath := fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID)

	// Correct flag
	resp = testutil.DoRequest(t, env.Server.URL, "POST", submitPath, `{"flag":"flag{comp}"}`, userTok)
	testutil.AssertStatus(t, resp, 200)
	b := testutil.DecodeJSON(t, resp)
	if b["success"] != true {
		t.Fatalf("expected success=true, got %v", b["success"])
	}

	// Already solved
	resp = testutil.DoRequest(t, env.Server.URL, "POST", submitPath, `{"flag":"flag{comp}"}`, userTok)
	testutil.AssertStatus(t, resp, 200)
	b = testutil.DecodeJSON(t, resp)
	if b["message"] != "already_solved" {
		t.Fatalf("expected already_solved, got %v", b["message"])
	}

	// Wrong flag with different user
	user2Tok := testutil.MakeToken("user2", "user")
	resp = testutil.DoRequest(t, env.Server.URL, "POST", submitPath, `{"flag":"wrong"}`, user2Tok)
	testutil.AssertStatus(t, resp, 200)
	b = testutil.DecodeJSON(t, resp)
	if b["success"] != false {
		t.Fatal("expected success=false for wrong flag")
	}

	// 401 no token
	resp = testutil.DoRequest(t, env.Server.URL, "POST", submitPath, `{"flag":"x"}`, "")
	testutil.AssertStatus(t, resp, 401)
	resp.Body.Close()
}

func TestSubmitFlagRateLimit(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	userTok := testutil.MakeToken("rateuser1", "user")

	// Create competition + challenge + add to comp
	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"title":"CompRate","description":"D","start_time":"2026-01-01T00:00:00Z","end_time":"2026-12-31T23:59:59Z"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	compID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/challenges",
		`{"title":"ChalRate","description":"D","score":200,"flag":"flag{rate}"}`, adminTok)
	testutil.AssertStatus(t, resp, 201)
	chalID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	resp = testutil.DoRequest(t, env.Server.URL, "POST", fmt.Sprintf("/api/v1/admin/competitions/%s/challenges", compID),
		fmt.Sprintf(`{"challenge_id":"%s"}`, chalID), adminTok)
	testutil.AssertStatus(t, resp, 201)
	resp.Body.Close()

	submitPath := fmt.Sprintf("/api/v1/competitions/%s/challenges/%s/submit", compID, chalID)

	// First 3 requests should succeed (rate limit is 3 per 10s)
	for i := 0; i < 3; i++ {
		resp = testutil.DoRequest(t, env.Server.URL, "POST", submitPath, `{"flag":"wrong"}`, userTok)
		testutil.AssertStatus(t, resp, 200)
		resp.Body.Close()
	}

	// 4th request should be rate limited (429)
	resp = testutil.DoRequest(t, env.Server.URL, "POST", submitPath, `{"flag":"wrong"}`, userTok)
	testutil.AssertStatus(t, resp, http.StatusTooManyRequests)
	resp.Body.Close()

	// Different user should still be able to submit
	user2Tok := testutil.MakeToken("rateuser2", "user")
	resp = testutil.DoRequest(t, env.Server.URL, "POST", submitPath, `{"flag":"wrong"}`, user2Tok)
	testutil.AssertStatus(t, resp, 200)
	resp.Body.Close()
}

func TestCompetitionStartEnd(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")

	// 创建比赛（默认 is_active=true）
	startTime := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	endTime := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"title":"Test Comp","description":"desc","start_time":"`+startTime+`","end_time":"`+endTime+`"}`,
		adminTok)
	testutil.AssertStatus(t, resp, http.StatusCreated)
	compID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	// 1. 手动结束比赛
	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions/"+compID+"/end", "", adminTok)
	testutil.AssertStatus(t, resp, http.StatusOK)
	body := testutil.DecodeJSON(t, resp)
	if body["is_active"] != false {
		t.Fatalf("expected is_active=false after end, got %v", body["is_active"])
	}

	// 2. 重复结束 → 409
	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions/"+compID+"/end", "", adminTok)
	testutil.AssertStatus(t, resp, http.StatusConflict)

	// 3. 手动开始比赛
	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions/"+compID+"/start", "", adminTok)
	testutil.AssertStatus(t, resp, http.StatusOK)
	body = testutil.DecodeJSON(t, resp)
	if body["is_active"] != true {
		t.Fatalf("expected is_active=true after start, got %v", body["is_active"])
	}

	// 4. 重复开始 → 409
	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions/"+compID+"/start", "", adminTok)
	testutil.AssertStatus(t, resp, http.StatusConflict)

	// 5. 不存在的比赛 → 404
	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions/00000000000000000000000000000000/start", "", adminTok)
	testutil.AssertStatus(t, resp, http.StatusNotFound)

	// 6. 非 admin → 403
	userTok := testutil.MakeToken("user1", "user")
	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions/"+compID+"/start", "", userTok)
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestCompetitionAutoStatus(t *testing.T) {
	testutil.Cleanup(t, env.DB)
	adminTok := testutil.MakeToken("admin1", "admin")
	userTok := testutil.MakeToken("user1", "user")

	// --- 自动激活测试 ---
	// 创建一个 start_time 在过去、end_time 在未来的比赛
	start1 := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	end1 := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	resp := testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"title":"Auto Activate","description":"","start_time":"`+start1+`","end_time":"`+end1+`"}`,
		adminTok)
	testutil.AssertStatus(t, resp, http.StatusCreated)
	comp1ID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	// 手动结束比赛使其 is_active=false
	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions/"+comp1ID+"/end", "", adminTok)
	testutil.AssertStatus(t, resp, http.StatusOK)

	// 通过 Get 触发 syncStatus，应自动激活
	resp = testutil.DoRequest(t, env.Server.URL, "GET", "/api/v1/competitions/"+comp1ID, "", userTok)
	testutil.AssertStatus(t, resp, http.StatusOK)
	body := testutil.DecodeJSON(t, resp)
	if body["is_active"] != true {
		t.Fatalf("expected auto-activation (is_active=true), got %v", body["is_active"])
	}

	// --- 自动结束测试 ---
	// 创建一个 end_time 在过去的比赛
	start2 := time.Now().Add(-3 * time.Hour).Format(time.RFC3339)
	end2 := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	resp = testutil.DoRequest(t, env.Server.URL, "POST", "/api/v1/admin/competitions",
		`{"title":"Auto End","description":"","start_time":"`+start2+`","end_time":"`+end2+`"}`,
		adminTok)
	testutil.AssertStatus(t, resp, http.StatusCreated)
	comp2ID := testutil.GetID(t, testutil.DecodeJSON(t, resp))

	// 新建比赛默认 is_active=true，但 end_time 已过
	// 通过 Get 触发 syncStatus，应自动结束
	resp = testutil.DoRequest(t, env.Server.URL, "GET", "/api/v1/competitions/"+comp2ID, "", userTok)
	testutil.AssertStatus(t, resp, http.StatusOK)
	body = testutil.DecodeJSON(t, resp)
	if body["is_active"] != false {
		t.Fatalf("expected auto-ending (is_active=false), got %v", body["is_active"])
	}

	// --- ListActive 过滤测试 ---
	// comp2 已自动结束，不应出现在 ListActive 中
	resp = testutil.DoRequest(t, env.Server.URL, "GET", "/api/v1/competitions", "", userTok)
	testutil.AssertStatus(t, resp, http.StatusOK)
	listBody := testutil.DecodeJSON(t, resp)
	comps := listBody["competitions"].([]any)
	for _, c := range comps {
		m := c.(map[string]any)
		if m["id"] == comp2ID {
			t.Fatal("auto-ended competition should not appear in ListActive")
		}
	}
}
