package email

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate -o emailfakes/fake_sender.go --fake-name SenderFake . EmailSender
