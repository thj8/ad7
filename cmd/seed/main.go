// Package main 是测试数据填充工具的入口。
// 通过 HTTP 调用 CTF 和 Auth 服务器创建测试数据，
// 使提交记录走完整链路（service → event → topthree），
// 从而可以验证一二三血功能是否正常工作。
package main

import (
	"bytes"
	crypto_rand "crypto/rand"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	_ "github.com/go-sql-driver/mysql"
)

// regMu 保证注册串行执行，避免并发触发 auth 服务限流（10 次/分钟/IP）。
var regMu sync.Mutex

// getDB 从 TEST_DSN 环境变量获取数据库连接。
func getDB() *sql.DB {
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		log.Fatal("TEST_DSN env var required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	return db
}

const (
	numComps     = 2  // 生成的比赛数量
	poolSize     = 50 // 题目池大小
	chalsPerComp = 25 // 每个比赛分配的题目数量
	usersPerComp = 30 // 每个比赛的模拟用户数量
	teamsPerComp = 5  // 每个比赛的队伍数量（队伍模式）

	// submitDelay 是每个用户两次提交之间的间隔。
	// 提交端点限流为每用户 3 次/10 秒，4 秒间隔确保不触发限流。
	submitDelay = 4 * time.Second
)

// solveCounts 定义每个用户的正确解题数量。
// solveCounts[i] 表示第 i 个用户（0=最强）的正确解题数。
// User 0 → 18/25 = 72%；User 29 → 1/25 = 4%。
// 低排名处故意制造并列，模拟真实的排行榜动态。
var solveCounts = [30]int{
	18, 16, 15, 14, 13, 12, 11, 10, 9, 8,
	7, 7, 6, 6, 5, 5, 4, 4, 3, 3,
	3, 2, 2, 2, 2, 2, 2, 1, 1, 1,
}

var (
	// categories 是题目分类列表，循环分配给题目
	categories = []string{"web", "pwn", "reverse", "crypto", "misc"}
	// scores 是题目分值列表，循环分配给题目
	scores = []int{100, 150, 200, 250, 300, 350, 400, 450, 500}

	// compTitles 是 15 个比赛的标题
	compTitles = []string{
		"2026 春季网络安全挑战赛",
		"第四届全国大学生信息安全竞赛",
		"RedTeam 攻防演练 CTF",
		"新星杯网络安全技能大赛",
		"极客挑战营 CTF 热身赛",
		"企业安全攻防演练周",
		"黑客马拉松 CTF 公开赛",
		"数据安全与隐私保护大赛",
		"云原生安全挑战赛",
		"工控系统安全实战赛",
		"第五届移动安全挑战赛",
		"区块链安全与隐私保护大赛",
		"AI 安全对抗赛",
		"网络安全应急响应实战赛",
		"零信任架构安全挑战赛",
	}

	// compDescs 是 15 个比赛的描述
	compDescs = []string{
		"汇聚全国顶尖黑客，共赴春季安全盛宴",
		"提升大学生网络安全意识与实战能力",
		"模拟真实攻防场景，检验企业防护能力",
		"发掘网络安全新星，培养未来安全专家",
		"为正式比赛热身，挑战高难度题目",
		"强化企业安全团队实战能力",
		"48小时极限挑战，展示黑客精神",
		"聚焦数据安全，保护个人隐私",
		"探索云原生环境下的安全挑战",
		"工业控制系统安全防护实战",
		"移动应用安全漏洞挖掘与利用",
		"区块链智能合约安全审计大赛",
		"AI 模型安全性与对抗性研究",
		"网络安全事件应急响应实战演练",
		"零信任架构下的安全挑战与解决方案",
	}

	// chalTemplates 是各分类下的英文题目模板
	chalTemplates = map[string][]struct {
		title string
		desc  string
		flag  string
	}{
		"web": {
			{"Broken Authentication", "Bypass the login system to get admin access.", "flag{br0k3n_4uth3nt1c4t10n_ftw}"},
			{"SQL Injection 101", "Find the injection point and dump the database.", "flag{sql1_1nj3ct10n_m4st3r}"},
			{"XSS Playground", "Execute arbitrary JavaScript in the admin's browser.", "flag{xss_cr0ss_s1t3_scr1pt}"},
			{"File Upload Bypass", "Upload a webshell and get RCE.", "flag{f1l3_upl04d_byp4ss3d}"},
			{"SSRF Attack", "Exploit SSRF to access internal services.", "flag{ssrf_s3rv3r_s1d3_r3qu3st}"},
			{"Insecure Deserialization", "Abuse the Java deserialization mechanism.", "flag{d3s3r14l1z4t10n_1s_3v1l}"},
			{"Prototype Pollution", "Pollute the Object.prototype in JavaScript.", "flag{pr0t0typ3_p0llut10n}"},
			{"Path Traversal", "Read files outside the web root.", "flag{p4th_tr4v3rs4l_3xpl01t}"},
			{"Race Condition", "Win the race by exploiting TOCTOU.", "flag{r4c3_c0nd1t10n_w1nn3r}"},
			{"JWT Forgery", "Forge a valid JWT token without the secret.", "flag{jwt_f0rg3ry_4tt4ck}"},
		},
		"pwn": {
			{"Buffer Overflow Basic", "Smash the stack and control EIP.", "flag{buff3r_0v3rfl0w_m4st3r}"},
			{"Ret2libc", "Return to libc without shellcode.", "flag{r3t2l1bc_3xpl01t}"},
			{"ROP Chain", "Build a ROP chain to get a shell.", "flag{r0p_ch41n_cr34t0r}"},
			{"Format String", "Leak memory and execute arbitrary code.", "flag{f0rm4t_str1ng_4tt4ck}"},
			{"Use After Free", "Exploit a dangling pointer vulnerability.", "flag{us3_4ft3r_fr33_3xpl01t}"},
			{"Heap Overflow", "Overflow a heap chunk and take control.", "flag{h34p_0v3rfl0w_pwn3d}"},
			{"House of Force", "Use the House of Force technique.", "flag{h0us3_0f_f0rc3_m4st3r}"},
			{"Double Free", "Trigger a double free vulnerability.", "flag{d0ubl3_fr33_vuln}"},
			{"Stack Canary Bypass", "Leak the canary and smash the stack.", "flag{c4n4ry_byp4ss3d_pwn3d}"},
			{"Syscall Table", "Build a custom shellcode using syscalls.", "flag{sysc4ll_sh3llc0d3_m4st3r}"},
		},
		"reverse": {
			{"Crack Me If You Can", "Find the correct password.", "flag{cr4ck_m3_1f_y0u_c4n}"},
			{"Keygen Me", "Write a keygen for this binary.", "flag{k3yg3n_m3_pl34s3}"},
			{"VM Protection", "Reverse the custom VM architecture.", "flag{vm_pr0t3ct10n_r3v3rs3d}"},
			{"Obfuscated Code", "Deobfuscate and understand the code.", "flag{0bfusc4t3d_c0d3_cr4ck3d}"},
			{"Packed Binary", "Unpack and analyze the binary.", "flag{p4ck3d_b1n4ry_unp4ck3d}"},
			{"Anti-Debugging", "Bypass anti-debugging measures.", "flag{4nt1_d3bug_byp4ss3d}"},
			{"API Hooking", "Find the hidden API calls.", "flag{4p1_h00k1ng_d3t3ct3d}"},
			{"Cryptographic Reverse", "Find the hidden crypto key.", "flag{crypt0_r3v3rs3_m4st3r}"},
			{"Native Library", "Reverse the Android native library.", "flag{n4t1v3_l1br4ry_r3v3rs3d}"},
			{"Firmware Analysis", "Extract secrets from the firmware.", "flag{f1rmw4r3_4n4lys1s_c0mpl3t3}"},
		},
		"crypto": {
			{"RSA Weak Modulus", "Factorize the weak RSA modulus.", "flag{rs4_w34k_m0dulus_cr4ck3d}"},
			{"AES ECB Mode", "Exploit AES in ECB mode.", "flag{43s_3cb_m0d3_1ns3cur3}"},
			{"Padding Oracle", "Use padding oracle attack to decrypt.", "flag{p4dd1ng_0r4cl3_4tt4ck}"},
			{"Hash Length Extension", "Perform hash length extension attack.", "flag{h4sh_l3ngth_3xt3ns10n}"},
			{"Timing Attack", "Extract the key using timing analysis.", "flag{t1m1ng_4tt4ck_3xpl01t}"},
			{"LFSR Prediction", "Predict the LFSR output.", "flag{lfsr_pr3d1ct10n_m4st3r}"},
			{"Mersenne Twister", "Predict MT19937 outputs.", "flag{m3rs3nn3_tw1st3r_cr4ck3d}"},
			{"Elliptic Curve", "Solve the discrete log problem.", "flag{3ll1pt1c_curv3_cr4ck3d}"},
			{"One Time Pad", "Recover the plaintext with reused key.", "flag{0n3_t1m3_p4d_r3us3d}"},
			{"Side Channel", "Extract secrets via power analysis.", "flag{s1d3_ch4nn3l_4tt4ck}"},
		},
		"misc": {
			{"Forensics 101", "Find the hidden flag in the image.", "flag{f0r3ns1cs_101_fl4g_f0und}"},
			{"Steganography", "Extract the hidden message.", "flag{st3g4n0gr4phy_m4st3r}"},
			{"Memory Forensics", "Analyze the memory dump for secrets.", "flag{m3m0ry_f0r3ns1cs_3xp3rt}"},
			{"PCAP Analysis", "Find the flag in the network capture.", "flag{pc4p_4n4lys1s_c0mpl3t3}"},
			{"ZIP Password", "Crack the ZIP password.", "flag{z1p_p4ssw0rd_cr4ck3d}"},
			{"QR Code Decode", "Decode the custom QR code.", "flag{qr_c0d3_d3c0d3d}"},
			{"Braille ASCII", "Decode the braille message.", "flag{br41ll3_4sc11_d3c0d3d}"},
			{"Morse Code", "Translate the morse code.", "flag{m0rs3_c0d3_tr4nsl4t3d}"},
			{"Encoding Madness", "Decode the multi-encoded string.", "flag{3nc0d1ng_m4dn3ss_s0lv3d}"},
			{"OSINT Challenge", "Find the flag using public information.", "flag{0s1nt_ch4ll3ng3_c0mpl3t3}"},
		},
	}

	// chalCN 是中文题目模板，约 3 道题目使用中文
	chalCN = []struct {
		title string
		desc  string
		flag  string
	}{
		{"简单的Web题", "寻找并利用网站的SQL注入漏洞。", "flag{w3b_sql_1nj3ct10n_e4sy}"},
		{"逆向入门", "分析这个简单的程序，找出正确的序列号。", "flag{r3v3rs3_34sy_st4rt3r}"},
		{"杂项签到题", "欢迎参加比赛！", "flag{w3lc0m3_t0_ctf_2026}"},
	}

	chalIdx map[string]int // 每个分类已使用的模板索引计数器
)

// ── 配置 ──

func authURL() string {
	if v := os.Getenv("AUTH_URL"); v != "" {
		return v
	}
	return "http://localhost:8081"
}

func ctfURL() string {
	if v := os.Getenv("CTF_URL"); v != "" {
		return v
	}
	return "http://localhost:8080"
}

// ── HTTP 辅助函数 ──

// postJSON 发送 POST 请求，body 可以为 nil。
// token 非空时设置 Authorization 头。
// 非 2xx 状态码直接 log.Fatalf。遇到 429 自动重试。
func postJSON(url string, body any, token string) map[string]any {
	for attempt := 0; attempt < 5; attempt++ {
		var bodyReader io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			bodyReader = bytes.NewReader(b)
		}
		req, err := http.NewRequest("POST", url, bodyReader)
		if err != nil {
			log.Fatalf("new request %s: %v", url, err)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatalf("post %s: %v", url, err)
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 429 {
			time.Sleep(time.Duration(2+attempt) * time.Second)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Fatalf("post %s: status %d: %s", url, resp.StatusCode, data)
		}
		if len(data) == 0 {
			return nil
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			log.Fatalf("decode %s: %v (%s)", url, err, data)
		}
		return m
	}
	log.Fatalf("post %s: too many retries (429)", url)
	return nil
}

// registerAndLogin 注册用户并返回 JWT token 和用户 ID。
// 如果用户已存在（409），直接登录（登录时无法获取用户ID，返回空字符串）。
func registerAndLogin(username, password string) (token string, userID string) {
	return registerAndLoginWithRole(username, password, "")
}

// registerAndLoginWithRole 注册指定角色的用户并返回 JWT token 和用户 ID。
// 注意：注册 API 会忽略 role 参数（安全限制），所以通过数据库直接提升角色。
func registerAndLoginWithRole(username, password, role string) (token string, userID string) {
	regMu.Lock()
	defer regMu.Unlock()

	db := getDB()
	defer db.Close()

	// 检查用户是否已存在
	var existingID, existingRole string
	db.QueryRow("SELECT res_id, role FROM users WHERE username = ?", username).Scan(&existingID, &existingRole)
	if existingID != "" {
		token = signToken(existingID, existingRole)
		return token, existingID
	}

	// 生成 res_id 和密码 hash
	resID := newUUID()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("hash password for %s: %v", username, err)
	}
	userRole := "member"
	if role != "" {
		userRole = role
	}
	_, err = db.Exec("INSERT INTO users (res_id, username, password_hash, role) VALUES (?, ?, ?, ?)",
		resID, username, string(hash), userRole)
	if err != nil {
		log.Fatalf("insert user %s: %v", username, err)
	}

	token = signToken(resID, userRole)
	return token, resID
}

// signToken 直接用 JWT secret 签发 token，绕过 auth 服务限流。
func signToken(userID, role string) string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-only-secret-change-in-production-abc123"
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	})
	token, err := t.SignedString([]byte(secret))
	if err != nil {
		log.Fatalf("sign token: %v", err)
	}
	return token
}

// newUUID 生成 32 字符十六进制 UUID（与项目 internal/uuid 包兼容）。
func newUUID() string {
	b := make([]byte, 16)
	crypto_rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func login(username, password string) (token string, userID string) {
	m := postJSON(authURL()+"/api/v1/login", map[string]string{
		"username": username,
		"password": password,
	}, "")
	tok, ok := m["token"].(string)
	if !ok {
		log.Fatalf("login %s: no token in response", username)
	}
	// 登录响应不包含用户ID，返回空字符串
	return tok, ""
}

// ── API 调用函数 ──

func apiCreateChallenge(token, title, category, desc string, score int, flag string) string {
	m := postJSON(ctfURL()+"/api/v1/admin/challenges", map[string]any{
		"title": title, "category": category, "description": desc,
		"score": score, "flag": flag,
	}, token)
	id, ok := m["id"].(string)
	if !ok {
		log.Fatalf("create challenge: no id: %v", m)
	}
	return id
}

func apiCreateCompetition(token, title, desc string, start, end time.Time, mode, teamJoinMode string) string {
	payload := map[string]any{
		"title": title, "description": desc,
		"start_time": start.Format(time.RFC3339),
		"end_time":   end.Format(time.RFC3339),
	}
	if mode != "" {
		payload["mode"] = mode
	}
	if teamJoinMode != "" {
		payload["team_join_mode"] = teamJoinMode
	}
	m := postJSON(ctfURL()+"/api/v1/admin/competitions", payload, token)
	id, ok := m["id"].(string)
	if !ok {
		log.Fatalf("create competition: no id: %v", m)
	}
	return id
}

func apiAddChallengeToComp(token, compID, chalID string) {
	postJSON(ctfURL()+"/api/v1/admin/competitions/"+compID+"/challenges",
		map[string]string{"challenge_id": chalID}, token)
}

func apiStartComp(token, compID string) {
	postJSONIgnore409(ctfURL()+"/api/v1/admin/competitions/"+compID+"/start", token)
}

func apiEndComp(token, compID string) {
	postJSONIgnore409(ctfURL()+"/api/v1/admin/competitions/"+compID+"/end", token)
}

func apiCreateTeam(token, name string) string {
	m := postJSON(authURL()+"/api/v1/admin/teams", map[string]string{"name": name}, token)
	id, ok := m["id"].(string)
	if !ok {
		log.Fatalf("create team: no id: %v", m)
	}
	return id
}

func apiAddTeamMember(token, teamID, userID string) {
	postJSON(authURL()+"/api/v1/admin/teams/"+teamID+"/members",
		map[string]string{"user_id": userID}, token)
}

func apiAddTeamToComp(token, compID, teamID string) {
	postJSON(ctfURL()+"/api/v1/admin/competitions/"+compID+"/teams",
		map[string]string{"team_id": teamID}, token)
}

// postJSONIgnore409 发送 POST 请求，容忍 409（已处于目标状态）。遇到 429 自动重试。
func postJSONIgnore409(url string, token string) {
	for attempt := 0; attempt < 5; attempt++ {
		req, err := http.NewRequest("POST", url, nil)
		if err != nil {
			log.Fatalf("new request %s: %v", url, err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatalf("post %s: %v", url, err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 429 {
			time.Sleep(time.Duration(2+attempt) * time.Second)
			continue
		}
		if resp.StatusCode == 409 {
			return
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Fatalf("post %s: status %d", url, resp.StatusCode)
		}
		return
	}
	log.Fatalf("post %s: too many retries (429)", url)
}

func apiSubmitFlag(token, compID, chalID, flag string) map[string]any {
	return postJSON(ctfURL()+"/api/v1/competitions/"+compID+"/challenges/"+chalID+"/submit",
		map[string]string{"flag": flag}, token)
}

// ── 随机选取 ──

func pickN(rng *rand.Rand, ids []string, n int) []string {
	s := make([]string, len(ids))
	copy(s, ids)
	rng.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
	return s[:n]
}

// ── 比赛模式类型 ──

type compMode string

const (
	modeIndividual compMode = "individual"
	modeTeamFree   compMode = "team"
	modeTeamAdmin  compMode = "team"
)

// getCompMode 根据比赛索引返回比赛模式
// 0: individual（个人模式）
// 1: team + free（队伍模式，自由加入）
func getCompMode(i int) (mode compMode, teamJoinMode string) {
	if i == 0 {
		return modeIndividual, ""
	}
	return modeTeamFree, "free"
}

// ── 用户提交任务（并发执行） ──

// userJob 是单个用户在比赛中的提交任务。
type userJob struct {
	CompIdx int              // 比赛序号
	UserIdx int              // 用户序号（0=最强）
	CompID  string           // 比赛 res_id
	ChalIDs []string         // 分配到该比赛的题目
	Flags   map[string]string // res_id → flag
	RNG     *rand.Rand       // 每个用户独立的随机数生成器
}

// run 注册用户、登录、按 solveCounts 提交 flag。
// 每次提交后 sleep submitDelay 以遵守限流规则。
func (j *userJob) run(wg *sync.WaitGroup) {
	defer wg.Done()

	username := fmt.Sprintf("comp%02d_player_%03d", j.CompIdx+1, j.UserIdx+1)
	userToken, _ := registerAndLogin(username, "password123")

	nCorrect := solveCounts[j.UserIdx]

	picked := make([]string, len(j.ChalIDs))
	copy(picked, j.ChalIDs)
	j.RNG.Shuffle(len(picked), func(i, k int) { picked[i], picked[k] = picked[k], picked[i] })

	correct := picked[:nCorrect]
	rest := picked[nCorrect:]

	// 正确提交：30% 概率先有一次错误尝试
	for _, cid := range correct {
		if j.RNG.Float64() < 0.3 {
			apiSubmitFlag(userToken, j.CompID, cid, "flag{wrong_attempt}")
			time.Sleep(submitDelay)
		}
		apiSubmitFlag(userToken, j.CompID, cid, j.Flags[cid])
		time.Sleep(submitDelay)
	}

	// 未解对的题目：0-2 次错误尝试
	for i := 0; i < j.RNG.Intn(3) && i < len(rest); i++ {
		apiSubmitFlag(userToken, j.CompID, rest[i], "flag{wrong_attempt}")
		time.Sleep(submitDelay)
	}
}

// teamJob 是队伍在比赛中的提交任务。
type teamJob struct {
	CompIdx     int              // 比赛序号
	TeamIdx     int              // 队伍序号（0=最强）
	CompID      string           // 比赛 res_id
	TeamID      string           // 队伍 res_id（由 main 预创建）
	AdminToken  string           // admin token（用于添加队员）
	ChalIDs     []string         // 分配到该比赛的题目
	Flags       map[string]string // res_id → flag
	RNG         *rand.Rand       // 每个队伍独立的随机数生成器
}

// run 为队伍注册用户、分配到队伍、按团队能力提交 flag。
func (j *teamJob) run(wg *sync.WaitGroup) {
	defer wg.Done()

	teamSize := 3 + j.RNG.Intn(3) // 每队 3-5 人

	// 队伍解决的题目数量（根据队伍序号，0队最强）
	nTeamCorrect := solveCounts[j.TeamIdx]

	// 注册并分配队员（每队最多 5 人，偏移 = TeamIdx * 5 避免用户重叠）
	maxTeamSize := 5
	userTokens := make([]string, 0, teamSize)
	for u := 0; u < teamSize; u++ {
		userIdx := j.TeamIdx*maxTeamSize + u
		if userIdx >= usersPerComp {
			userIdx = usersPerComp - 1
		}
		username := fmt.Sprintf("comp%02d_player_%03d", j.CompIdx+1, userIdx+1)
		token, userID := registerAndLogin(username, "password123")
		userTokens = append(userTokens, token)

		if userID != "" {
			apiAddTeamMember(j.AdminToken, j.TeamID, userID)
		}
	}

	if len(userTokens) == 0 {
		return
	}

	// 打乱题目
	picked := make([]string, len(j.ChalIDs))
	copy(picked, j.ChalIDs)
	j.RNG.Shuffle(len(picked), func(i, k int) { picked[i], picked[k] = picked[k], picked[i] })

	correct := picked[:nTeamCorrect]
	rest := picked[nTeamCorrect:]

	// 由不同队员提交正确答案，模拟团队协作
	for idx, cid := range correct {
		userIdx := idx % len(userTokens)
		token := userTokens[userIdx]

		// 30% 概率先有一次错误尝试
		if j.RNG.Float64() < 0.3 {
			wrongUserIdx := (userIdx + 1) % len(userTokens)
			apiSubmitFlag(userTokens[wrongUserIdx], j.CompID, cid, "flag{wrong_attempt}")
			time.Sleep(submitDelay)
		}

		apiSubmitFlag(token, j.CompID, cid, j.Flags[cid])
		time.Sleep(submitDelay)
	}

	// 未解对的题目：0-2 次错误尝试，由随机队员提交
	numWrongAttempts := j.RNG.Intn(3)
	for i := 0; i < numWrongAttempts && i < len(rest); i++ {
		userIdx := (i + 1) % len(userTokens)
		apiSubmitFlag(userTokens[userIdx], j.CompID, rest[i], "flag{wrong_attempt}")
		time.Sleep(submitDelay)
	}
}

// ── main ──

func main() {
	flag.Parse()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	chalIdx = make(map[string]int)

	log.Printf("auth=%s  ctf=%s", authURL(), ctfURL())

	// 注册 admin 用户并获取 token
	log.Println("registering admin user...")
	adminToken, _ := registerAndLoginWithRole("seed_admin", "seed_admin_password", "admin")

	// 创建 50 道题目
	log.Println("creating challenges...")
	chalFlags := make(map[string]string)
	chalIDs := make([]string, poolSize)
	cnCount := 0
	for i := range chalIDs {
		cat := categories[i%len(categories)]
		sc := scores[i%len(scores)]
		var title, desc, flag string
		if cnCount < 3 && i%17 == 0 {
			t := chalCN[cnCount%len(chalCN)]
			title = t.title
			desc = t.desc
			flag = t.flag
			cnCount++
		} else {
			templates := chalTemplates[cat]
			idx := chalIdx[cat]
			t := templates[idx%len(templates)]
			title = t.title
			desc = t.desc
			flag = t.flag
			chalIdx[cat]++
		}
		id := apiCreateChallenge(adminToken, title, cat, desc, sc, flag)
		chalIDs[i] = id
		chalFlags[id] = flag
	}
	log.Printf("created %d challenges", len(chalIDs))

	// 创建比赛：i=0 个人赛，i=1 团队赛
	now := time.Now()
	for i := 0; i < numComps; i++ {
		start := now.Add(-1 * time.Hour)
		end := now.Add(48 * time.Hour)

		mode, teamJoinMode := getCompMode(i)
		var modeStr, teamJoinModeStr string
		if mode != modeIndividual {
			modeStr = string(mode)
			teamJoinModeStr = teamJoinMode
		}

		compID := apiCreateCompetition(adminToken,
			compTitles[i%len(compTitles)],
			compDescs[i%len(compDescs)],
			start, end, modeStr, teamJoinModeStr)

		picked := pickN(rng, chalIDs, chalsPerComp)
		for _, cid := range picked {
			apiAddChallengeToComp(adminToken, compID, cid)
		}

		var teamIDs []string
		if mode != modeIndividual {
			// 队伍模式：创建队伍
			log.Printf("competition %02d: creating %d teams...", i+1, teamsPerComp)
			for t := 0; t < teamsPerComp; t++ {
				teamName := fmt.Sprintf("comp%02d_team_%02d", i+1, t+1)
				teamID := apiCreateTeam(adminToken, teamName)
				teamIDs = append(teamIDs, teamID)

				// 如果是 admin 加入模式，将队伍添加到比赛
				if teamJoinMode == "admin" {
					apiAddTeamToComp(adminToken, compID, teamID)
				}
			}
		}

		apiStartComp(adminToken, compID)
		log.Printf("competition %02d  id=%s  mode=%s  team_join=%s  started",
			i+1, compID, func() string {
				if mode == modeIndividual {
					return "individual"
				}
				return "team"
			}(), func() string {
				if teamJoinMode == "" {
					return "-"
				}
				return teamJoinMode
			}())

		// 并发启动提交任务
		var wg sync.WaitGroup
		if mode == modeIndividual {
			// 个人模式：每个用户独立提交
			for u := 0; u < usersPerComp; u++ {
				wg.Add(1)
				job := &userJob{
					CompIdx: i,
					UserIdx: u,
					CompID:  compID,
					ChalIDs: picked,
					Flags:   chalFlags,
					RNG:     rand.New(rand.NewSource(now.UnixNano() + int64(i)*100 + int64(u))),
				}
				go job.run(&wg)
			}
		} else {
			// 队伍模式：每个队伍作为一个整体提交
			for t := 0; t < teamsPerComp; t++ {
				wg.Add(1)
				job := &teamJob{
					CompIdx:    i,
					TeamIdx:    t,
					CompID:     compID,
					TeamID:     teamIDs[t],
					AdminToken: adminToken,
					ChalIDs:    picked,
					Flags:      chalFlags,
					RNG:        rand.New(rand.NewSource(now.UnixNano() + int64(i)*100 + int64(t))),
				}
				go job.run(&wg)
			}
		}
		wg.Wait()

		apiEndComp(adminToken, compID)
		log.Printf("competition %02d  id=%s  %s ~ %s  done", i+1, compID,
			start.Format("2006-01-02 15:04"), end.Format("2006-01-02 15:04"))
	}

	log.Println("seed complete!")
}
