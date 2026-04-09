package buddy

type Rarity string

const (
	RarityCommon    Rarity = "common"
	RarityUncommon  Rarity = "uncommon"
	RarityRare      Rarity = "rare"
	RarityEpic      Rarity = "epic"
	RarityLegendary Rarity = "legendary"
)

var Rarities = []Rarity{
	RarityCommon,
	RarityUncommon,
	RarityRare,
	RarityEpic,
	RarityLegendary,
}

type Species string

var SpeciesList = []Species{
	"duck",
	"goose",
	"blob",
	"cat",
	"dragon",
	"octopus",
	"owl",
	"penguin",
	"turtle",
	"snail",
	"ghost",
	"axolotl",
	"capybara",
	"cactus",
	"robot",
	"rabbit",
	"mushroom",
	"chonk",
}

type Eye string

var Eyes = []Eye{
	"·",
	"✦",
	"×",
	"◉",
	"@",
	"°",
}

type Hat string

var Hats = []Hat{
	"none",
	"crown",
	"tophat",
	"propeller",
	"halo",
	"wizard",
	"beanie",
	"tinyduck",
}

type StatName string

var StatNames = []StatName{
	"DEBUGGING",
	"PATIENCE",
	"CHAOS",
	"WISDOM",
	"SNARK",
}

type CompanionBones struct {
	Rarity  Rarity           `json:"rarity"`
	Species Species          `json:"species"`
	Eye     Eye              `json:"eye"`
	Hat     Hat              `json:"hat"`
	Shiny   bool             `json:"shiny"`
	Stats   map[StatName]int `json:"stats"`
}

type CompanionSoul struct {
	Name        string `json:"name"`
	Personality string `json:"personality"`
}

type Companion struct {
	CompanionBones
	CompanionSoul
	HatchedAt int64 `json:"hatched_at"`
}

var RarityWeights = map[Rarity]int{
	RarityCommon:    60,
	RarityUncommon:  25,
	RarityRare:      10,
	RarityEpic:      4,
	RarityLegendary: 1,
}

var RarityStars = map[Rarity]string{
	RarityCommon:    "★",
	RarityUncommon:  "★★",
	RarityRare:      "★★★",
	RarityEpic:      "★★★★",
	RarityLegendary: "★★★★★",
}

var RarityFloor = map[Rarity]int{
	RarityCommon:    5,
	RarityUncommon:  15,
	RarityRare:      25,
	RarityEpic:      35,
	RarityLegendary: 50,
}
