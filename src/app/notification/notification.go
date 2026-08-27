// Package notification provides a pluggable notification foundation for ars-kit.
//
// Currently only email is supported (src/app/notification/email/).
// Future channels (SMS, push, webhook) go as sibling packages:
//
//	notification/
//	  email/     ← implemented
//	  sms/       ← when needed
//	  push/      ← when needed
//
// Each channel owns its own Sender interface — no shared abstraction across
// channels. Callers declare exactly what they need (e.g. email.EmailSender),
// keeping the dependency graph explicit.
//
// See README.md for a guide on adding a new channel.
package notification
