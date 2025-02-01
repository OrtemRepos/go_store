package main

import "github.com/OrtemRepos/go_store/internal/app"

func main() {
	err := app.Run()
	if err != nil {
		panic(err)
	}
}