package main

import (
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"log"
	"os"
	"strings"
)

func main() {
	m, err := migrate.New("file://sql/migrations", strings.Replace(os.Getenv("DATABASE_URL"), "postgresql://", "pgx5://", 1))
	if err != nil {
		log.Fatal(err)
	}
	err = m.Up()
	if err != nil {
		log.Fatal(err)
	}
}
