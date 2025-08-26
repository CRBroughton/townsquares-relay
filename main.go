package main

import (
	"os"

	"github.crom/crbroughton/townsquares-relay/cmd"
)

func main() {
	// Backward compatibility: if arguments are provided and first arg looks like a config file,
	// prepend "serve" to maintain existing behavior
	if len(os.Args) > 1 && !isKnownCommand(os.Args[1]) {
		// Insert "serve" as the first argument
		newArgs := make([]string, len(os.Args)+1)
		newArgs[0] = os.Args[0]
		newArgs[1] = "serve"
		copy(newArgs[2:], os.Args[1:])
		os.Args = newArgs
	}
	
	cmd.Execute()
}

func isKnownCommand(arg string) bool {
	knownCommands := []string{"serve", "tailscale", "auth", "help", "--help", "-h", "--version", "-v"}
	for _, cmd := range knownCommands {
		if arg == cmd {
			return true
		}
	}
	return false
}
