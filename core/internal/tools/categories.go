package tools

var DangerousTools = map[string]bool{
	"deploy_app":     true,
	"remove_app":     true,
	"browse_session": true,
}

func RequiresApproval(toolName string) bool {
	return DangerousTools[toolName]
}
