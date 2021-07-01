package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func (a *goBlog) setInMemoryDatabase() {
	a.db, _ = a.openDatabase(":memory:", false)
}

func Test_database(t *testing.T) {
	t.Run("Basic Database Test", func(t *testing.T) {
		app := &goBlog{
			cfg: &config{},
		}

		db, err := app.openDatabase(":memory:", false)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		_, err = db.exec("create table test(test text);")
		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		_, err = db.exec("insert into test (test) values ('Test')")
		if err != nil {
			t.Fatalf("Error: %v", err)
		}

		row, err := db.queryRow("select count(test) from test")
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

		rows, err := db.query("select count(test), test from test")
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

func Test_parallelDatabase(t *testing.T) {
	t.Run("Test parallel db access", func(t *testing.T) {
		// Test that parallel database access works without problems

		t.Parallel()

		app := &goBlog{
			cfg: &config{},
		}
		app.setInMemoryDatabase()

		_, err := app.db.exec("create table test(test text);")
		require.NoError(t, err)

		t.Run("1", func(t *testing.T) {
			for i := 0; i < 10000; i++ {
				_, e := app.db.exec("insert into test (test) values ('Test')")
				require.NoError(t, e)
			}
		})

		t.Run("2", func(t *testing.T) {
			for i := 0; i < 10000; i++ {
				_, e := app.db.exec("insert into test (test) values ('Test')")
				require.NoError(t, e)
			}
		})

		t.Run("3", func(t *testing.T) {
			for i := 0; i < 10000; i++ {
				_, e := app.db.queryRow("select count(test) from test")
				require.NoError(t, e)
			}
		})
	})
}
