// frpdesk — frps's users (each with a token and the ports it may take) as an frps server plugin,
// and a web UI to manage them, export their frpc.toml and watch frps.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"frpdesk/internal/config"
	"frpdesk/internal/server"
	"frpdesk/internal/users"
)

var version = "dev" // set by the build: -ldflags "-X main.version=..."

func main() {
	configPath := flag.String("c", "frpdesk.toml", "the config file")
	showVersion := flag.Bool("v", false, "print the version")
	check := flag.Bool("check", false, "check the config and the users' file, then exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	store, err := users.Load(cfg.UsersFile)
	if err != nil {
		log.Fatal(err)
	}
	if *check {
		fmt.Printf("ok: %d users in %s\n", store.Count(), cfg.UsersFile)
		return
	}
	log.Printf("frpdesk %s: %d users in %s", version, store.Count(), cfg.UsersFile)
	srv, err := server.New(cfg, store, version)
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		srv.Shutdown()
		os.Exit(0)
	}()
	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}
