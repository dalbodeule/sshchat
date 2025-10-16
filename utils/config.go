package utils

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port             string
	Geoip            string
	CountryBlacklist []string
	PgDsn            string
	RootPath         string
	LokiHost         string
}

func GetConfig() *Config {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	geoipDbfile := os.Getenv("GEOIP_DB")
	countryBlacklist := os.Getenv("COUNTRY_BLACKLIST")
	pgDsn := os.Getenv("DB_DSN")
	rootPath := os.Getenv("ROOT_PATH")
	lokiHost := os.Getenv("LOKI_HOST")

	return &Config{
		Port:             port,
		Geoip:            geoipDbfile,
		CountryBlacklist: strings.Split(countryBlacklist, ","),
		PgDsn:            pgDsn,
		RootPath:         rootPath,
		LokiHost:         lokiHost,
	}
}
