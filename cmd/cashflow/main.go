// Command cashflow runs the Cashflow web server.
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/VakZok/Cashflow/internal/web"
)

func main() {
	addr := flag.String("addr", "0.0.0.0:47613", "address to listen on")
	dbPath := flag.String("db", "money_manager.db", "path to the SQLite database file")
	flag.Parse()

	app, err := web.New(web.Config{DBPath: *dbPath})
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = app.Close() }()

	log.Printf("listening on http://%s (db: %s)", *addr, *dbPath)
	log.Fatal(http.ListenAndServe(*addr, app.Handler()))
}
