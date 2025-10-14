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
}

func GetConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	port := os.Getenv("PORT")
	geoip_dbfile := os.Getenv("GEOIP_DB")
	country_blacklist := os.Getenv("COUNTRY_BLACKLIST")

	return &Config{
		Port:             port,
		Geoip:            geoip_dbfile,
		CountryBlacklist: strings.Split(country_blacklist, ","),
	}
}
