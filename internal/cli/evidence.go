package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/demandflow"
	evidenceadapter "github.com/jesseedcp/devflow-agent/internal/evidence"
)

func runEvidence(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("evidence requires a subcommand: add, fetch, or list")
	}
	switch args[0] {
	case "add":
		return runEvidenceAdd(args[1:], stdout, stderr)
	case "fetch":
		return runEvidenceFetch(args[1:], stdout, stderr)
	case "list":
		return runEvidenceList(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown evidence command %q", args[0])
	}
}

func runEvidenceAdd(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("evidence add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, demandID, evidenceType, criterion, status, summary, link, by string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&evidenceType, "type", "", "evidence type: api, log, monitor, manual, link")
	fs.StringVar(&criterion, "criterion", "", "acceptance criterion or business rule")
	fs.StringVar(&status, "status", "pass", "evidence status: pass, fail, blocked")
	fs.StringVar(&summary, "summary", "", "evidence summary")
	fs.StringVar(&link, "link", "", "optional URL/path/reference")
	fs.StringVar(&by, "by", "", "recorder")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root = normalizedRoot(root)
	demandID = strings.TrimSpace(demandID)
	if demandID == "" {
		return fmt.Errorf("--demand is required")
	}
	record, err := demandflow.AddEvidence(demandflow.AddEvidenceOptions{
		Root:      root,
		DemandID:  demandID,
		Type:      evidenceType,
		Criterion: criterion,
		Status:    status,
		Summary:   summary,
		Link:      link,
		By:        by,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "evidence recorded for %s: %s %s\n", demandID, strings.ToUpper(record.Status), record.Type)
	return nil
}

type repeatedStringFlag []string

func (f *repeatedStringFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *repeatedStringFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func runEvidenceFetch(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("evidence fetch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, demandID, evidenceType, criterion, method, targetURL, body, expectContains string
	var expectStatus int
	var headers repeatedStringFlag
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&evidenceType, "type", "", "evidence fetch type: api or link")
	fs.StringVar(&criterion, "criterion", "", "acceptance criterion or business rule")
	fs.StringVar(&method, "method", "GET", "HTTP method for api evidence")
	fs.StringVar(&targetURL, "url", "", "URL to fetch")
	fs.Var(&headers, "header", "HTTP header, repeatable, as Name: value")
	fs.StringVar(&body, "body", "", "HTTP request body for api evidence")
	fs.IntVar(&expectStatus, "expect-status", 200, "expected HTTP status")
	fs.StringVar(&expectContains, "expect-contains", "", "expected response body substring for api evidence")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root = normalizedRoot(root)
	demandID = strings.TrimSpace(demandID)
	evidenceType = strings.ToLower(strings.TrimSpace(evidenceType))
	if demandID == "" {
		return fmt.Errorf("--demand is required")
	}
	if criterion = strings.Join(strings.Fields(criterion), " "); criterion == "" {
		return fmt.Errorf("--criterion is required")
	}
	if strings.TrimSpace(targetURL) == "" {
		return fmt.Errorf("--url is required")
	}
	var result evidenceadapter.FetchResult
	switch evidenceType {
	case "api":
		result = evidenceadapter.HTTPFetcher{}.Fetch(context.Background(), evidenceadapter.HTTPFetchRequest{
			Method:         method,
			URL:            targetURL,
			Headers:        []string(headers),
			Body:           body,
			ExpectStatus:   expectStatus,
			ExpectContains: expectContains,
			Timeout:        10 * time.Second,
		})
	case "link":
		result = evidenceadapter.LinkFetcher{}.Fetch(context.Background(), evidenceadapter.LinkFetchRequest{URL: targetURL, ExpectStatus: expectStatus, Timeout: 10 * time.Second})
	default:
		return fmt.Errorf("--type must be one of api, link")
	}
	link := result.URL
	summary := result.Summary
	if result.ResponseExcerpt != "" {
		summary += " response=" + result.ResponseExcerpt
	}
	record, err := demandflow.AddEvidence(demandflow.AddEvidenceOptions{
		Root:           root,
		DemandID:       demandID,
		Type:           evidenceType,
		Criterion:      criterion,
		Status:         result.Status,
		Summary:        summary,
		Link:           link,
		Source:         "fetch",
		Method:         result.Method,
		URL:            result.URL,
		ExpectedStatus: strconv.Itoa(expectStatus),
		ActualStatus:   strconv.Itoa(result.ActualStatus),
		ExpectContains: expectContains,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "evidence fetched for %s: %s %s\n", demandID, strings.ToUpper(record.Status), record.Type)
	if record.Status != "pass" {
		return fmt.Errorf("evidence fetch recorded %s", record.Status)
	}
	return nil
}

func runEvidenceList(args []string, stdout io.Writer, stderr io.Writer) error {
	opts, err := parseDemandLookupArgs("evidence list", args)
	if err != nil {
		return err
	}
	records, err := demandflow.ListEvidence(opts.root, opts.demandID)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Acceptance evidence: %s\n", opts.demandID)
	if len(records) == 0 {
		fmt.Fprintln(stdout, "  none")
		return nil
	}
	for _, record := range records {
		fmt.Fprintf(stdout, "  %s %s %s\n", strings.ToUpper(record.Status), record.Type, record.Criterion)
		if record.Summary != "" {
			fmt.Fprintf(stdout, "    %s\n", record.Summary)
		}
		if record.Link != "" {
			fmt.Fprintf(stdout, "    link: %s\n", record.Link)
		}
		if record.By != "" {
			fmt.Fprintf(stdout, "    by: %s\n", record.By)
		}
	}
	return nil
}

func normalizedRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return "."
	}
	return root
}
