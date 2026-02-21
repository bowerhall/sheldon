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
}
