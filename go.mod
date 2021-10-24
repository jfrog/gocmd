module github.com/jfrog/gocmd

go 1.15

require (
	github.com/jfrog/build-info-go v0.0.0-20211020140610-2b15ac5444b5
	github.com/jfrog/gofrog v1.0.7
	github.com/jfrog/jfrog-client-go v1.5.0
	github.com/stretchr/testify v1.7.0
)

//replace github.com/jfrog/jfrog-client-go => ../jfrog-client-go

//replace github.com/jfrog/build-info-go => ../build-info-go
