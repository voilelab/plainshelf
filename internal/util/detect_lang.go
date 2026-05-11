package util

import "strings"

func DetectLanguage(text string) string {
	const maxRunes = 20000

	runes := []rune(text)
	if len(runes) > maxRunes {
		runes = runes[:maxRunes]
	}

	var cjk, latin int
	var hantScore, hansScore int

	for _, r := range runes {
		switch {
		case isCJK(r):
			cjk++
			if strings.ContainsRune(hantChars, r) {
				hantScore++
			}
			if strings.ContainsRune(hansChars, r) {
				hansScore++
			}
		case r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z':
			latin++
		}
	}

	total := len(runes)
	if total == 0 {
		return ""
	}

	if cjk > latin && cjk > total/20 {
		if hantScore > hansScore {
			return "zh-Hant"
		}
		if hansScore > hantScore {
			return "zh-Hans"
		}
		return "zh-Hant" // 你的使用場景可預設繁中
	}

	if latin > cjk && latin > total/20 {
		return "en"
	}

	return ""
}

func isCJK(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
}

const hantChars = "後臺萬與為國會時來對開關個這裏著麼說見體點學實發經過還當"
const hansChars = "后台万与为国会时来对开关个这里着么说见体点学实发经过还当"
