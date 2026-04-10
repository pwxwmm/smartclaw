package gateway

import (
	"fmt"
	"log/slog"
	"sync"
)

type Pair struct {
	UserID    string
	Platforms []string
	SessionID string
}

type PairingManager struct {
	pairs map[string]*Pair
	mu    sync.RWMutex
}

func NewPairingManager() *PairingManager {
	return &PairingManager{
		pairs: make(map[string]*Pair),
	}
}

func (pm *PairingManager) PairSession(userID, platform, sessionID string) *Pair {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pair, ok := pm.pairs[userID]; ok {
		pair.SessionID = sessionID
		for _, p := range pair.Platforms {
			if p == platform {
				return pair
			}
		}
		pair.Platforms = append(pair.Platforms, platform)
		return pair
	}

	pair := &Pair{
		UserID:    userID,
		Platforms: []string{platform},
		SessionID: sessionID,
	}
	pm.pairs[userID] = pair
	return pair
}

func (pm *PairingManager) GetPair(userID string) *Pair {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.pairs[userID]
}

func (pm *PairingManager) FindSessionForPlatform(userID, platform string) (string, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	pair, ok := pm.pairs[userID]
	if !ok {
		return "", fmt.Errorf("pairing: no pair found for user %s", userID)
	}

	for _, p := range pair.Platforms {
		if p == platform {
			return pair.SessionID, nil
		}
	}
	return "", fmt.Errorf("pairing: user %s not paired on platform %s", userID, platform)
}

func (pm *PairingManager) UnpairPlatform(userID, platform string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pair, ok := pm.pairs[userID]
	if !ok {
		return
	}

	filtered := make([]string, 0, len(pair.Platforms))
	for _, p := range pair.Platforms {
		if p != platform {
			filtered = append(filtered, p)
		}
	}

	if len(filtered) == 0 {
		delete(pm.pairs, userID)
		slog.Info("pairing: removed pair, no platforms remaining", "user", userID)
		return
	}

	pair.Platforms = filtered
	slog.Info("pairing: unpaired platform", "user", userID, "platform", platform)
}

func (pm *PairingManager) ListActivePairs() []*Pair {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	pairs := make([]*Pair, 0, len(pm.pairs))
	for _, pair := range pm.pairs {
		pairs = append(pairs, pair)
	}
	return pairs
}
