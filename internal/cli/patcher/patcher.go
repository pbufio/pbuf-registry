package patcher

type Patcher interface {
	Patch(outputPath, content string) (string, error)
}

func ApplyPatchers(patchers []Patcher, outputPath string, content string) (string, error) {
	for _, patcher := range patchers {
		var err error
		content, err = patcher.Patch(outputPath, content)
		if err != nil {
			return "", err
		}
	}
	return content, nil
}
