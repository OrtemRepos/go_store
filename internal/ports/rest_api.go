package ports

type RestAPI interface {
	Serve()
	Stop() error
}
