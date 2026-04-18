package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"ad7/internal/snowflake"
)

const (
	numComps     = 15
	poolSize     = 50
	chalsPerComp = 25
	usersPerComp = 30
)

// solveCounts[i] = correct solves for user i (0 = best).
// User 0 → 18/25 = 72%; User 29 → 1/25 = 4%.
// Deliberate ties at lower ranks create realistic leaderboard dynamics
// while varied challenge scores differentiate tied solve-counts.
var solveCounts = [30]int{
	18, 16, 15, 14, 13, 12, 11, 10, 9, 8,
	7, 7, 6, 6, 5, 5, 4, 4, 3, 3,
	3, 2, 2, 2, 2, 2, 2, 1, 1, 1,
}

var (
	categories = []string{"web", "pwn", "reverse", "crypto", "misc"}
	scores     = []int{100, 150, 200, 250, 300, 350, 400, 450, 500}

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

	chalCN = []struct {
		title string
		desc  string
		flag  string
	}{
		{"简单的Web题", "寻找并利用网站的SQL注入漏洞。", "flag{w3b_sql_1nj3ct10n_e4sy}"},
		{"逆向入门", "分析这个简单的程序，找出正确的序列号。", "flag{r3v3rs3_34sy_st4rt3r}"},
		{"杂项签到题", "欢迎参加比赛！", "flag{w3lc0m3_t0_ctf_2026}"},
	}

	chalCats []string
	chalIdx  map[string]int
)

func dsn() string {
	if d := os.Getenv("TEST_DSN"); d != "" {
		return d
	}
	return "root:asfdsfedarjeiowvgfsd@tcp(192.168.5.44:3306)/ctf?parseTime=true"
}

func main() {
	clean := flag.Bool("clean", true, "delete all rows before seeding")
	flag.Parse()

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Initialize challenge category counters
	chalCats = make([]string, 0, poolSize)
	chalIdx = make(map[string]int)
	for i := 0; i < poolSize; i++ {
		cat := categories[i%len(categories)]
		chalCats = append(chalCats, cat)
	}

	db, err := sql.Open("mysql", dsn())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("ping: %v", err)
	}

	if *clean {
		cleanAll(db)
		log.Println("cleaned existing data")
	}

	// 1. Create a pool of 50 challenges with varied scores & categories.
	chalIDs := createChallenges(db)
	log.Printf("created %d challenges", len(chalIDs))

	// 2. Create 15 competitions + assign challenges + generate submissions.
	now := time.Now()
	for i := 0; i < numComps; i++ {
		compID, start, end := createComp(db, i, now)
		picked := pickN(rng, chalIDs, chalsPerComp)
		assignChals(db, compID, picked)
		genSubmissions(db, rng, compID, picked, start, end)
		log.Printf("competition %02d  id=%s  %s ~ %s  done", i+1, compID,
			start.Format("2006-01-02 15:04"), end.Format("2006-01-02 15:04"))
	}

	log.Println("seed complete!")
}

// ── helpers ──

func cleanAll(db *sql.DB) {
	for _, t := range []string{
		"competition_challenges", "notifications",
		"submissions", "competitions", "challenges",
	} {
		db.Exec("DELETE FROM " + t)
	}
}

func createChallenges(db *sql.DB) []string {
	ids := make([]string, poolSize)
	cnCount := 0 // Track Chinese challenges (limit to ~3)
	for i := range ids {
		rid := snowflake.Next()
		cat := categories[i%len(categories)]
		sc := scores[i%len(scores)]

		var title, desc, flag string

		// Use Chinese for ~3 challenges, rest English
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

		_, err := db.Exec(`INSERT INTO challenges
			(res_id, title, category, description, score, flag, is_enabled)
			VALUES (?, ?, ?, ?, ?, ?, 1)`,
			rid, title, cat, desc, sc, flag,
		)
		must(err, "create challenge %d", i)
		ids[i] = rid
	}
	return ids
}

func createComp(db *sql.DB, idx int, now time.Time) (string, time.Time, time.Time) {
	rid := snowflake.Next()

	var start, end time.Time
	switch {
	case idx < 5: // past
		start = now.AddDate(0, 0, -(idx+1)*7)
		end = start.Add(48 * time.Hour)
	case idx < 10: // current
		start = now.Add(time.Duration(idx-7) * 24 * time.Hour)
		end = start.Add(72 * time.Hour)
	default: // future
		start = now.AddDate(0, 0, (idx-9)*7)
		end = start.Add(48 * time.Hour)
	}

	_, err := db.Exec(`INSERT INTO competitions
		(res_id, title, description, start_time, end_time, is_active)
		VALUES (?, ?, ?, ?, ?, 1)`,
		rid,
		compTitles[idx%len(compTitles)],
		compDescs[idx%len(compDescs)],
		start, end,
	)
	must(err, "create competition %d", idx)
	return rid, start, end
}

func pickN(rng *rand.Rand, ids []string, n int) []string {
	s := make([]string, len(ids))
	copy(s, ids)
	rng.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
	return s[:n]
}

func assignChals(db *sql.DB, compID string, chalIDs []string) {
	for _, cid := range chalIDs {
		rid := snowflake.Next()
		_, err := db.Exec(`INSERT INTO competition_challenges
			(res_id, competition_id, challenge_id) VALUES (?, ?, ?)`,
			rid, compID, cid)
		must(err, "assign challenge to comp")
	}
}

func genSubmissions(db *sql.DB, rng *rand.Rand, compID string, chalIDs []string, compStart, compEnd time.Time) {
	dur := compEnd.Sub(compStart)

	for u := 0; u < usersPerComp; u++ {
		userID := fmt.Sprintf("player_%03d", u+1)
		nCorrect := solveCounts[u]

		// Pick which challenges this user solves.
		picked := make([]string, len(chalIDs))
		copy(picked, chalIDs)
		rng.Shuffle(len(picked), func(i, j int) { picked[i], picked[j] = picked[j], picked[i] })

		correct := picked[:nCorrect]
		rest := picked[nCorrect:]

		// Higher-ranked users start solving earlier (for tiebreak by last_solve_time).
		userBase := compStart.Add(dur / time.Duration(usersPerComp+1) * time.Duration(u))
		step := (compEnd.Sub(userBase)) / time.Duration(nCorrect+1)

		for j, cid := range correct {
			t := userBase.Add(step * time.Duration(j+1))
			// 30% chance of a prior wrong attempt
			if rng.Float64() < 0.3 {
				insertSub(db, userID, cid, compID, false, t.Add(-2*time.Minute))
			}
			insertSub(db, userID, cid, compID, true, t)
		}

		// 0-2 wrong attempts on unsolved challenges
		for j := 0; j < rng.Intn(3) && j < len(rest); j++ {
			t := compStart.Add(dur / time.Duration(u+2) * time.Duration(j+1))
			insertSub(db, userID, rest[j], compID, false, t)
		}
	}
}

func insertSub(db *sql.DB, userID string, chalID, compID string, correct bool, t time.Time) {
	flag := "flag{wrong_attempt}"
	if correct {
		flag = "flag{correct}"
	}
	rid := snowflake.Next()
	_, err := db.Exec(`INSERT INTO submissions
		(res_id, user_id, challenge_id, competition_id, submitted_flag, is_correct, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rid, userID, chalID, compID, flag, correct, t)
	must(err, "insert submission")
}

func must(err error, msg string, args ...any) {
	if err != nil {
		log.Fatalf(msg+": %v", append(args, err)...)
	}
}
