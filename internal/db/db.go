package db

import (
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"

	"github.com/geordanr/goallery2/internal/config"
)

// Connect opens and verifies a MySQL connection using the provided DBConfig.
func Connect(cfg config.DBConfig) (*sqlx.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)
	db, err := sqlx.Connect("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("could not connect to MySQL at %s:%d as %s: %w\n  check that the server is running and that the credentials and database name in the config file are correct", cfg.Host, cfg.Port, cfg.User, err)
	}
	return db, nil
}
