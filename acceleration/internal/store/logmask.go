package store

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/nyaruka/phonenumbers"
)

// logMask is one kind of personal data kept out of logs. keys nil means any attribute,
// otherwise only attributes with one of those names. may is a cheap check, with no regexp,
// that text could match, so a line without it skips pattern, as mayHoldCredential does.
type logMask struct {
	keys    []string
	may     func(text string) bool
	pattern *regexp.Regexp
	mask    func(match string) string
}

// logMasks are the personal data masked in every logged text. Operators read these logs and a
// caller's number is personal data, so a new kind is one entry here.
//
// The international number rule also masks a + and 7 or more digits anywhere, such as the
// monotonic m=+1234567.89 of a time.Time printed with %+v, which is harmless. Its separators
// of up to 2 characters can swallow a number right after one ("+15550001101 2 times" masks as
// "+1•••12"), which still hides the number.
var logMasks = []logMask{
	{
		may:     func(text string) bool { return strings.Contains(text, "sip-") },
		pattern: regexp.MustCompile(`sip-(?:\+|%2[Bb])?[0-9]+`),
		mask:    func(string) string { return "phone_caller" },
	},
	{
		may: func(text string) bool {
			return strings.Contains(text, "%2B") || strings.Contains(text, "%2b") || hasPlusDigit(text)
		},
		pattern: regexp.MustCompile(`(?:\+|%2[Bb])[1-9](?:[ ().-]{0,2}[0-9]){6,14}`),
		mask:    maskPhone,
	},
	{
		keys:    []string{"number", "caller", "to", "from"},
		may:     func(text string) bool { return digitCount(text) >= 7 },
		pattern: regexp.MustCompile(`\(?[0-9](?:[ ().-]{0,2}[0-9]){6,14}`),
		mask:    maskNationalNumber,
	},
}

var (
	dateStart = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}`)
	ipv4      = regexp.MustCompile(`^[0-9]{1,3}(\.[0-9]{1,3}){3}$`)
)

// maskNationalNumber leaves a date and an IPv4 address, which a phone attribute such as from
// or to can hold and which look like a number.
func maskNationalNumber(match string) string {
	if dateStart.MatchString(match) || ipv4.MatchString(match) {
		return match
	}
	return maskPhone(match)
}

// maskPhone is number as + and its country calling code, then ••• and its last 2 digits.
func maskPhone(number string) string {
	number = strings.NewReplacer("%2B", "+", "%2b", "+").Replace(number)
	var digits strings.Builder
	for i := 0; i < len(number); i++ {
		if c := number[i]; '0' <= c && c <= '9' || (c == '+' && i == 0) {
			digits.WriteByte(c)
		}
	}
	normalized := digits.String()
	last2 := normalized[max(len(normalized)-2, 0):]
	if !strings.HasPrefix(normalized, "+") {
		return "•••" + last2
	}
	if parsed, err := phonenumbers.Parse(normalized, ""); err == nil {
		return "+" + strconv.Itoa(int(parsed.GetCountryCode())) + "•••" + last2
	}
	return "+•••" + last2
}

// maskLogText is text with every rule of logMasks that applies to key applied, or text itself
// when none matched.
func maskLogText(key, text string) string {
	for _, rule := range logMasks {
		if rule.keys != nil && !slices.Contains(rule.keys, key) {
			continue
		}
		if !rule.may(text) {
			continue
		}
		text = rule.pattern.ReplaceAllStringFunc(text, rule.mask)
	}
	return text
}

func hasPlusDigit(text string) bool {
	for i := 0; i+1 < len(text); i++ {
		if text[i] == '+' && '0' <= text[i+1] && text[i+1] <= '9' {
			return true
		}
	}
	return false
}

func digitCount(text string) int {
	n := 0
	for i := 0; i < len(text); i++ {
		if '0' <= text[i] && text[i] <= '9' {
			n++
		}
	}
	return n
}
