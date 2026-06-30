package fido

import (
	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/messages"
)

// DefaultAttachmentLimitBytes matches messages.DefaultAttachmentLimitBytes (5 MiB).
const DefaultAttachmentLimitBytes = messages.DefaultAttachmentLimitBytes

// FidoMaxMessageBodyBytes is the FTS-0001 Type-2 message text field limit.
const FidoMaxMessageBodyBytes = 16 * 1024

// NetmailAttachmentLimit returns the max attachment size for netmail on a network.
func NetmailAttachmentLimit(cfg *Config, nd *NetworkDef) int64 {
	if nd != nil && nd.MaxNetmailAttachmentBytes > 0 {
		return int64(nd.MaxNetmailAttachmentBytes)
	}
	if cfg != nil && cfg.MaxNetmailAttachmentBytes > 0 {
		return int64(cfg.MaxNetmailAttachmentBytes)
	}
	return DefaultAttachmentLimitBytes
}

// EchoAttachmentLimit returns the max attachment size for an echo conference.
func EchoAttachmentLimit(conf *conferences.Conference) int64 {
	if conf != nil && conf.MaxAttachmentBytes > 0 {
		return int64(conf.MaxAttachmentBytes)
	}
	return DefaultAttachmentLimitBytes
}
