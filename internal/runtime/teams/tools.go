// 来源：公众号@小林coding
// 后端八股网站：xiaolincoding.com
// Agent网站：xiaolinnote.com
// 简历模版：jianli.xiaolinnote.com

package teams

import (
	"context"
	"fmt"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/runtime/tools"
)

// SendMessageTool allows agents to send messages to named teammates.
type SendMessageTool struct {
	TeamMgr    *TeamManager
	SenderName string
}

func (t *SendMessageTool) Name() string                 { return "SendMessage" }
func (t *SendMessageTool) Category() tools.ToolCategory { return tools.CategoryCommand }
func (t *SendMessageTool) Description() string {
	return "Send a message to another named agent in the team. The recipient will see it on their next turn."
}

func (t *SendMessageTool) Schema() map[string]any {
	return map[string]any{
		"name":        t.Name(),
		"description": t.Description(),
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"to": map[string]any{
					"type":        "string",
					"description": "Name of the recipient agent",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Message content to send",
				},
			},
			"required": []string{"to", "content"},
		},
	}
}

func (t *SendMessageTool) Execute(ctx context.Context, args map[string]any) tools.ToolResult {
	to, _ := args["to"].(string)
	content, _ := args["content"].(string)
	if to == "" || content == "" {
		return tools.ToolResult{Output: "Error: 'to' and 'content' are required", IsError: true}
	}

	// The lead is not registered as a Member (it lives in the parent
	// process and only reads from its own mailbox), so route to it by
	// finding any team the sender belongs to.
	if to == LeadName {
		for _, teamName := range t.TeamMgr.ListTeams() {
			team := t.TeamMgr.GetTeam(teamName)
			if team == nil {
				continue
			}
			if _, ok := team.Members[t.SenderName]; ok {
				team.SendMessage(t.SenderName, LeadName, content)
				return tools.ToolResult{
					Output: fmt.Sprintf("Message sent to %s.", LeadName),
				}
			}
		}
		return tools.ToolResult{
			Output:  fmt.Sprintf("Error: cannot find team for sender '%s'", t.SenderName),
			IsError: true,
		}
	}

	// Find a registered teammate with this name.
	for _, teamName := range t.TeamMgr.ListTeams() {
		team := t.TeamMgr.GetTeam(teamName)
		if team == nil {
			continue
		}
		if _, ok := team.Members[to]; ok {
			team.SendMessage(t.SenderName, to, content)
			return tools.ToolResult{
				Output: fmt.Sprintf("Message sent to %s.", to),
			}
		}
	}

	return tools.ToolResult{
		Output:  fmt.Sprintf("Error: recipient '%s' not found in any team", to),
		IsError: true,
	}
}

// TeamCreateTool creates a new agent team.
type TeamCreateTool struct {
	TeamMgr *TeamManager
}

func (t *TeamCreateTool) Name() string                 { return "TeamCreate" }
func (t *TeamCreateTool) Category() tools.ToolCategory { return tools.CategoryCommand }
func (t *TeamCreateTool) Description() string {
	return "Create a new agent team for multi-agent collaboration. After creating, use Agent tool with team_name to spawn teammates."
}

func (t *TeamCreateTool) Schema() map[string]any {
	return map[string]any{
		"name":        t.Name(),
		"description": t.Description(),
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"team_name": map[string]any{
					"type":        "string",
					"description": "Name for the team",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "What this team will work on",
				},
			},
			"required": []string{"team_name"},
		},
	}
}

func (t *TeamCreateTool) Execute(ctx context.Context, args map[string]any) tools.ToolResult {
	name, _ := args["team_name"].(string)
	if name == "" {
		return tools.ToolResult{Output: "Error: team_name is required", IsError: true}
	}

	// Deduplicate: if name exists, append suffix
	baseName := name
	for i := 2; t.TeamMgr.GetTeam(name) != nil; i++ {
		name = fmt.Sprintf("%s-%d", baseName, i)
	}

	mode := detectBackend()
	team := t.TeamMgr.CreateTeam(name, mode)

	desc, _ := args["description"].(string)
	return tools.ToolResult{
		Output: fmt.Sprintf("Team \"%s\" created (mode: %s). Use Agent tool with team_name=\"%s\" to add teammates.\nDescription: %s",
			team.Name, team.Mode, team.Name, desc),
	}
}

// TeamDeleteTool deletes an agent team and stops all members.
type TeamDeleteTool struct {
	TeamMgr *TeamManager
}

func (t *TeamDeleteTool) Name() string { return "TeamDelete" }

func (t *TeamDeleteTool) Category() tools.ToolCategory { return tools.CategoryCommand }
func (t *TeamDeleteTool) Description() string {
	return "Delete a team, stopping all its members."
}

func (t *TeamDeleteTool) Schema() map[string]any {
	return map[string]any{
		"name":        t.Name(),
		"description": t.Description(),
		"input_schema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"team_name": map[string]any{
					"type":        "string",
					"description": "Name of the team to delete",
				},
			},
			"required": []string{"team_name"},
		},
	}
}

func (t *TeamDeleteTool) Execute(ctx context.Context, args map[string]any) tools.ToolResult {
	name, _ := args["team_name"].(string)
	if name == "" {
		return tools.ToolResult{Output: "Error: team_name is required", IsError: true}
	}

	team := t.TeamMgr.GetTeam(name)
	if team == nil {
		return tools.ToolResult{
			Output:  fmt.Sprintf("Error: team '%s' not found", name),
			IsError: true,
		}
	}

	memberCount := len(team.Members)
	var memberNames []string
	for n := range team.Members {
		memberNames = append(memberNames, n)
	}

	t.TeamMgr.DeleteTeam(name)
	return tools.ToolResult{
		Output: fmt.Sprintf("Team \"%s\" deleted. Stopped %d member(s): %s", name, memberCount, strings.Join(memberNames, ", ")),
	}
}
