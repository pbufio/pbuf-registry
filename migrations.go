package main

import (
	"database/sql"

	_ "github.com/lib/pq"
	"github.com/pbufio/pbuf-registry/internal/config"
	"github.com/pbufio/pbuf-registry/migrations"
)

func main() {
	config.NewLoader().MustLoad()

	db, err := sql.Open("postgres", config.Cfg.Data.Database.DSN)
	if err != nil {
		panic(err)
	}
	migrations.Migrate(db)
}
