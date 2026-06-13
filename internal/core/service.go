package core

import (
	"fmt"
	"strings"
	"sync"
)

const (
	base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	rangeSize   = 100000
)

// RangeProvider defines the interface for fetching key ranges (e.g., from ZooKeeper)
type RangeProvider interface {
	FetchRange(size uint64) (uint64, uint64, error)
}

type KeyService struct {
	provider   RangeProvider
	currentKey uint64
	maxKey     uint64
	mu         sync.Mutex
}

func NewKeyService(provider RangeProvider) (*KeyService, error) {
	ks := &KeyService{
		provider: provider,
	}
	// Initial fetch
	if err := ks.fetchNewRange(); err != nil {
		return nil, fmt.Errorf("initial range fetch failed: %v", err)
	}
	return ks, nil
}

func (ks *KeyService) fetchNewRange() error {
	start, end, err := ks.provider.FetchRange(rangeSize)
	if err != nil {
		return err
	}
	ks.currentKey = start
	ks.maxKey = end
	return nil
}

func (ks *KeyService) GetNextKey() (string, error) {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	if ks.currentKey >= ks.maxKey {
		if err := ks.fetchNewRange(); err != nil {
			return "", err
		}
	}

	key := ks.currentKey
	ks.currentKey++

	return toBase62(key), nil
}

func toBase62(num uint64) string {
	if num == 0 {
		return string(base62Chars[0])
	}

	var sb strings.Builder
	for num > 0 {
		rem := num % 62
		sb.WriteByte(base62Chars[rem])
		num /= 62
	}

	// Reverse the string
	str := sb.String()
	runes := []rune(str)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
