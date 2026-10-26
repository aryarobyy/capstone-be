package testutil

import "testing"

func TestWithDBName(t *testing.T) {
	if got := WithDBName("host=x dbname=a user=u", "b"); got != "host=x dbname=b user=u" {
		t.Fatal(got)
	}
	if got := WithDBName("host=x", "b"); got != "host=x dbname=b" {
		t.Fatal(got)
	}
}

func TestOpenTestDB(t *testing.T) {
	db := OpenTestDB(t)
	Exec(t, db, `INSERT INTO users(name,email,password) VALUES('a','a@x.io','x')`)
	if Count(t, db, `SELECT count(*) FROM users`) != 1 {
		t.Fatal("expected one user")
	}
}
