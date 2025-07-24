package example

//go:generate goverter gen .

// goverter:converter
type Converter interface {
	// goverter:argmap $2 Status
	// goverter:argmap $3 Priority
	// goverter:map Name Title
	ConvertTask(source Task, status string, priority int, ctx Context) TaskOutput
}

type Task struct {
	ID   int
	Name string
}

type TaskOutput struct {
	ID       int
	Title    string
	Status   string
	Priority int
}

type Context struct {
	UserID string
}