package fido

import (
	"fmt"

	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/fido/uuencode"
	"github.com/virtbbs/virtbbs/internal/messages"
)

// ExtractInboundAttachments decodes uuencode blocks from text, returning a
// clean body and attachment inputs. Files over maxBytes are rejected.
func ExtractInboundAttachments(text string, maxBytes int64) (clean string, files []messages.AttachmentInput, err error) {
	decoded, cleanBody, err := uuencode.Decode(text)
	if err != nil {
		return text, nil, err
	}
	if len(decoded) == 0 {
		return text, nil, nil
	}
	if maxBytes <= 0 {
		maxBytes = DefaultAttachmentLimitBytes
	}
	for _, f := range decoded {
		if int64(len(f.Data)) > maxBytes {
			return text, nil, fmt.Errorf("attachment %q exceeds size limit (%d bytes, max %d)", f.Filename, len(f.Data), maxBytes)
		}
		files = append(files, messages.AttachmentInput{
			Filename: f.Filename,
			Data:     f.Data,
		})
	}
	return cleanBody, files, nil
}

// SaveInboundAttachments stores extracted files for a posted message.
func SaveInboundAttachments(store *messages.Store, root string, messageID int64, files []messages.AttachmentInput, maxBytes int64) error {
	if len(files) == 0 {
		return nil
	}
	return store.SaveAttachments(root, messageID, files, maxBytes)
}

// BodyWithAttachments appends uuencoded attachments to a message body for
// Fido export. Returns body unchanged if attachments do not fit in maxBytes.
func BodyWithAttachments(body string, files []messages.AttachmentInput, maxBytes int) string {
	if len(files) == 0 {
		return body
	}
	if maxBytes <= 0 {
		maxBytes = FidoMaxMessageBodyBytes
	}
	var decoded []uuencode.DecodedFile
	for _, f := range files {
		decoded = append(decoded, uuencode.DecodedFile{
			Filename: f.Filename,
			Data:     f.Data,
		})
	}
	with := uuencode.AppendToBody(body, decoded)
	if len(with) > maxBytes {
		return body
	}
	return with
}

// InboundAttachmentLimit picks netmail vs echo limit for a tossed message.
func InboundAttachmentLimit(cfg *Config, nd *NetworkDef, confStore *conferences.Store, confID int, isNetmail bool) int64 {
	if isNetmail {
		return NetmailAttachmentLimit(cfg, nd)
	}
	if confStore != nil && confID > 0 {
		if c, err := confStore.Get(confID); err == nil && c != nil {
			return EchoAttachmentLimit(c)
		}
	}
	return DefaultAttachmentLimitBytes
}
