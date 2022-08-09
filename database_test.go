package main

import (
	"testing"
)

func Test_database(t *testing.T) {
	t.Run("Basic Database Test", func(t *testing.T) {
		app := &goBlog{
			cfg: &config{},
		}

		db, err := app.openDatabase(":memory:", false)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		_, err = db.Exec("create table test(test text);")
		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		_, err = db.Exec("insert into test (test) values ('Test')")
		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		row, err := db.QueryRow("select count(test) from test")
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		var test1 int
		err = row.Scan(&test1)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if test1 != 1 {
			t.Error("Wrong result")
		}

		rows, err := db.Query("select count(test), test from test")
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		var test2 int
		var testStr string
		if !rows.Next() {
			t.Error("No result row")
		}
		err = rows.Scan(&test2, &testStr)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if test2 != 1 || testStr != "Test" {
			t.Error("Wrong result")
		}

		err = db.close()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
	})
}
