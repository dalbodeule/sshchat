package main

import (
	"fmt"
	"log"
	"os"

	"sshchat/utils"

	"github.com/gliderlabs/ssh"
	"github.com/joho/godotenv"
)

func sessionHandler(s ssh.Session) {
	ptyReq, _, isPty := s.Pty()
	if !isPty {
		_, _ = fmt.Fprintln(s, "Err: PTY requires. Reconnect with -t option.")
		_ = s.Exit(1)
		return
	}

	remote := s.RemoteAddr().String()
	username := s.User()

	log.Printf("[sshchat] %s connected. %s", username, remote)
	client := utils.NewClient(s, ptyReq.Window.Height, ptyReq.Window.Width, username, remote)

	defer func() {
		client.Close()
		log.Printf("[sshchat] %s disconnected. %s", username, remote)
	}()

	client.EventLoop()
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	port := os.Getenv("PORT")

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
		Addr:    ":" + port,
		Handler: sessionHandler,
	}
	for _, key := range keys {
		s.AddHostKey(key)
	}

	log.Print("Listening on :" + port)
	log.Fatal(s.ListenAndServe())
}
