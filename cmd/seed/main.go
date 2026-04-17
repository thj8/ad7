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
		log.Printf("competition %02d  id=%-18d  %s ~ %s  done", i+1, compID,
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

func createChallenges(db *sql.DB) []int64 {
	ids := make([]int64, poolSize)
	for i := range ids {
		rid := snowflake.Next()
		cat := categories[i%len(categories)]
		sc := scores[i%len(scores)]
		_, err := db.Exec(`INSERT INTO challenges
			(res_id, title, category, description, score, flag, is_enabled)
			VALUES (?, ?, ?, ?, ?, ?, 1)`,
			rid,
			fmt.Sprintf("Challenge-%03d", i+1),
			cat,
			fmt.Sprintf("Description for challenge %d (%s)", i+1, cat),
			sc,
			fmt.Sprintf("flag{%s_%d}", cat, i+1),
		)
		must(err, "create challenge %d", i)
		ids[i] = rid
	}
	return ids
}

func createComp(db *sql.DB, idx int, now time.Time) (int64, time.Time, time.Time) {
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
		fmt.Sprintf("CTF Competition %02d", idx+1),
		fmt.Sprintf("Description for competition %d", idx+1),
		start, end,
	)
	must(err, "create competition %d", idx)
	return rid, start, end
}

func pickN(rng *rand.Rand, ids []int64, n int) []int64 {
	s := make([]int64, len(ids))
	copy(s, ids)
	rng.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
	return s[:n]
}

func assignChals(db *sql.DB, compID int64, chalIDs []int64) {
	for _, cid := range chalIDs {
		rid := snowflake.Next()
		_, err := db.Exec(`INSERT INTO competition_challenges
			(res_id, competition_id, challenge_id) VALUES (?, ?, ?)`,
			rid, compID, cid)
		must(err, "assign challenge to comp")
	}
}

func genSubmissions(db *sql.DB, rng *rand.Rand, compID int64, chalIDs []int64, compStart, compEnd time.Time) {
	dur := compEnd.Sub(compStart)

	for u := 0; u < usersPerComp; u++ {
		userID := fmt.Sprintf("player_%03d", u+1)
		nCorrect := solveCounts[u]

		// Pick which challenges this user solves.
		picked := make([]int64, len(chalIDs))
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

func insertSub(db *sql.DB, userID string, chalID, compID int64, correct bool, t time.Time) {
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
