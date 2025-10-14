package utils

import (
	"log"
	"net"

	"github.com/oschwald/geoip2-golang"
)

type IpInfo struct {
	Country       string
	City          string
	Timezone      string
	Isp           string
	IsAnonymousIP bool
}

func GetDB(db string) (*geoip2.Reader, error) {
	geoip, err := geoip2.Open(db)
	if err != nil {
		log.Panicf("Failed to open database: %v", err)
	}

	return geoip, err
}

func GetIPInfo(ip string, db *geoip2.Reader) *IpInfo {
	parsedIp := net.ParseIP(ip)

	country := func(ip net.IP) string {
		country, _ := db.Country(parsedIp)

		println(country.Country.IsoCode)

		if country != nil && country.Country.IsoCode != "" {
			println(country.Country.IsoCode)
			return country.Country.IsoCode
		} else {
			return "ZZ"
		}
	}(parsedIp)
	city, timezone := func(ip net.IP) (string, string) {
		city, _ := db.City(parsedIp)

		if city != nil {
			return city.City.Names["en"], city.Location.TimeZone
		} else {
			return "Unknown", "UTC+0"
		}
	}(parsedIp)
	isp := func(ip net.IP) string {
		isp, _ := db.ISP(parsedIp)

		if isp != nil {
			return isp.ISP
		} else {
			return "Unknown"
		}
	}(parsedIp)
	isAnonymousIP := func(ip net.IP) bool {
		is, _ := db.AnonymousIP(parsedIp)

		if is != nil {
			return is.IsAnonymousVPN ||
				is.IsPublicProxy ||
				is.IsAnonymous
		} else {
			return false
		}
	}(parsedIp)

	return &IpInfo{
		Country:       country,
		City:          city,
		Timezone:      timezone,
		Isp:           isp,
		IsAnonymousIP: isAnonymousIP,
	}
}
