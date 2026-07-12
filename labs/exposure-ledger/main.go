package main

import (
	"database/sql"
	"fmt"
	"sync"

	_ "github.com/lib/pq"
)

const dsn = "host=127.0.0.1 port=5432 user=dev password=dev dbname=playground sslmode=disable"

func main() {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(50)
	if err := db.Ping(); err != nil {
		panic(err)
	}

	fmt.Println("scenario: 50 logical requests, each RETRIED 3x (150 calls), amount=10, high limit")
	runIdemTest(db, "no-idem", func(k string) { disburseLocked(db, 1, 10) })
	runIdemTest(db, "idem", func(k string) { disburseIdempotent(db, 1, 10, k) })
}

func runIdemTest(db *sql.DB, name string, call func(key string)) {
	mustExec(db, `TRUNCATE disbursement`)
	mustExec(db, `UPDATE client_exposure SET outstanding = 0, limit_amount = 1000000 WHERE client_id = 1`)
	const logical, retries, amount = 50, 3, 10
	var wg sync.WaitGroup
	for i := 0; i < logical; i++ {
		key := fmt.Sprintf("req-%03d", i)
		for r := 0; r < retries; r++ {
			wg.Add(1)
			go func(k string) { defer wg.Done(); call(k) }(key)
		}
	}
	wg.Wait()
	rows := scalar(db, `SELECT count(*) FROM disbursement`)
	out := scalar(db, `SELECT outstanding FROM client_exposure WHERE client_id = 1`)
	want := int64(logical * amount)
	status := "OK  each request applied exactly once"
	if out != want {
		status = fmt.Sprintf("XX  double-issue: charged %d, wanted %d (retries applied %dx)", out, want, out/want)
	}
	fmt.Printf("[%-7s] logical=%d calls=%d disbursements=%3d outstanding=%d (want %d)  %s\n",
		name, logical, logical*retries, rows, out, want, status)
}

// per-client lock, but NO idempotency: every call is a fresh correct disbursement.
func disburseLocked(db *sql.DB, clientID, amount int64) bool {
	tx, err := db.Begin()
	if err != nil {
		return false
	}
	defer tx.Rollback()
	var outstanding, limit int64
	if err := tx.QueryRow(
		`SELECT outstanding, limit_amount FROM client_exposure WHERE client_id = $1 FOR UPDATE`, clientID,
	).Scan(&outstanding, &limit); err != nil {
		return false
	}
	if outstanding+amount > limit {
		return false
	}
	if _, err := tx.Exec(`UPDATE client_exposure SET outstanding = outstanding + $1 WHERE client_id = $2`, amount, clientID); err != nil {
		return false
	}
	if _, err := tx.Exec(`INSERT INTO disbursement(client_id, amount) VALUES ($1, $2)`, clientID, amount); err != nil {
		return false
	}
	return tx.Commit() == nil
}

// per-client lock + idempotency key: a retried request is a no-op.
func disburseIdempotent(db *sql.DB, clientID, amount int64, key string) bool {
	tx, err := db.Begin()
	if err != nil {
		return false
	}
	defer tx.Rollback()
	var outstanding, limit int64
	if err := tx.QueryRow(
		`SELECT outstanding, limit_amount FROM client_exposure WHERE client_id = $1 FOR UPDATE`, clientID,
	).Scan(&outstanding, &limit); err != nil {
		return false
	}
	var id int64
	err = tx.QueryRow(
		`INSERT INTO disbursement(client_id, amount, idempotency_key) VALUES ($1, $2, $3)
		 ON CONFLICT (idempotency_key) DO NOTHING RETURNING id`, clientID, amount, key,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return tx.Commit() == nil // key already applied -> idempotent no-op
	}
	if err != nil {
		return false
	}
	if outstanding+amount > limit {
		return false // rollback drops the just-inserted row
	}
	if _, err := tx.Exec(`UPDATE client_exposure SET outstanding = outstanding + $1 WHERE client_id = $2`, amount, clientID); err != nil {
		return false
	}
	return tx.Commit() == nil
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
