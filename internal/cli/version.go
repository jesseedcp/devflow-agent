package cli

import (
	"fmt"
	"io"

	"github.com/jesseedcp/devflow-agent/internal/version"
)

func runVersion(stdout io.Writer) error {
	_, err := fmt.Fprintln(stdout, version.Current().String())
	return err
}
