module github.com/jfrog/gocmd

go 1.15

require (
	github.com/jfrog/build-info-go v0.1.0
	github.com/jfrog/gofrog v1.1.0
	github.com/jfrog/jfrog-client-go v1.6.0
	github.com/stretchr/testify v1.7.0
)

replace github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-client-go v1.6.1-0.20211116141412-7dc86e653b1a

replace github.com/jfrog/build-info-go => github.com/jfrog/build-info-go v0.1.1-0.20211116095320-767c1e8a864f

//replace github.com/jfrog/gofrog => github.com/jfrog/gofrog v1.0.7-0.20211109140605-15e312b86c9f
