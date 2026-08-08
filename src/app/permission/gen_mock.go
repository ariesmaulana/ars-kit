package permission

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate -o fakes/fake_service.go --fake-name ServiceFake . Service
