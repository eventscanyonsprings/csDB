package main

import (
	"context"
	"fmt"
	"os"

	"canyon-springs-residents/store"

	_ "modernc.org/sqlite"
)

func main() {
	path := "canyon-springs.db"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	db, err := store.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	defer db.Close()
	users, err := store.ListLoginHashes(context.Background(), db)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list:", err)
		os.Exit(1)
	}
	if len(users) == 0 {
		fmt.Println("no users")
		return
	}
	for _, u := range users {
		fmt.Printf("%s:%s\n", u.Username, u.PasswordHash)
	}
}
