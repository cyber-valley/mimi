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
	url := strings.Replace(os.Getenv("DATABASE_URL"), "postgresql://", "pgx5://", 1)
	m, err := migrate.New("file://sql/migrations", url)
	if err != nil {
		log.Fatal(err)
	}
	err = m.Up()
	if err != nil {
		log.Fatal(err)
	}
}
