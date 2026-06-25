package whitelist

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"
)

var fallbackDomains = []string{
	"vk.ru", "yandex.ru", "gosuslugi.ru", "mail.ru", "ozon.ru",
	"avito.ru", "rutube.ru", "sberbank.ru", "2gis.ru", "dzen.ru",
}

type Service struct {
	url    string
	client *http.Client

	mu      sync.RWMutex
	domains []string
}

func New(url string) *Service {
	return &Service{
		url: url,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		domains: append([]string(nil), fallbackDomains...),
	}
}

func (s *Service) Start(ctx context.Context) {
	s.refresh(ctx)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.refresh(ctx)
			}
		}
	}()
}

func (s *Service) refresh(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}

	domains, err := parseDomains(resp.Body)
	if err != nil || len(domains) == 0 {
		return
	}

	s.mu.Lock()
	s.domains = domains
	s.mu.Unlock()
}

func parseDomains(r io.Reader) ([]string, error) {
	var domains []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, " ") {
			continue
		}
		domains = append(domains, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return domains, nil
}

func (s *Service) RandomDomain() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.domains) == 0 {
		return "", fmt.Errorf("no domains available")
	}
	return s.domains[rand.IntN(len(s.domains))], nil
}

func (s *Service) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.domains)
}
