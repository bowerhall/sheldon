package router

func Classify(message string) (*Route, error) {
	return &Route{
		PrimaryDomains: []int{1},
		RelatedDomains: []int{},
		ModelTier:      "sonnet",
	}, nil
}
