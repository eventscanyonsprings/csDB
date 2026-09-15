package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"canyon-springs-residents/store"

	_ "modernc.org/sqlite"
)

func main() {
	dbPath := flag.String("db", "canyon-springs.db", "path to database")
	username := flag.String("user", "", "username")
	password := flag.String("pass", "", "password")
	makeAdmin := flag.Bool("admin", false, "grant admin role")
	flag.Parse()
	if *username == "" || *password == "" {
		flag.Usage()
		os.Exit(1)
	}
	db, err := store.Open(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	defer db.Close()
	exists, _ := store.UserExists(context.Background(), db, *username)
	if exists {
		fmt.Fprintln(os.Stderr, "user already exists:", *username)
		os.Exit(1)
	}
	if err := store.CreateUser(context.Background(), db, *username, *password, *makeAdmin); err != nil {
		fmt.Fprintln(os.Stderr, "create:", err)
		os.Exit(1)
	}
	fmt.Printf("created user %q (admin=%v)\n", *username, *makeAdmin)
}
