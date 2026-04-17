package plugin

import (
	"database/sql"

	"github.com/go-chi/chi/v5"

	"ad7/internal/middleware"
)

type Plugin interface {
	Register(r chi.Router, db *sql.DB, auth *middleware.Auth)
}
