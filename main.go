package main

import (
	"fmt"
	"os"

	"github.com/RedHatInsights/valpop/cmd"
)

func main() {

	if err := cmd.Execute(); err != nil {
		fmt.Printf("%s", err)
		os.Exit(127)
	}
}
