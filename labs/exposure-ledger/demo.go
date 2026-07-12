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
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(20)
	if err := db.Ping(); err != nil {
		panic(err)
	}

	fmt.Println("######## DEMO 1: FOR UPDATE — 5 concurrent on client 1, limit=20 amount=10 (2 win) ########")
	reset(db)
	start = time.Now()
	done := make(chan struct{})
	go monitor(db, done)
	var wg sync.WaitGroup
	for i := 1; i <= 5; i++ {
		wg.Add(1)
		go func(id int) { defer wg.Done(); disburseLockedVerbose(db, id, 1, 10) }(i)
	}
	wg.Wait()
	close(done)
	time.Sleep(30 * time.Millisecond)
	fmt.Printf(">>> final outstanding = %d (limit 20)\n\n", scalar(db, `SELECT outstanding FROM client_exposure WHERE client_id=1`))

	fmt.Println("######## DEMO 2: SERIALIZABLE — same load, watch aborts (40001) & retries ########")
	reset(db)
	start = time.Now()
	var wg2 sync.WaitGroup
	for i := 1; i <= 5; i++ {
		wg2.Add(1)
		go func(id int) { defer wg2.Done(); serVerbose(db, id, 1, 10) }(i)
	}
	wg2.Wait()
	fmt.Printf(">>> final outstanding = %d (limit 20)\n", scalar(db, `SELECT outstanding FROM client_exposure WHERE client_id=1`))
}

func reset(db *sql.DB) {
	mustExec(db, `TRUNCATE disbursement`)
	mustExec(db, `UPDATE client_exposure SET outstanding=0, limit_amount=20 WHERE client_id=1`)
}

// monitor asks Postgres itself who is blocked by whom, live.
func monitor(db *sql.DB, done <-chan struct{}) {
	t := time.NewTicker(12 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
			rows, err := db.Query(`SELECT pid, coalesce(wait_event_type,''), coalesce(wait_event,''), pg_blocking_pids(pid)::int8[]
				FROM pg_stat_activity
				WHERE datname='playground' AND state='active' AND pid <> pg_backend_pid()
				  AND cardinality(pg_blocking_pids(pid)) > 0`)
			if err != nil {
				continue
			}
			for rows.Next() {
				var pid int
				var wet, we string
				var by pq.Int64Array
				if rows.Scan(&pid, &wet, &we, &by) == nil {
					mu.Lock()
					fmt.Printf("[+%-7v DB] postgres says: backend %d is BLOCKED (%s/%s), waiting on backend %v\n",
						time.Since(start).Round(time.Millisecond), pid, wet, we, []int64(by))
					mu.Unlock()
				}
			}
			rows.Close()
		}
	}
}

func disburseLockedVerbose(db *sql.DB, gid int, clientID, amount int64Alias) bool {
	tx, err := db.Begin()
	if err != nil {
		return false
	}
	defer tx.Rollback()

	var pid int
	tx.QueryRow(`SELECT pg_backend_pid()`).Scan(&pid)
	logf(gid, "BEGIN                      (postgres backend pid=%d)", pid)

	t0 := time.Now()
	var outstanding, limit int64
	if err := tx.QueryRow(`SELECT outstanding, limit_amount FROM client_exposure WHERE client_id=$1 FOR UPDATE`, clientID).Scan(&outstanding, &limit); err != nil {
		return false
	}
	waited := time.Since(t0)
	if waited > 5*time.Millisecond {
		logf(gid, "SELECT ... FOR UPDATE  -> acquired AFTER WAITING %v  (outstanding=%d)", waited.Round(time.Millisecond), outstanding)
	} else {
		logf(gid, "SELECT ... FOR UPDATE  -> acquired immediately     (outstanding=%d)", outstanding)
	}

	if outstanding+amount > limit {
		logf(gid, "check %d+%d > %d  -> REJECT + ROLLBACK", outstanding, amount, limit)
		return false
	}
	logf(gid, "check %d+%d <= %d  OK -> UPDATE, holding lock 40ms...", outstanding, amount, limit)
	time.Sleep(40 * time.Millisecond)
	tx.Exec(`UPDATE client_exposure SET outstanding=outstanding+$1 WHERE client_id=$2`, amount, clientID)
	tx.Exec(`INSERT INTO disbursement(client_id, amount) VALUES($1,$2)`, clientID, amount)
	if tx.Commit() == nil {
		logf(gid, "COMMIT -> lock released, outstanding now %d", outstanding+amount)
		return true
	}
	return false
}

func serVerbose(db *sql.DB, gid int, clientID, amount int64Alias) bool {
	for attempt := 1; ; attempt++ {
		applied, retry := trySerVerbose(db, gid, attempt, clientID, amount)
		if !retry {
			return applied
		}
		logf(gid, "   x 40001 serialization_failure -> RETRY (attempt %d)", attempt+1)
	}
}

func trySerVerbose(db *sql.DB, gid, attempt int, clientID, amount int64Alias) (bool, bool) {
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return false, false
	}
	defer tx.Rollback()
	var pid int
	tx.QueryRow(`SELECT pg_backend_pid()`).Scan(&pid)

	var outstanding, limit int64
	if err := tx.QueryRow(`SELECT outstanding, limit_amount FROM client_exposure WHERE client_id=$1`, clientID).Scan(&outstanding, &limit); err != nil {
		return false, isSer(err)
	}
	if outstanding+amount > limit {
		logf(gid, "try#%d read outstanding=%d, check fails -> REJECT", attempt, outstanding)
		return false, false
	}
	logf(gid, "try#%d read outstanding=%d (pid=%d), check OK -> write, commit...", attempt, outstanding, pid)
	time.Sleep(10 * time.Millisecond)
	tx.Exec(`UPDATE client_exposure SET outstanding=outstanding+$1 WHERE client_id=$2`, amount, clientID)
	tx.Exec(`INSERT INTO disbursement(client_id, amount) VALUES($1,$2)`, clientID, amount)
	if err := tx.Commit(); err != nil {
		if isSer(err) {
			return false, true
		}
		return false, false
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

type int64Alias = int64
