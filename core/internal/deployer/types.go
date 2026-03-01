package deployer

type BuildResult struct {
	ImageName string
	ImageTag  string
	Size      int64
	Duration  string
}

type DeployResult struct {
	Resources []string
	Status    string
	URL       string // full URL to access the app (e.g., http://1.2.3.4:8080 or https://app.example.com)
	Port      int    // exposed port (for IP-only deployments)
}
