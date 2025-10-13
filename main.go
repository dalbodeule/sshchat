package main

import (
	"io"
	"log"
	"os"

	"sshchat/utils"

	"github.com/gliderlabs/ssh"
	"github.com/joho/godotenv"
)

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

	sessionHandler := func(s ssh.Session) {
		_, _ = io.WriteString(s, "Hello World\n")
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
