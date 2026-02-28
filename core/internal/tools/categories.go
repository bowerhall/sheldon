package tools

var DangerousTools = map[string]bool{
	"deploy_app": true,
	"remove_app": true,
}

func RequiresApproval(toolName string) bool {
	return DangerousTools[toolName]
}
