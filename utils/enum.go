package utils

const (
	noTidy                 = "no-tidy"
	recursiveTidy          = "recursive-tidy"
	recursiveTidyOverwrite = "recursive-tidy-overwrite"
)

type TidyEnum struct {
	tidyValue string
}

func (te *TidyEnum) SetNoTidy() {
	te.tidyValue = noTidy
}

func (te *TidyEnum) SetRecursiveTidy() {
	te.tidyValue = recursiveTidy
}

func (te *TidyEnum) SetRecursiveTidyOverwrite() {
	te.tidyValue = recursiveTidyOverwrite
}

func (te *TidyEnum) GetNoTidy() string {
	return noTidy
}

func (te *TidyEnum) GetRecursiveTidy() string {
	return recursiveTidy
}

func (te *TidyEnum) GetRecursiveTidyOverwrite() string {
	return recursiveTidyOverwrite
}

func (te *TidyEnum) GetTidyValue() string {
	return te.tidyValue
}