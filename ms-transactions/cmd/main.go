package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/ilia/ms-transactions/internal/database"
	walletdb "github.com/ilia/ms-transactions/internal/database/wallet"
	"github.com/ilia/ms-transactions/internal/domain/wallet"
	"github.com/ilia/ms-transactions/internal/http/handler"
	jwtmw "github.com/ilia/ms-transactions/internal/http/middleware"
	"github.com/jmoiron/sqlx"
)

type config struct {
	jwtKey string
	addr   string
}

type app struct {
	db     *sqlx.DB
	router *chi.Mux
}

func initConfig() config {
	jwtKey := os.Getenv("JWT_KEY")
	if jwtKey == "" {
		log.Fatal("JWT_KEY environment variable is required")
	}
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "3001"
	}
	return config{jwtKey: jwtKey, addr: ":" + addr}
}

func initDatabase() *sqlx.DB {
	db, err := database.Connect()
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}

	// This auto-migration step is here to facilitate app validation.
	// In production, migrations would be applied by a dedicated tool
	// under a separate database user before the app starts.
	if err := database.RunMigrations(db, "internal/database/migrations"); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	return db
}

func initRouter(db *sqlx.DB, cfg config) *chi.Mux {
	repo := walletdb.New(db)
	svc := wallet.NewService(repo)
	h := handler.New(svc)

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(jwtmw.JWT(cfg.jwtKey))
	h.Routes(r)

	return r
}

func main() {
	cfg := initConfig()

	a := app{
		db: initDatabase(),
	}
	defer a.db.Close()

	a.router = initRouter(a.db, cfg)

	log.Printf("listening on %s", cfg.addr)
	log.Fatal(http.ListenAndServe(cfg.addr, a.router))
}
