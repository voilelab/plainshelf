package util

import (
	"bufio"
	"io"
)

func LineCount(fp io.Reader) (int, error) {
	reader := bufio.NewReader(fp)
	count := 0
	hasLineContent := false

	for {
		fragment, err := reader.ReadSlice('\n')
		if len(fragment) > 0 {
			hasLineContent = true
		}
		if err == nil {
			count++
			hasLineContent = false
			continue
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err == io.EOF {
			if hasLineContent {
				count++
			}
			return count, nil
		}
		return 0, Errorf("%w", err)
	}
}

func CharCount(fp io.Reader) (int, error) {
	scanner := bufio.NewScanner(fp)
	scanner.Split(bufio.ScanRunes)
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, Errorf("%w", err)
	}
	return count, nil
}
