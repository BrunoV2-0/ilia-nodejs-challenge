package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/ilia/ms-users/internal/database"
	userdb "github.com/ilia/ms-users/internal/database/user"
	domain "github.com/ilia/ms-users/internal/domain/user"
	"github.com/ilia/ms-users/internal/http/client"
	"github.com/ilia/ms-users/internal/http/handler"
	jwtmw "github.com/ilia/ms-users/internal/http/middleware"
	"github.com/jmoiron/sqlx"
)

type config struct {
	jwtKey          string
	transactionsURL string
	internalJwtKey  string
	addr            string
}

func initConfig() config {
	jwtKey := os.Getenv("JWT_KEY")
	if jwtKey == "" {
		log.Fatal("JWT_KEY is required")
	}

	internalJwtKey := os.Getenv("JWT_INTERNAL_KEY")
	if internalJwtKey == "" {
		log.Fatal("JWT_INTERNAL_KEY is required")
	}

	transactionsURL := os.Getenv("TRANSACTIONS_URL")
	if transactionsURL == "" {
		log.Fatal("TRANSACTIONS_URL is required")
	}

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":3002"
	}

	return config{
		jwtKey:          jwtKey,
		internalJwtKey:  internalJwtKey,
		transactionsURL: transactionsURL,
		addr:            addr,
	}
}

func initDatabase() *sqlx.DB {
	db, err := database.Connect()
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	return db
}

func initRouter(svc *domain.Service, cfg config) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	authH := handler.NewAuthHandler(svc, cfg.jwtKey)
	userH := handler.NewUserHandler(svc)

	r.Post("/auth", authH.ServeHTTP)

	userH.PublicRoutes(r)

	r.Group(func(r chi.Router) {
		r.Use(jwtmw.JWT(cfg.jwtKey))
		userH.ProtectedRoutes(r)
	})

	return r
}

func main() {
	cfg := initConfig()

	db := initDatabase()
	defer db.Close()

	if err := database.RunMigrations(db, "internal/database/migrations"); err != nil {
		log.Fatalf("running migrations: %v", err)
	}

	repo := userdb.New(db)
	wallets := client.New(cfg.transactionsURL, cfg.internalJwtKey)
	svc := domain.NewService(repo, wallets)

	r := initRouter(svc, cfg)

	log.Printf("ms-users listening on %s", cfg.addr)
	if err := http.ListenAndServe(cfg.addr, r); err != nil {
		log.Fatalf("server: %v", err)
	}
}
