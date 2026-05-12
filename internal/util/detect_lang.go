package util

import (
	"bufio"
	"io"
	"strings"
	"unicode"
)

func DetectLanguage(fp io.Reader) (string, error) {
	scanner := bufio.NewScanner(fp)
	scanner.Split(bufio.ScanRunes)

	const maxRunes = 20000
	var runes []rune
	for scanner.Scan() {
		runes = append(runes, []rune(scanner.Text())...)
		if len(runes) >= maxRunes {
			runes = runes[:maxRunes]
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return "", Errorf("%w", err)
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
		case unicode.IsLetter(r):
			latin++
		}
	}

	total := len(runes)
	if total == 0 {
		return "", nil
	}

	if cjk > latin && cjk > total/20 {
		if hantScore > hansScore {
			return "zh-Hant", nil
		}
		if hansScore > hantScore {
			return "zh-Hans", nil
		}
		return "zh-Hant", nil
	}

	if latin > cjk && latin > total/20 {
		return "en", nil
	}

	return "", nil
}

func isCJK(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
}

const hantChars = "後臺萬與為國會時來對開關個這裏著麼說見體點學實發經過還當"
const hansChars = "后台万与为国会时来对开关个这里着么说见体点学实发经过还当"
