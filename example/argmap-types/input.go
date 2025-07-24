package example

//go:generate goverter gen .

// goverter:converter
type Converter interface {
	// goverter:argmap $2 Count
	// goverter:argmap $3 IsActive
	// goverter:argmap $4 Score
	ConvertWithTypes(source Input, count int32, active bool, score float64) Output
}

type Input struct {
	Name string
}

type Output struct {
	Name     string
	Count    int32
	IsActive bool
	Score    float64
}