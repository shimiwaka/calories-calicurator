package main

import (
	"database/sql"
	"log"
	"net/http"
	"net/http/cgi"
	"os"
	"path"
	"path/filepath"
)

type app struct {
	db    *sql.DB
	dbErr error
}

func main() {
	log.SetPrefix("[calories] ")
	serve := len(os.Args) > 1 && os.Args[1] == "serve"

	root, err := rootDir(serve)
	if err != nil {
		log.Fatal(err)
	}
	a := &app{}
	a.db, a.dbErr = openDB(filepath.Join(root, "db", "calories.db"))

	if serve {
		addr := "localhost:8080"
		if len(os.Args) > 2 {
			addr = os.Args[2]
		}
		mux := http.NewServeMux()
		mux.Handle("/api/", a)
		mux.Handle("/", http.FileServer(http.Dir(root)))
		log.Printf("listening on http://%s/", addr)
		log.Fatal(http.ListenAndServe(addr, mux))
	}

	if err := cgi.Serve(a); err != nil {
		log.Fatal(err)
	}
}

func rootDir(serve bool) (string, error) {
	if serve {
		return os.Getwd()
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(filepath.Dir(exe)), nil
}

func (a *app) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := &ctx{app: a, w: w, r: r}
	if a.dbErr != nil {
		c.fail(a.dbErr)
		return
	}
	var err error
	switch path.Base(r.URL.Path) {
	case "auth.php":
		err = c.auth()
	case "daily.php":
		err = c.daily()
	case "events.php":
		err = c.events()
	case "monthly.php":
		err = c.monthly()
	case "settings.php":
		err = c.settings()
	case "api_settings.php":
		err = c.apiSettings()
	default:
		c.json(http.StatusNotFound, errBody("Not Found"))
	}
	if err != nil {
		c.fail(err)
	}
}
