package background

type Daemon interface {
	Name() string
	Run() error
}
