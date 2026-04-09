package buddy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Roll struct {
	Bones           CompanionBones `json:"bones"`
	InspirationSeed int            `json:"inspiration_seed"`
}

type CompanionService struct {
	configPath string
}

func NewCompanionService() *CompanionService {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".smartclaw", "companion.json")
	return &CompanionService{configPath: configPath}
}

func mulberry32(seed uint32) func() float64 {
	return func() float64 {
		seed |= 0
		seed = (seed + 0x6d2b79f5) | 0
		t := uint32(seed ^ (seed >> 15))
		t = (t + (t * (t ^ (t >> 7) | 61 | t))) ^ t
		return float64((t^(t>>14))>>0) / 4294967296.0
	}
}

func hashString(s string) uint32 {
	h := uint32(2166136261)
	for _, c := range s {
		h ^= uint32(c)
		h *= 16777619
	}
	return h
}

func pickRarity(rng func() float64) Rarity {
	total := 0
	for _, w := range RarityWeights {
		total += w
	}
	roll := rng() * float64(total)
	for _, r := range Rarities {
		roll -= float64(RarityWeights[r])
		if roll < 0 {
			return r
		}
	}
	return RarityCommon
}

func pickSpecies(rng func() float64) Species {
	idx := int(rng() * float64(len(SpeciesList)))
	return SpeciesList[idx]
}

func pickEye(rng func() float64) Eye {
	idx := int(rng() * float64(len(Eyes)))
	return Eyes[idx]
}

func pickHat(rng func() float64) Hat {
	idx := int(rng() * float64(len(Hats)))
	return Hats[idx]
}

func rollStats(rng func() float64, rarity Rarity) map[StatName]int {
	floor := RarityFloor[rarity]
	stats := make(map[StatName]int)

	peakIdx := int(rng() * float64(len(StatNames)))
	dumpIdx := int(rng() * float64(len(StatNames)))
	for dumpIdx == peakIdx {
		dumpIdx = int(rng() * float64(len(StatNames)))
	}

	for i, name := range StatNames {
		if i == peakIdx {
			stats[name] = min(100, floor+50+int(rng()*30))
		} else if i == dumpIdx {
			stats[name] = max(1, floor-10+int(rng()*15))
		} else {
			stats[name] = floor + int(rng()*40)
		}
	}
	return stats
}

func RollFromSeed(seed string) Roll {
	rng := mulberry32(hashString(seed))

	rarity := pickRarity(rng)
	bones := CompanionBones{
		Rarity:  rarity,
		Species: pickSpecies(rng),
		Eye:     pickEye(rng),
		Hat:     "none",
		Shiny:   rng() < 0.01,
		Stats:   rollStats(rng, rarity),
	}

	if rarity != RarityCommon {
		bones.Hat = pickHat(rng)
	}

	return Roll{
		Bones:           bones,
		InspirationSeed: int(rng() * 1e9),
	}
}

func (s *CompanionService) GetCompanion(userID string) (*Companion, error) {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return nil, nil
	}

	var stored struct {
		Name        string `json:"name"`
		Personality string `json:"personality"`
		HatchedAt   int64  `json:"hatched_at"`
	}

	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, err
	}

	roll := RollFromSeed(userID + "-friend-2026-401")

	companion := &Companion{
		CompanionBones: roll.Bones,
		CompanionSoul: CompanionSoul{
			Name:        stored.Name,
			Personality: stored.Personality,
		},
		HatchedAt: stored.HatchedAt,
	}

	return companion, nil
}

func (s *CompanionService) SaveCompanion(companion *Companion) error {
	if err := os.MkdirAll(filepath.Dir(s.configPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(companion, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.configPath, data, 0644)
}

func (s *CompanionService) HatchCompanion(userID, name, personality string) (*Companion, error) {
	roll := RollFromSeed(userID + "-friend-2026-401")

	companion := &Companion{
		CompanionBones: roll.Bones,
		CompanionSoul: CompanionSoul{
			Name:        name,
			Personality: personality,
		},
		HatchedAt: time.Now().Unix(),
	}

	if err := s.SaveCompanion(companion); err != nil {
		return nil, err
	}

	return companion, nil
}
