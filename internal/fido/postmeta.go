package fido

import (
	"database/sql"

	"github.com/virtbbs/virtbbs/internal/conferences"
	"github.com/virtbbs/virtbbs/internal/messages"
)

// ApplyLocalEchoMeta assigns MSGID, REPLY, echo flag, origin kludges (LANG, TZUTC),
// and a random tagline for locally posted echomail. orig must be the sending node's
// address; pass Addr{} to skip when FidoNet is disabled or unconfigured.
func ApplyLocalEchoMeta(m *messages.Message, conf *conferences.Conference, orig Addr, lang string, replyTo *messages.Message, db *sql.DB, cfg *Config) {
	if m == nil || orig == (Addr{}) {
		return
	}
	if conf != nil && conf.Echo {
		m.Echo = true
	}
	m.FidoMsgID = FormatMSGID(orig, NewMSGIDSerial())
	m.FidoKludges = MergeOriginKludges(m.FidoKludges, lang)
	if replyTo != nil && replyTo.FidoMsgID != "" {
		m.FidoReply = replyTo.FidoMsgID
	}
	if conf != nil && conf.Echo {
		AppendEchoTagline(m, db, ResolveTaglinesPath(cfg, conf))
	}
}
