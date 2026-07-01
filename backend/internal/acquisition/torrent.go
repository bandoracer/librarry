package acquisition

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

func torrentInfoHashV1(data []byte) (string, error) {
	start, end, err := torrentInfoSpan(data)
	if err != nil {
		return "", err
	}
	sum := sha1.Sum(data[start:end])
	return hex.EncodeToString(sum[:]), nil
}

func normalizeTorrentHash(value string) string {
	value = strings.TrimSpace(value)
	if len(value) != 40 {
		return value
	}
	for _, r := range value {
		if r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F' {
			continue
		}
		return value
	}
	return strings.ToLower(value)
}

func torrentInfoSpan(data []byte) (int, int, error) {
	if len(data) == 0 || data[0] != 'd' {
		return 0, 0, errors.New("torrent metadata must be a bencoded dictionary")
	}
	pos := 1
	for pos < len(data) {
		if data[pos] == 'e' {
			return 0, 0, errors.New("torrent metadata missing info dictionary")
		}
		key, next, err := parseBencodeString(data, pos)
		if err != nil {
			return 0, 0, err
		}
		pos = next
		valueStart := pos
		valueEnd, err := parseBencodeElement(data, pos)
		if err != nil {
			return 0, 0, err
		}
		if string(key) == "info" {
			return valueStart, valueEnd, nil
		}
		pos = valueEnd
	}
	return 0, 0, errors.New("unterminated torrent metadata dictionary")
}

func parseBencodeElement(data []byte, pos int) (int, error) {
	if pos >= len(data) {
		return 0, errors.New("unexpected end of bencode data")
	}
	switch data[pos] {
	case 'i':
		return parseBencodeInteger(data, pos)
	case 'l':
		return parseBencodeList(data, pos)
	case 'd':
		return parseBencodeDictionary(data, pos)
	default:
		if data[pos] >= '0' && data[pos] <= '9' {
			_, end, err := parseBencodeString(data, pos)
			return end, err
		}
		return 0, fmt.Errorf("invalid bencode token %q at offset %d", data[pos], pos)
	}
}

func parseBencodeInteger(data []byte, pos int) (int, error) {
	if pos >= len(data) || data[pos] != 'i' {
		return 0, errors.New("expected bencode integer")
	}
	pos++
	if pos >= len(data) {
		return 0, errors.New("unterminated bencode integer")
	}
	if data[pos] == '-' {
		pos++
	}
	if pos >= len(data) || data[pos] < '0' || data[pos] > '9' {
		return 0, errors.New("invalid bencode integer")
	}
	for pos < len(data) && data[pos] >= '0' && data[pos] <= '9' {
		pos++
	}
	if pos >= len(data) || data[pos] != 'e' {
		return 0, errors.New("unterminated bencode integer")
	}
	return pos + 1, nil
}

func parseBencodeList(data []byte, pos int) (int, error) {
	if pos >= len(data) || data[pos] != 'l' {
		return 0, errors.New("expected bencode list")
	}
	pos++
	for pos < len(data) && data[pos] != 'e' {
		next, err := parseBencodeElement(data, pos)
		if err != nil {
			return 0, err
		}
		pos = next
	}
	if pos >= len(data) || data[pos] != 'e' {
		return 0, errors.New("unterminated bencode list")
	}
	return pos + 1, nil
}

func parseBencodeDictionary(data []byte, pos int) (int, error) {
	if pos >= len(data) || data[pos] != 'd' {
		return 0, errors.New("expected bencode dictionary")
	}
	pos++
	for pos < len(data) && data[pos] != 'e' {
		_, next, err := parseBencodeString(data, pos)
		if err != nil {
			return 0, err
		}
		pos, err = parseBencodeElement(data, next)
		if err != nil {
			return 0, err
		}
	}
	if pos >= len(data) || data[pos] != 'e' {
		return 0, errors.New("unterminated bencode dictionary")
	}
	return pos + 1, nil
}

func parseBencodeString(data []byte, pos int) ([]byte, int, error) {
	if pos >= len(data) || data[pos] < '0' || data[pos] > '9' {
		return nil, 0, errors.New("expected bencode string")
	}
	length := 0
	for pos < len(data) && data[pos] >= '0' && data[pos] <= '9' {
		length = length*10 + int(data[pos]-'0')
		pos++
	}
	if pos >= len(data) || data[pos] != ':' {
		return nil, 0, errors.New("invalid bencode string length")
	}
	pos++
	end := pos + length
	if length < 0 || end > len(data) {
		return nil, 0, errors.New("bencode string exceeds input length")
	}
	return data[pos:end], end, nil
}
