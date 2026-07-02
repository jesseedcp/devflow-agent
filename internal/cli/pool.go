package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/platform"
	platformfeishu "github.com/jesseedcp/devflow-agent/internal/platform/feishu"
)

func runPool(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("pool subcommand is required")
	}
	switch args[0] {
	case "list":
		return runPoolList(args[1:], stdout)
	default:
		return fmt.Errorf("unknown pool subcommand %q", args[0])
	}
}

func runPoolList(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("pool list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var appToken, tableID, baseURL, appID, appSecret string
	fs.StringVar(&appToken, "feishu-bitable", "", "Feishu Bitable app token")
	fs.StringVar(&tableID, "table", "", "Feishu Bitable table id")
	fs.StringVar(&baseURL, "feishu-base-url", "", "Feishu OpenAPI base URL override")
	fs.StringVar(&appID, "feishu-app-id", "", "Feishu app id override")
	fs.StringVar(&appSecret, "feishu-app-secret", "", "Feishu app secret override")
	if err := fs.Parse(args); err != nil {
		return err
	}
	adapter := platformfeishu.BitableAdapter{
		BaseURL: strings.TrimSpace(baseURL),
		TokenClient: &platformfeishu.TenantTokenClient{
			BaseURL:   strings.TrimSpace(baseURL),
			AppID:     strings.TrimSpace(appID),
			AppSecret: strings.TrimSpace(appSecret),
		},
	}
	demands, err := adapter.ListDemands(context.Background(), platform.IntakeRef{AppToken: strings.TrimSpace(appToken), TableID: strings.TrimSpace(tableID)})
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, "record-id\tstatus\ttitle")
	for _, demand := range demands {
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", demand.ID, demand.Status, demand.Title)
	}
	return nil
}
