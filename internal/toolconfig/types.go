package toolconfig

type ToolName string

const (
	ToolClaudeCode ToolName = "claude-code"
	ToolCursor     ToolName = "cursor"
	ToolWindsurf   ToolName = "windsurf"
	ToolGemini     ToolName = "gemini"
)

type ToolStatus string

const (
	StatusConfigured    ToolStatus = "configured"
	StatusPartial       ToolStatus = "partial"
	StatusNotConfigured ToolStatus = "not-configured"
)

type Tool struct {
	Name   ToolName
	Label  string
	Status ToolStatus
}

var AllTools = []Tool{
	{Name: ToolClaudeCode, Label: "Claude Code"},
	{Name: ToolCursor, Label: "Cursor"},
	{Name: ToolWindsurf, Label: "Windsurf"},
	{Name: ToolGemini, Label: "Gemini"},
}

func ToolByName(name string) *Tool {
	for i := range AllTools {
		if string(AllTools[i].Name) == name {
			return &AllTools[i]
		}
	}
	return nil
}

func ValidToolNames() []string {
	names := make([]string, len(AllTools))
	for i, t := range AllTools {
		names[i] = string(t.Name)
	}
	return names
}
