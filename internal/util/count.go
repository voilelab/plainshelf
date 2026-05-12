package util

import (
	"bufio"
	"io"
)

func LineCount(fp io.Reader) (int, error) {
	scanner := bufio.NewScanner(fp)
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, Errorf("%w", err)
	}
	return count, nil
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
