package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"ad7/internal/config"
	"ad7/internal/handler"
	"ad7/internal/middleware"
	"ad7/internal/plugin"
	"ad7/internal/service"
	"ad7/internal/store"
	"ad7/plugins/leaderboard"
	"ad7/plugins/notification"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "config file path")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	st, err := store.New(cfg.DB.DSN())
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer st.Close()

	auth := middleware.NewAuth(cfg.JWT.Secret, cfg.JWT.AdminRole)

	challengeSvc := service.NewChallengeService(st)
	submissionSvc := service.NewSubmissionService(st, st)

	challengeH := handler.NewChallengeHandler(challengeSvc)
	submissionH := handler.NewSubmissionHandler(submissionSvc)

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.Authenticate)

		r.Get("/challenges", challengeH.List)
		r.Get("/challenges/{id}", challengeH.Get)
		r.Post("/challenges/{id}/submit", submissionH.Submit)

		r.Route("/admin", func(r chi.Router) {
			r.Use(auth.RequireAdmin)
			r.Post("/challenges", challengeH.Create)
			r.Put("/challenges/{id}", challengeH.Update)
			r.Delete("/challenges/{id}", challengeH.Delete)
			r.Get("/submissions", submissionH.List)
		})
	})

	plugins := []plugin.Plugin{
		leaderboard.New(),
		notification.New(),
	}
	for _, p := range plugins {
		p.Register(r, st.DB(), auth)
	}

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
