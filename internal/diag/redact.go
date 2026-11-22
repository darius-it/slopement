package diag

import (
	"regexp"
)

// What is taken out of a line as it is written: nothing here may reach a file,
// let alone a report. Redaction works on the text of a line, so it holds
// whoever wrote it (mautrix's logs, the standard log, the app's own).

var (
	// Matrix access and refresh tokens, and bearer tokens in headers.
	tokenRE  = regexp.MustCompile(`(?i)\b(syt|sy[a-z]{1,3}|mat[a-z]?|mct|mcr|mar|mas)_[A-Za-z0-9_\-]{12,}`)
	bearerRE = regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/\-]+=*`)
	// key=value and "key":"value" of what is secret: tokens, passwords, keys.
	secretJSONRE  = regexp.MustCompile(`(?i)"(access_token|refresh_token|id_token|token|password|new_password|passphrase|client_secret|recovery_key|private_key|session_key|secret|key|code|auth_token)"\s*:\s*"(?:[^"\\]|\\.)*"`)
	secretQueryRE = regexp.MustCompile(`(?i)([?&"'\s](?:access_token|refresh_token|id_token|token|password|client_secret|code|state|recovery_key)=)[^&\s"']+`)
	// What people wrote or named: message text, rooms, topics, people.
	textJSONRE = regexp.MustCompile(`(?i)"(body|formatted_body|topic|name|displayname|display_name|reason|alias|canonical_alias|avatar_url|filename|url)"\s*:\s*"(?:[^"\\]|\\.)*"`)
	// A recovery key as people write it down: groups of four base58.
	recoveryRE = regexp.MustCompile(`\b(?:[A-HJ-NP-Za-km-z1-9]{4}\s){10,11}[A-HJ-NP-Za-km-z1-9]{4}\b`)
	// Matrix identifiers, hashed at export (see hash).
	idRE = regexp.MustCompile(`[!@#$][A-Za-z0-9._=/+\-]{1,255}(?::[A-Za-z0-9.\-]+(?::\d+)?)?`)
)

// Redact removes what must not be logged from p and returns the result.
func Redact(p []byte) []byte {
	out := tokenRE.ReplaceAll(p, []byte("[token]"))
	out = bearerRE.ReplaceAll(out, []byte("${1}[token]"))
	out = secretJSONRE.ReplaceAll(out, []byte(`"$1":"[removed]"`))
	out = secretQueryRE.ReplaceAll(out, []byte("${1}[removed]"))
	out = textJSONRE.ReplaceAll(out, []byte(`"$1":"[removed]"`))
	out = recoveryRE.ReplaceAll(out, []byte("[recovery key]"))
	return out
}

// hash replaces the identifiers in data: user IDs (@) always, room IDs and
// aliases (! #) and event IDs ($) unless the export keeps them. The server
// name stays, since it says which homeserver a problem is with; the rest is a
// short hash, the same for the same ID in every export of this install.
func (h hasher) hash(data []byte) []byte {
	return idRE.ReplaceAllFunc(data, func(m []byte) []byte {
		s := string(m)
		if !h.rooms && (s[0] == '!' || s[0] == '#' || s[0] == '$') || s[0] == '@' {
			local, server := s[1:], ""
			for i := 0; i < len(local); i++ {
				if local[i] == ':' {
					local, server = local[:i], local[i:]
					break
				}
			}
			if len(local) < 3 && server == "" {
				return m // not an identifier: "@a", "$1"
			}
			return []byte(s[:1] + h.short(local) + server)
		}
		return m
	})
}
