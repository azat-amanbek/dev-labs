//go:build ignore

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lib/pq"
)

const dsn = "host=127.0.0.1 port=5432 user=dev password=dev dbname=playground sslmode=disable"

func main() {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(60)
	if err := db.Ping(); err != nil {
		panic(err)
	}
	fmt.Println("HOT KEY: 500 concurrent disbursements on ONE client, limit=1000 amount=10 -> max 100 must pass")
	runHot(db, "FOR UPDATE", func() (bool, int) { return disburseLocked(db, 1, 10), 1 })
	runHot(db, "SERIALIZABLE", func() (bool, int) { return disburseSerializable(db, 1, 10) })
}

func runHot(db *sql.DB, name string, fn func() (bool, int)) {
	mustExec(db, `TRUNCATE disbursement`)
	mustExec(db, `UPDATE client_exposure SET outstanding = 0, limit_amount = 1000 WHERE client_id = 1`)
	const N = 500
	var ok, attempts int64
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			applied, at := fn()
			atomic.AddInt64(&attempts, int64(at))
			if applied {
				atomic.AddInt64(&ok, 1)
			}
		}()
	}
	wg.Wait()
	out := scalar(db, `SELECT outstanding FROM client_exposure WHERE client_id = 1`)
	fmt.Printf("[%-12s] succeeded=%3d  tx_attempts=%4d (retries=%4d)  outstanding=%d/1000  %v\n",
		name, ok, attempts, attempts-N, out, time.Since(start).Round(time.Millisecond))
}

func disburseLocked(db *sql.DB, clientID, amount int64) bool {
	tx, err := db.Begin()
	if err != nil {
		return false
	}
	defer tx.Rollback()
	var outstanding, limit int64
	if err := tx.QueryRow(`SELECT outstanding, limit_amount FROM client_exposure WHERE client_id=$1 FOR UPDATE`, clientID).Scan(&outstanding, &limit); err != nil {
		return false
	}
	if outstanding+amount > limit {
		return false
	}
	if _, err := tx.Exec(`UPDATE client_exposure SET outstanding=outstanding+$1 WHERE client_id=$2`, amount, clientID); err != nil {
		return false
	}
	if _, err := tx.Exec(`INSERT INTO disbursement(client_id, amount) VALUES($1,$2)`, clientID, amount); err != nil {
		return false
	}
	return tx.Commit() == nil
}

func disburseSerializable(db *sql.DB, clientID, amount int64) (bool, int) {
	attempts := 0
	for {
		attempts++
		applied, err := trySerializable(db, clientID, amount)
		if err == nil {
			return applied, attempts
		}
		if isSerFailure(err) {
			continue // conflict -> retry the whole tx
		}
		return false, attempts
	}
}

func trySerializable(db *sql.DB, clientID, amount int64) (bool, error) {
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var outstanding, limit int64
	if err := tx.QueryRow(`SELECT outstanding, limit_amount FROM client_exposure WHERE client_id=$1`, clientID).Scan(&outstanding, &limit); err != nil {
		return false, err
	}
	if outstanding+amount > limit {
		return false, nil // rejected by limit, not retryable
	}
	if _, err := tx.Exec(`UPDATE client_exposure SET outstanding=outstanding+$1 WHERE client_id=$2`, amount, clientID); err != nil {
		return false, err
	}
	if _, err := tx.Exec(`INSERT INTO disbursement(client_id, amount) VALUES($1,$2)`, clientID, amount); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func isSerFailure(err error) bool {
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
