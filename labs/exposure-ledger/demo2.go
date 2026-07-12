//go:build ignore

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lib/pq"
)

const dsn = "host=127.0.0.1 port=5432 user=dev password=dev dbname=playground sslmode=disable"

var start = time.Now()
var mu sync.Mutex

func logf(gid int, f string, a ...any) {
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("[+%-7v G%d] %s\n", time.Since(start).Round(time.Millisecond), gid, fmt.Sprintf(f, a...))
}

func main() {
	db, _ := sql.Open("postgres", dsn)
	db.SetMaxOpenConns(20)
	if err := db.Ping(); err != nil {
		panic(err)
	}
	mustExec(db, `TRUNCATE disbursement`)
	mustExec(db, `UPDATE client_exposure SET outstanding=0, limit_amount=20 WHERE client_id=1`)

	fmt.Println("######## SERIALIZABLE (fixed): 5 concurrent, limit=20 amount=10 -> 2 win, watch retries ########")
	start = time.Now()
	var wg sync.WaitGroup
	var totalAttempts int
	var amu sync.Mutex
	for i := 1; i <= 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			n := serVerbose(db, id, 1, 10)
			amu.Lock()
			totalAttempts += n
			amu.Unlock()
		}(i)
	}
	wg.Wait()
	out := scalar(db, `SELECT outstanding FROM client_exposure WHERE client_id=1`)
	fmt.Printf(">>> final outstanding=%d (limit 20) · total tx attempts across 5 logical ops = %d\n", out, totalAttempts)
}

func serVerbose(db *sql.DB, gid int, clientID, amount int64) int {
	for attempt := 1; ; attempt++ {
		applied, retry := trySer(db, gid, attempt, clientID, amount)
		if !retry {
			_ = applied
			return attempt
		}
		logf(gid, "   x 40001 serialization_failure -> RETRY (now attempt %d)", attempt+1)
	}
}

func trySer(db *sql.DB, gid, attempt int, clientID, amount int64) (bool, bool) {
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return false, false
	}
	defer tx.Rollback()

	var outstanding, limit int64
	if err := tx.QueryRow(`SELECT outstanding, limit_amount FROM client_exposure WHERE client_id=$1`, clientID).Scan(&outstanding, &limit); err != nil {
		return false, isSer(err)
	}
	if outstanding+amount > limit {
		logf(gid, "try#%d read outstanding=%d -> check fails -> REJECT (final)", attempt, outstanding)
		return false, false
	}
	logf(gid, "try#%d read outstanding=%d -> check OK -> UPDATE+INSERT+COMMIT...", attempt, outstanding)
	time.Sleep(8 * time.Millisecond) // widen the conflict window
	if _, err := tx.Exec(`UPDATE client_exposure SET outstanding=outstanding+$1 WHERE client_id=$2`, amount, clientID); err != nil {
		return false, isSer(err) // conflict often surfaces right here, on the write
	}
	if _, err := tx.Exec(`INSERT INTO disbursement(client_id, amount) VALUES($1,$2)`, clientID, amount); err != nil {
		return false, isSer(err)
	}
	if err := tx.Commit(); err != nil {
		return false, isSer(err)
	}
	logf(gid, "try#%d COMMIT OK -> outstanding now %d", attempt, outstanding+amount)
	return true, false
}

func isSer(err error) bool {
	var pe *pq.Error
	if errors.As(err, &pe) {
		return pe.Code == "40001"
	}
	return false
}

func mustExec(db *sql.DB, q string) {
	if _, err := db.Exec(q); err != nil {
		panic(err)
	}
}
func scalar(db *sql.DB, q string) int64 {
	var v int64
	if err := db.QueryRow(q).Scan(&v); err != nil {
		panic(err)
	}
	return v
}
