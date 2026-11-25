package main

import (
	"os"
	"strings"
	"time"
)

func main() {
	_ = os.WriteFile(".fakeagent", []byte(strings.Join(append([]string{os.Getenv("COCKPIT_LEAD")}, os.Args[1:]...), "\n")), 0o644)
	time.Sleep(time.Hour)
}
