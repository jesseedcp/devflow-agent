package cli

import (
	"bytes"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jesseedcp/devflow-agent/internal/demandflow"
)

type workbenchDemand struct {
	ID        string
	State     string
	Attention string
}

type workbenchModel struct {
	opts       workbenchOptions
	demands    []workbenchDemand
	selected   int
	message    string
	showDetail bool
	detail     demandflow.ConsoleSummary
	detailErr  error
}

type workbenchLoadedMsg struct {
	demands []workbenchDemand
	err     error
}

var workbenchRunNext = func(opts workbenchOptions, demandID string) string {
	summary, err := demandflow.InspectConsole(opts.root, demandID)
	if err != nil {
		return "Blocked: " + err.Error()
	}
	if !summary.PrimaryAction.Runnable {
		if summary.PrimaryAction.BlockReason != "" {
			return "Blocked: " + summary.PrimaryAction.BlockReason
		}
		return "Blocked: action is not runner-safe"
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err = runConsoleNext(consoleArgs{root: opts.root, demandID: demandID, configPath: opts.configPath, qualityCommand: opts.qualityCommand}, &stdout, &stderr)
	if err != nil {
		return "Blocked: " + err.Error()
	}
	return strings.TrimSpace(stdout.String())
}

var workbenchDrive = func(opts workbenchOptions, demandID string) string {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	args := []string{"--root", opts.root, "--demand", demandID}
	if opts.configPath != "" {
		args = append(args, "--config", opts.configPath)
	}
	for _, command := range opts.qualityCommand {
		args = append(args, "--quality-command", command)
	}
	if err := runDrive(args, &stdout, &stderr); err != nil {
		return "Drive failed: " + err.Error()
	}
	return strings.TrimSpace(stdout.String())
}

var workbenchEvaluate = func(opts workbenchOptions, demandID string) string {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runEvaluate([]string{"--root", opts.root, "--demand", demandID}, &stdout, &stderr); err != nil {
		return "Evaluate failed: " + err.Error()
	}
	return strings.TrimSpace(stdout.String())
}

func newWorkbenchModel(opts workbenchOptions) workbenchModel {
	return workbenchModel{opts: opts, message: "Loading demands..."}
}

func (m workbenchModel) Init() tea.Cmd {
	return m.loadDemands
}

func (m workbenchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case workbenchLoadedMsg:
		m.demands = msg.demands
		m.detailErr = msg.err
		if msg.err != nil {
			m.message = msg.err.Error()
		} else {
			m.message = fmt.Sprintf("%d demands", len(m.demands))
		}
		m.clampSelection()
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.demands)-1 {
				m.selected++
			}
		case "enter":
			m.showDetail = !m.showDetail
			m.loadSelectedDetail()
		case "r":
			m.message = m.runSelected(workbenchRunNext)
			return m, m.loadDemands
		case "d":
			m.message = m.runSelected(workbenchDrive)
			return m, m.loadDemands
		case "e":
			m.message = m.runSelected(workbenchEvaluate)
			return m, m.loadDemands
		case "R", "ctrl+r":
			m.message = "Refreshing demands..."
			return m, m.loadDemands
		}
	}
	return m, nil
}

func (m workbenchModel) View() string {
	var builder strings.Builder
	builder.WriteString("Devflow Workbench\n\n")
	if len(m.demands) == 0 {
		builder.WriteString("No demands found\n")
	} else {
		for index, demand := range m.demands {
			cursor := " "
			if index == m.selected {
				cursor = ">"
			}
			fmt.Fprintf(&builder, "%s %-24s %-22s %s\n", cursor, demand.ID, demand.State, demand.Attention)
		}
	}
	if m.showDetail {
		m.renderDetail(&builder)
	}
	if m.message != "" {
		fmt.Fprintf(&builder, "\n%s\n", m.message)
	}
	builder.WriteString("\n↑/↓ select · enter detail · r run · d drive · e evaluate · R refresh · q quit\n")
	return builder.String()
}

func (m workbenchModel) loadDemands() tea.Msg {
	summaries, err := demandflow.ListConsole(m.opts.root)
	if err != nil {
		return workbenchLoadedMsg{err: err}
	}
	demands := make([]workbenchDemand, 0, len(summaries))
	for _, summary := range summaries {
		demands = append(demands, workbenchDemand{ID: summary.Workspace.Demand.ID, State: string(summary.Workspace.State), Attention: summary.Workspace.Attention})
	}
	return workbenchLoadedMsg{demands: demands}
}

func (m *workbenchModel) clampSelection() {
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(m.demands) {
		m.selected = len(m.demands) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func (m *workbenchModel) loadSelectedDetail() {
	if len(m.demands) == 0 {
		return
	}
	summary, err := demandflow.InspectConsole(m.opts.root, m.demands[m.selected].ID)
	m.detail = summary
	m.detailErr = err
}

func (m workbenchModel) selectedDemandID() string {
	if len(m.demands) == 0 || m.selected < 0 || m.selected >= len(m.demands) {
		return ""
	}
	return m.demands[m.selected].ID
}

func (m workbenchModel) runSelected(fn func(workbenchOptions, string) string) string {
	demandID := m.selectedDemandID()
	if demandID == "" {
		return "Blocked: no demand selected"
	}
	return fn(m.opts, demandID)
}

func (m workbenchModel) renderDetail(builder *strings.Builder) {
	builder.WriteString("\nSummary\n")
	if m.detailErr != nil {
		fmt.Fprintf(builder, "unavailable: %v\n", m.detailErr)
		return
	}
	summary := m.detail
	if summary.Workspace.Demand.ID == "" && len(m.demands) > 0 {
		summary, m.detailErr = demandflow.InspectConsole(m.opts.root, m.demands[m.selected].ID)
		if m.detailErr != nil {
			fmt.Fprintf(builder, "unavailable: %v\n", m.detailErr)
			return
		}
	}
	fmt.Fprintf(builder, "State: %s\n", summary.Workspace.State)
	fmt.Fprintf(builder, "Attention: %s\n", summary.Workspace.Attention)
	builder.WriteString("Next:\n")
	renderWorkbenchAction(builder, summary.PrimaryAction)
	builder.WriteString("Run-ready:\n")
	if summary.RunReadyAction.Runnable {
		renderWorkbenchAction(builder, summary.RunReadyAction)
	} else {
		fmt.Fprintf(builder, "  %s\n", summary.RunReadyAction.BlockReason)
	}
}

func renderWorkbenchAction(builder *strings.Builder, action demandflow.ConsoleAction) {
	if action.Label != "" {
		fmt.Fprintf(builder, "  %s\n", action.Label)
	}
	if action.Command != "" {
		fmt.Fprintf(builder, "  %s\n", action.Command)
	}
	if action.BlockReason != "" && !action.Runnable {
		fmt.Fprintf(builder, "  blocked: %s\n", action.BlockReason)
	}
}
