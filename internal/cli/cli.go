package cli

import (
	"fmt"
	"io"
)

const helpText = `devflow - backend demand delivery agent

Usage:
  devflow help
  devflow start --title <title> --description <text>

Commands:
  help    Show this help text
  start   Create a new demand workspace
`

func Run(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" {
		_, err := fmt.Fprint(stdout, helpText)
		return err
	}
	switch args[0] {
	case "start":
		return fmt.Errorf("start command requires the artifact store")
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], helpText)
	}
}
