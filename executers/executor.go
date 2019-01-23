package executers

var registeredExecutor *GoExecutor

type GoExecutor interface {
	execute() error
}

func register(executor GoExecutor) {
	registeredExecutor = &executor
}

func GetCompatibleExecutor() GoExecutor {
	return *registeredExecutor
}