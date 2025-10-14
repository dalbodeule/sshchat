package main

import (
	"fmt"
	"log"
	"slices"
	"strings"

	"sshchat/utils"

	"github.com/gliderlabs/ssh"
	"github.com/oschwald/geoip2-golang"
)

var config = utils.GetConfig()

func sessionHandler(s ssh.Session, geoip *geoip2.Reader) {
	ptyReq, _, isPty := s.Pty()
	if !isPty {
		_, _ = fmt.Fprintln(s, "Err: PTY requires. Reconnect with -t option.")
		_ = s.Exit(1)
		return
	}

	remote := s.RemoteAddr().String()
	username := s.User()

	geoStatus := utils.GetIPInfo(remote, geoip)
	if geoStatus == nil {
		log.Printf("[sshchat] %s connected. %s / UNK [FORCE DISCONNECT]", username, remote)
		_, _ = fmt.Fprintf(s, "[system] Your access country is blacklisted. UNK")
		_ = s.Close()
		return
	} else {
		log.Printf("[sshchat] %s connected. %s / %s", username, remote, geoStatus.Country)
	}

	if slices.Contains(config.CountryBlacklist, geoStatus.Country) {
		log.Printf("[sshchat] %s country blacklisted. %s", username, remote)
		_, _ = fmt.Fprintf(s, "[system] Your access country is blacklisted. %s\n", geoStatus.Country)
		_ = s.Close()
	}

	if geoStatus.Country == "ZZ" && !(strings.HasPrefix(remote, "127") || strings.HasPrefix(remote, "[::1]")) {
		log.Printf("[sshchat] unknown country blacklisted. %s", username)
		_, _ = fmt.Fprintf(s, "[system] Unknown country is blacklisted. %s\n", geoStatus.Country)
		_ = s.Close()
	} else {
		log.Printf("[sshchat] %s is localhost whitelisted.", username)
	}

	client := utils.NewClient(s, ptyReq.Window.Height, ptyReq.Window.Width, username, remote)

	defer func() {
		client.Close()
		log.Printf("[sshchat] %s disconnected. %s / %s", username, remote, geoStatus.Country)
	}()

	client.EventLoop()
}

func main() {
	geoip, err := utils.GetDB(config.Geoip)
	port := config.Port
	if err != nil {
		log.Fatalf("Geoip db is error: %v", err)
	}

	keys, err := utils.CheckHostKey()
	if err != nil {
		log.Print("Failed to check SSH keys: generate one.\n", err)
		err = utils.GenerateHostKey()
		if err != nil {
			log.Fatal(err)
		}

		keys, err = utils.CheckHostKey()
		if err != nil {
			log.Fatal(err)
		}
	}

	s := &ssh.Server{
		Addr: ":" + port,
		Handler: func(s ssh.Session) {
			sessionHandler(s, geoip)
		},
	}
	for _, key := range keys {
		s.AddHostKey(key)
	}

	log.Print("Listening on :" + port)
	log.Fatal(s.ListenAndServe())
}
