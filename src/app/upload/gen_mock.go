package upload

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate -o uploadfakes/fake_uploader.go --fake-name UploaderFake . Uploader
