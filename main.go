package main

import (
	"io"
	"log"

	"sshchat/utils"

	"github.com/gliderlabs/ssh"
)

func main() {
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
		Addr:    ":2222",
		Handler: sessionHandler,
	}
	for _, key := range keys {
		s.AddHostKey(key)
	}

	log.Fatal(s.ListenAndServe())
}
