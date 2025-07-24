package example

//go:generate goverter gen .

// goverter:converter
type Converter interface {
	// goverter:argmap $2 TargetField2
	// goverter:argmap $3 TargetField3
	Convert(source Input, arg2 string, arg3 int) Output
}

type Input struct {
	Field1 string
}

type Output struct {
	Field1       string
	TargetField2 string
	TargetField3 int
}