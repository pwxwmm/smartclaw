package buddy

import (
	"path/filepath"
	"testing"
)

func TestRollFromSeed_Deterministic(t *testing.T) {
	r1 := RollFromSeed("test-seed-123")
	r2 := RollFromSeed("test-seed-123")
	if r1.Bones.Rarity != r2.Bones.Rarity {
		t.Errorf("Rarity mismatch: %s vs %s", r1.Bones.Rarity, r2.Bones.Rarity)
	}
	if r1.Bones.Species != r2.Bones.Species {
		t.Errorf("Species mismatch: %s vs %s", r1.Bones.Species, r2.Bones.Species)
	}
	if r1.InspirationSeed != r2.InspirationSeed {
		t.Errorf("InspirationSeed mismatch: %d vs %d", r1.InspirationSeed, r2.InspirationSeed)
	}
}

func TestRollFromSeed_DifferentSeeds(t *testing.T) {
	r1 := RollFromSeed("seed-a")
	r2 := RollFromSeed("seed-b")
	if r1.InspirationSeed == r2.InspirationSeed {
		t.Error("Different seeds produced same InspirationSeed (unlikely)")
	}
}

func TestPickRarity_CommonMostLikely(t *testing.T) {
	counts := map[Rarity]int{}
	iterations := 10000
	for i := 0; i < iterations; i++ {
		rng := mulberry32(uint32(i + 1))
		r := pickRarity(rng)
		counts[r]++
	}
	if counts[RarityCommon] <= counts[RarityUncommon] {
		t.Errorf("Common (%d) should be more frequent than Uncommon (%d)", counts[RarityCommon], counts[RarityUncommon])
	}
	if float64(counts[RarityCommon])/float64(iterations) < 0.4 {
		t.Errorf("Common frequency too low: %d/%d", counts[RarityCommon], iterations)
	}
}

func TestRollStats_PeakAndDump(t *testing.T) {
	rng := mulberry32(42)
	stats := rollStats(rng, RarityCommon)

	var peakStat StatName
	var dumpStat StatName
	peakVal := -1
	dumpVal := 999

	for _, name := range StatNames {
		v := stats[name]
		if v > peakVal {
			peakVal = v
			peakStat = name
		}
		if v < dumpVal {
			dumpVal = v
			dumpStat = name
		}
	}
	if peakVal <= dumpVal {
		t.Errorf("Peak (%d) should be > Dump (%d)", peakVal, dumpVal)
	}
	if peakStat == dumpStat {
		t.Error("Peak and dump stat should differ")
	}
}

func TestRollStats_PeakHigherThanFloor(t *testing.T) {
	rng := mulberry32(99)
	stats := rollStats(rng, RarityRare)
	floor := RarityFloor[RarityRare]

	var peakVal int
	for _, name := range StatNames {
		if stats[name] > peakVal {
			peakVal = stats[name]
		}
	}
	if peakVal < floor+50 {
		t.Errorf("Peak stat %d should be >= floor+50 = %d", peakVal, floor+50)
	}
}

func TestShiny_ApproximatelyOnePercent(t *testing.T) {
	shinyCount := 0
	iterations := 1000
	for i := 0; i < iterations; i++ {
		r := RollFromSeed("shiny-test-" + string(rune(i)))
		if r.Bones.Shiny {
			shinyCount++
		}
	}
	if shinyCount > 30 {
		t.Errorf("Shiny count = %d/%d, expected 0-30 range", shinyCount, iterations)
	}
}

func TestCompanionService_GetCompanion_MissingFile(t *testing.T) {
	s := NewCompanionService()
	s.configPath = filepath.Join(t.TempDir(), "nonexistent.json")
	companion, err := s.GetCompanion("user-1")
	if err != nil {
		t.Errorf("GetCompanion with missing file should not error: %v", err)
	}
	if companion != nil {
		t.Error("GetCompanion with missing file should return nil")
	}
}

func TestCompanionService_SaveAndGet_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewCompanionService()
	s.configPath = filepath.Join(dir, "companion.json")

	roll := RollFromSeed("test-user-friend-2026-401")
	companion := &Companion{
		CompanionBones: roll.Bones,
		CompanionSoul: CompanionSoul{
			Name:        "TestBuddy",
			Personality: "cheerful",
		},
		HatchedAt: 1234567890,
	}

	if err := s.SaveCompanion(companion); err != nil {
		t.Fatalf("SaveCompanion failed: %v", err)
	}

	got, err := s.GetCompanion("test-user")
	if err != nil {
		t.Fatalf("GetCompanion failed: %v", err)
	}
	if got.Name != "TestBuddy" {
		t.Errorf("Name = %q, want %q", got.Name, "TestBuddy")
	}
	if got.Personality != "cheerful" {
		t.Errorf("Personality = %q, want %q", got.Personality, "cheerful")
	}
	if got.HatchedAt != 1234567890 {
		t.Errorf("HatchedAt = %d, want 1234567890", got.HatchedAt)
	}
}

func TestCompanionService_HatchCompanion(t *testing.T) {
	dir := t.TempDir()
	s := NewCompanionService()
	s.configPath = filepath.Join(dir, "companion.json")

	companion, err := s.HatchCompanion("user-123", "Fluffy", "mischievous")
	if err != nil {
		t.Fatalf("HatchCompanion failed: %v", err)
	}
	if companion.Name != "Fluffy" {
		t.Errorf("Name = %q, want %q", companion.Name, "Fluffy")
	}
	if companion.Personality != "mischievous" {
		t.Errorf("Personality = %q, want %q", companion.Personality, "mischievous")
	}
	if companion.HatchedAt == 0 {
		t.Error("HatchedAt should be set")
	}
	if companion.Rarity == "" {
		t.Error("Rarity should be set")
	}
}

func TestRarityWeights_Values(t *testing.T) {
	expected := map[Rarity]int{
		RarityCommon:    60,
		RarityUncommon:  25,
		RarityRare:      10,
		RarityEpic:      4,
		RarityLegendary: 1,
	}
	for r, w := range expected {
		if RarityWeights[r] != w {
			t.Errorf("RarityWeights[%s] = %d, want %d", r, RarityWeights[r], w)
		}
	}
}

func TestRarityStars_Values(t *testing.T) {
	expected := map[Rarity]string{
		RarityCommon:    "★",
		RarityUncommon:  "★★",
		RarityRare:      "★★★",
		RarityEpic:      "★★★★",
		RarityLegendary: "★★★★★",
	}
	for r, stars := range expected {
		if RarityStars[r] != stars {
			t.Errorf("RarityStars[%s] = %q, want %q", r, RarityStars[r], stars)
		}
	}
}

func TestRarityFloor_Values(t *testing.T) {
	expected := map[Rarity]int{
		RarityCommon:    5,
		RarityUncommon:  15,
		RarityRare:      25,
		RarityEpic:      35,
		RarityLegendary: 50,
	}
	for r, floor := range expected {
		if RarityFloor[r] != floor {
			t.Errorf("RarityFloor[%s] = %d, want %d", r, RarityFloor[r], floor)
		}
	}
}

func TestMulberry32_Deterministic(t *testing.T) {
	rng1 := mulberry32(42)
	rng2 := mulberry32(42)
	for i := 0; i < 100; i++ {
		if rng1() != rng2() {
			t.Error("mulberry32 should be deterministic for same seed")
			break
		}
	}
}

func TestHashString_Deterministic(t *testing.T) {
	h1 := hashString("hello")
	h2 := hashString("hello")
	if h1 != h2 {
		t.Error("hashString should be deterministic")
	}
	if hashString("world") == h1 {
		t.Error("Different strings should produce different hashes")
	}
}
