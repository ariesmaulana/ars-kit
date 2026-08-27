# Notification Module

Foundation module for sending notifications. Currently supports email
only; SMS, push, and webhook go as sibling packages under
`notification/`.

## Adding a new channel

1. Create a sibling package: `notification/<channel>/`.
2. Define a `Sender` interface with a `Send(ctx, Message) error` method.
3. Add provider implementations in separate files.
4. Add a factory in `factory.go` that switches on provider.
5. Add config fields to `config.Config` and env vars in `.env.example`.
6. Wire in `buildApp()` — add a field to the `App` struct.

Keep each channel independent — no shared abstraction across channels.
