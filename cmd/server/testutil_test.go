package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/gorm"

	"github.com/owenwexler/gotth-plus/internal/config"
	"github.com/owenwexler/gotth-plus/internal/database"
)

// TestMain moves to the project root, because the server reads static/ and seedData.json relative
// to its working directory.
func TestMain(m *testing.M) {
	if err := os.Chdir(filepath.Join("..", "..")); err != nil {
		fmt.Fprintln(os.Stderr, "chdir to project root:", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// newTestApp builds the real handler (every route behind every middleware) on a throwaway database.
func newTestApp(t *testing.T, env config.Env) (http.Handler, *gorm.DB) {
	t.Helper()

	db, err := database.ConnectDb(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeDB(db) })

	return buildHandler(env, newMux(env, db)), db
}

var devEnv = config.Env{GoEnv: "development", Port: "0"}
