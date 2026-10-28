package app

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func (a *App) messageText(key string, values map[string]any) string {
	locale := a.locale
	if locale == "" {
		locale = a.cfg.Locale
	}
	return formatMessage(a.t(key), locale, values)
}

// The upstream catalogs use named substitutions and ICU cardinal plurals.
// Parse balanced branches so translations can include nested substitutions.
func formatMessage(pattern, locale string, values map[string]any) string {
	var result strings.Builder
	for len(pattern) > 0 {
		start := strings.IndexByte(pattern, '{')
		if start < 0 {
			result.WriteString(pattern)
			break
		}
		result.WriteString(pattern[:start])
		end := messageBraceEnd(pattern, start)
		if end < 0 {
			result.WriteString(pattern[start:])
			break
		}
		body := pattern[start+1 : end]
		parts := strings.SplitN(body, ",", 3)
		key := strings.TrimSpace(parts[0])
		value, exists := values[key]
		if !exists {
			result.WriteString(pattern[start : end+1])
		} else if len(parts) == 3 && strings.TrimSpace(parts[1]) == "plural" {
			n, _ := strconv.ParseFloat(fmt.Sprint(value), 64)
			branches := map[string]string{}
			rest := strings.TrimSpace(parts[2])
			for len(rest) > 0 {
				at := strings.IndexByte(rest, '{')
				if at < 0 {
					break
				}
				close := messageBraceEnd(rest, at)
				if close < 0 {
					break
				}
				branches[strings.TrimSpace(rest[:at])] = rest[at+1 : close]
				rest = strings.TrimSpace(rest[close+1:])
			}
			// Flutter Intl generates =0/=1/=2 as the zero/one/two
			// parameters, which also participate in locale rule selection.
			for index, category := range []string{"zero", "one", "two"} {
				if explicit, ok := branches["="+strconv.Itoa(index)]; ok {
					branches[category] = explicit
				}
			}
			branch, ok := branches["="+strconv.FormatFloat(n, 'f', -1, 64)]
			if !ok && n >= 0 && n <= 2 && n == math.Trunc(n) {
				branch, ok = branches[[]string{"zero", "one", "two"}[int(n)]]
			}
			if !ok {
				branch, ok = branches[messagePlural(locale, n)]
			}
			if !ok {
				branch = branches["other"]
			}
			result.WriteString(formatMessage(strings.ReplaceAll(branch, "#", fmt.Sprint(value)), locale, values))
		} else {
			result.WriteString(fmt.Sprint(value))
		}
		pattern = pattern[end+1:]
	}
	return result.String()
}

func messageBraceEnd(text string, start int) int {
	depth := 0
	for i := start; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func messagePlural(locale string, n float64) string {
	// These seven rules follow the source's Dart Intl 0.20.3 evaluation,
	// including its rounded integer operand and explicit-number behavior.
	i := int64(math.Round(n))
	integer := n == math.Trunc(n)
	language := strings.Split(strings.ReplaceAll(locale, "_", "-"), "-")[0]
	switch language {
	case "ja", "ko", "zh":
		return "other"
	case "ru":
		if !integer {
			return "other"
		}
		mod10, mod100 := (i%10+10)%10, (i%100+100)%100
		if mod10 == 1 && mod100 != 11 {
			return "one"
		}
		if mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14) {
			return "few"
		}
		return "many"
	case "fr":
		if i == 0 || i == 1 {
			return "one"
		}
		if i != 0 && i%1000000 == 0 && integer {
			return "many"
		}
	case "es":
		if n == 1 {
			return "one"
		}
		if i != 0 && i%1000000 == 0 && integer {
			return "many"
		}
	default:
		if n == 1 {
			return "one"
		}
	}
	return "other"
}
