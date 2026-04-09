package upstreamproxy

import (
	"net/http"
	"net/url"
	"regexp"
	"sync"
)

type RoutingRule struct {
	Pattern     string `json:"pattern"`
	UpstreamURL string `json:"upstream_url"`
	Priority    int    `json:"priority"`
	regex       *regexp.Regexp
}

type TrafficRouter struct {
	rules []RoutingRule
	proxy map[string]*UpstreamProxy
	mu    sync.RWMutex
}

func NewTrafficRouter() *TrafficRouter {
	return &TrafficRouter{
		rules: make([]RoutingRule, 0),
		proxy: make(map[string]*UpstreamProxy),
	}
}

func (tr *TrafficRouter) AddRule(rule RoutingRule) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	regex, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return err
	}

	rule.regex = regex

	for i, r := range tr.rules {
		if r.Priority < rule.Priority {
			tr.rules = append(tr.rules[:i], append([]RoutingRule{rule}, tr.rules[i:]...)...)
			return nil
		}
	}

	tr.rules = append(tr.rules, rule)
	return nil
}

func (tr *TrafficRouter) RemoveRule(pattern string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	for i, rule := range tr.rules {
		if rule.Pattern == pattern {
			tr.rules = append(tr.rules[:i], tr.rules[i+1:]...)
			break
		}
	}
}

func (tr *TrafficRouter) Route(req *http.Request) (*url.URL, error) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	path := req.URL.Path

	for _, rule := range tr.rules {
		if rule.regex != nil && rule.regex.MatchString(path) {
			targetURL, err := url.Parse(rule.UpstreamURL)
			if err != nil {
				continue
			}
			return targetURL, nil
		}
	}

	return nil, nil
}

func (tr *TrafficRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	targetURL, err := tr.Route(req)
	if err != nil || targetURL == nil {
		http.Error(w, "no matching route", http.StatusBadGateway)
		return
	}

	tr.mu.RLock()
	proxy, exists := tr.proxy[targetURL.String()]
	tr.mu.RUnlock()

	if !exists {
		http.Error(w, "proxy not configured", http.StatusInternalServerError)
		return
	}

	proxy.ServeHTTP(w, req)
}

func (tr *TrafficRouter) AddProxy(upstreamURL string, proxy *UpstreamProxy) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.proxy[upstreamURL] = proxy
}

func (tr *TrafficRouter) GetRules() []RoutingRule {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	result := make([]RoutingRule, len(tr.rules))
	copy(result, tr.rules)
	return result
}

func (tr *TrafficRouter) Clear() {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.rules = make([]RoutingRule, 0)
	tr.proxy = make(map[string]*UpstreamProxy)
}
