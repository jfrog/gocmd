module github.com/jfrog/gocmd

go 1.15

require (
	github.com/jfrog/build-info-go v0.1.2
	github.com/jfrog/gofrog v1.1.0
	github.com/jfrog/jfrog-client-go v1.6.1
	github.com/stretchr/testify v1.7.0
)

replace github.com/jfrog/jfrog-client-go => github.com/jfrog/jfrog-client-go v1.6.3-0.20211129155531-7d566d06876a

replace github.com/jfrog/build-info-go => github.com/jfrog/build-info-go v0.1.3-0.20211129083555-15dcf532860b

// replace github.com/jfrog/gofrog => github.com/jfrog/gofrog v1.0.7-0.20211109140605-15e312b86c9f
