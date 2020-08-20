package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

func checkSession(sid, session string) (string, error) {
	rows, err := db.Query("SELECT time, uname FROM sessions WHERE sid=? AND id=?;", sid, session)

	if err != nil {
		return "", err
	}

	rows.Next()

	var sessionTime int64
	var uname string

	if err := rows.Scan(&sessionTime, &uname); err != nil {
		return "", err
	}

	defer rows.Close()

	now := time.Now().Unix()
	if sessionTime < now-(30*60) {
		db.Exec("DELETE FROM sessions WHERE sid=? AND id=?;", sid, session)
		return "", fmt.Errorf("session: %s, too old! %d seconds too old", session, now-sessionTime-(30*60))
	}

	row := db.QueryRow("SELECT role FROM people WHERE sid=? AND uname=?;", sid, uname)

	var role string

	err = row.Scan(&role)

	if err != nil {
		return "", err
	}

	return role, nil
}

func loadDatabase() *sql.DB {
	if godotenv.Load("credentials.env") != nil {
		logger.Fatal("Failed to get credentials while loading database")
	}

	uname := os.Getenv("DB_USER")
	pword := os.Getenv("DB_PASS")
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")

	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/edustream", uname, pword, host, port))

	if err != nil {
		logger.Fatal(err.Error())
	}

	return db
}
