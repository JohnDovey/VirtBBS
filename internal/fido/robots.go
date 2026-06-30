package fido

import "strings"

// IsRobotNetmailMessage reports inbound netmail from a Fido robot or utility
// auto-reply (AreaFix, FileFix, FREQ, PING/PONG, TRACE). Such messages should
// not carry user-style tear lines or Origin footers.
func IsRobotNetmailMessage(pm *Message) bool {
	if pm == nil {
		return false
	}
	fn := strings.TrimSpace(pm.FromName)
	if IsAreaFixResponse(fn) || IsFileFixResponse(fn) || IsFreqRequest(fn) {
		return true
	}
	subj := strings.TrimSpace(pm.Subject)
	if IsPong(subj) || IsTraceReply(subj) {
		return true
	}
	return strings.EqualFold(subj, "AreaFix response") ||
		strings.EqualFold(subj, "FileFix response")
}

// StripRobotNetmailFooter removes tear lines, Origin, and trailing taglines
// from robot netmail body text before local storage or display.
func StripRobotNetmailFooter(body string) string {
	return EchoMainBody(body)
}
