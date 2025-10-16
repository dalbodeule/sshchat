package main

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"slices"
	"strings"

	"github.com/grafana/loki-client-go/loki"
	slogloki "github.com/samber/slog-loki/v3"
	slogmulti "github.com/samber/slog-multi"

	"sshchat/db"
	"sshchat/utils"

	"github.com/gliderlabs/ssh"
	"github.com/oschwald/geoip2-golang"
	"github.com/uptrace/bun"
)

var config = utils.GetConfig()

func sessionHandler(s ssh.Session, geoip *geoip2.Reader, pgDb *bun.DB, logger *slog.Logger) {
	ptyReq, _, isPty := s.Pty()
	if !isPty {
		_, _ = fmt.Fprintln(s, "Err: PTY requires. Reconnect with -t option.")
		_ = s.Exit(1)
		return
	}

	addr := s.RemoteAddr().String()
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	remote := strings.Trim(host, "[]")
	username := s.User()

	geoStatus := utils.GetIPInfo(remote, geoip)
	if geoStatus == nil {
		logger.Info("[sshchat] connected", "user", username, "remote", remote, "country", "UNK", "status", "FORCE DISCONNECT")
		_, _ = fmt.Fprintf(s, "[system] Your access country is blacklisted. UNK")
		_ = s.Close()
		return
	} else {
		logger.Info("[sshchat] connected", "user", username, "remote", remote, "country", geoStatus.Country)
	}

	if slices.Contains(config.CountryBlacklist, geoStatus.Country) {
		logger.Info("[sshchat] country blacklisted", "user", username, "remote", remote)
		_, _ = fmt.Fprintf(s, "[system] Your access country is blacklisted. %s\n", geoStatus.Country)
		_ = s.Close()
	}

	if geoStatus.Country == "ZZ" {
		if strings.HasPrefix(remote, "127") || strings.HasPrefix(remote, "::1") {
			logger.Info("[sshchat] localhost whitelisted", "user", username)
		} else {
			logger.Info("[sshchat] unknown country blacklisted", "user", username)
			_, _ = fmt.Fprintf(s, "[system] Unknown country is blacklisted. %s\n", geoStatus.Country)
			_ = s.Close()
		}
	}

	client := utils.NewClient(s, ptyReq.Window.Height, ptyReq.Window.Width, username, remote)

	defer func() {
		client.Close()
		logger.Info("[sshchat] disconnected", "user", username, "remote", remote, "country", geoStatus.Country)
	}()

	client.EventLoop()
}

func getLogger(lokiHost string, identify string) (*slog.Logger, error) {
	if lokiHost == "" {
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		logger.Info("Loki host is not set. Logging to stdout")

		return logger, nil
	}

	config, _ := loki.NewDefaultConfig(lokiHost)
	config.TenantID = "sshchat"
	client, err := loki.New(config)
	if err != nil {
		slog.Error("Failed to create Loki client", "error", err)
		return nil, err
	}

	logger := slog.New(
		slogmulti.Fanout(
			slog.NewTextHandler(os.Stdout, nil),
			slogloki.Option{Level: slog.LevelDebug, Client: client}.NewLokiHandler(),
		),
	)
	logger = logger.With(
		slog.String("app", "sshchat"),
		slog.String("identify", identify),
	)
	logger.Info("Logging to Loki", "host", lokiHost)

	return logger, nil
}

func main() {
	logger, err := getLogger(config.LokiHost, config.Identify)
	if err != nil {
		logger.Error("Failed to create logger", "error", err)
		return
	}

	geoip, err := utils.GetDB(config.RootPath + "/" + config.Geoip)
	if err != nil {
		logger.Error("Geoip db is error", "error", err)
		return
	}

	pgDb, err := db.GetDB(config.PgDsn)
	if err != nil {
		log.Fatalf("DB Connection error: %v", err)
	}

	port := config.Port

	keys, err := utils.CheckHostKey(config.RootPath)
	if err != nil {
		logger.Error("Failed to check SSH keys: generate one", "error", err)
		err = utils.GenerateHostKey(config.RootPath)
		if err != nil {
			logger.Error("Fatal error", "error", err)
			return
		}

		keys, err = utils.CheckHostKey(config.RootPath)
		if err != nil {
			log.Fatal(err)
		}
	}

	s := &ssh.Server{
		Addr: ":" + port,
		Handler: func(s ssh.Session) {
			sessionHandler(s, geoip, pgDb, logger)
		},
	}
	for _, key := range keys {
		s.AddHostKey(key)
	}

	defer func() {
		_ = pgDb.Close()
	}()

	logger.Info("Starting server", "port", port)
	if err := s.ListenAndServe(); err != nil {
		logger.Error("Server failed", "error", err)
	}
}
