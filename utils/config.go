package utils

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port             string
	Geoip            string
	CountryBlacklist []string
	PgDsn            string
}

func GetConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	port := os.Getenv("PORT")
	geoipDbfile := os.Getenv("GEOIP_DB")
	countryBlacklist := os.Getenv("COUNTRY_BLACKLIST")
	pgDsn := os.Getenv("DB_DSN")

	return &Config{
		Port:             port,
		Geoip:            geoipDbfile,
		CountryBlacklist: strings.Split(countryBlacklist, ","),
		PgDsn:            pgDsn,
	}
}
