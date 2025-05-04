package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"

	"github.com/jackc/pgx/v5"
	"github.com/medinapdr/world-gen/config"
	"github.com/medinapdr/world-gen/models"
)

// WorldService manages the creation and retrieval of worlds
type WorldService struct {
	dbConfig  *config.DatabaseConfig
	appConfig *config.AppConfig
}

// NewWorldService creates a new instance of the service
func NewWorldService(dbConfig *config.DatabaseConfig, appConfig *config.AppConfig) *WorldService {
	return &WorldService{
		dbConfig:  dbConfig,
		appConfig: appConfig,
	}
}

// GenerateWorld creates a new world based on the theme
func (s *WorldService) GenerateWorld(ctx context.Context, theme string) (*models.World, error) {
	if theme == "" {
		theme = "fantasy"
	}

	if !validateTheme(theme) {
		theme = "fantasy"
	}

	climate := randomClimate()
	features := randomFeatures(climate)
	fauna := randomFauna(climate, theme)
	flora := randomFlora(climate, theme)
	cultures := randomCultures(theme)
	dangers := randomDangers(climate, theme)
	languages := randomLanguages(theme)

	w := &models.World{
		Name:        randomName(theme),
		Description: generateDescription(theme, climate, features, fauna, flora),
		Population:  rand.Intn(10000000),
		Climate:     climate,
		Features:    features,
		Theme:       theme,
		Fauna:       fauna,
		Flora:       flora,
		Cultures:    cultures,
		Dangers:     dangers,
		Languages:   languages,
	}

	if s.dbConfig.DB != nil {
		err := s.saveWorldToDB(ctx, w)
		if err != nil {
			log.Printf("Error inserting into DB: %v", err)
		}
	}

	if s.dbConfig.RedisClient != nil {
		s.cacheWorld(ctx, w)
	}

	return w, nil
}

// saveWorldToDB persists the world to the database and updates the ID
func (s *WorldService) saveWorldToDB(ctx context.Context, w *models.World) error {
	var id int
	err := s.dbConfig.DB.QueryRow(ctx,
		`INSERT INTO worlds(name, description, population, climate, features, theme) 
		 VALUES($1,$2,$3,$4,$5,$6) RETURNING id`,
		w.Name, w.Description, w.Population, w.Climate, w.Features, w.Theme).Scan(&id)

	if err != nil {
		return err
	}

	w.ID = id
	return nil
}

// cacheWorld stores the world in Redis
func (s *WorldService) cacheWorld(ctx context.Context, w *models.World) {
	historyKey := "world-history"
	worldJSON, err := json.Marshal(w)
	if err != nil {
		log.Printf("Error serializing world: %v", err)
	} else {
		s.dbConfig.RedisClient.LPush(ctx, historyKey, string(worldJSON))
		s.dbConfig.RedisClient.LTrim(ctx, historyKey, 0, int64(s.appConfig.HistoryLimit-1))

		// Cache individual world by ID if available
		if w.ID > 0 {
			worldKey := fmt.Sprintf("world:%d", w.ID)
			s.dbConfig.RedisClient.Set(ctx, worldKey, string(worldJSON), 0)
		}
	}
}

// GetWorldByID retrieves a specific world by its ID
func (s *WorldService) GetWorldByID(ctx context.Context, id int) (*models.World, error) {
	// Try to get from Redis cache first
	if s.dbConfig.RedisClient != nil {
		worldKey := fmt.Sprintf("world:%d", id)
		worldJSON, err := s.dbConfig.RedisClient.Get(ctx, worldKey).Result()

		if err == nil {
			var world models.World
			if err := json.Unmarshal([]byte(worldJSON), &world); err == nil {
				return &world, nil
			}
		}
	}

	// Fallback to database if Redis failed or world not found in cache
	if s.dbConfig.DB != nil {
		var world models.World
		err := s.dbConfig.DB.QueryRow(ctx,
			`SELECT id, name, description, population, climate, features, theme, created_at 
			 FROM worlds WHERE id = $1`, id).Scan(
			&world.ID, &world.Name, &world.Description, &world.Population,
			&world.Climate, &world.Features, &world.Theme, &world.CreatedAt)

		if err == nil {
			// Update cache
			if s.dbConfig.RedisClient != nil {
				s.cacheWorld(ctx, &world)
			}
			return &world, nil
		} else if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("world with ID %d not found", id)
		} else {
			return nil, err
		}
	}

	return nil, fmt.Errorf("no database connection available")
}

// SearchWorlds searches for worlds based on criteria
func (s *WorldService) SearchWorlds(ctx context.Context, query string, theme, climate string, limit, offset int) ([]models.World, int, error) {
	if s.dbConfig.DB == nil {
		return nil, 0, fmt.Errorf("no database connection available")
	}

	if limit <= 0 {
		limit = 10
	}

	// Build base query without LIMIT and OFFSET for counting total records
	baseQuery := `SELECT id, name, description, population, climate, features, theme, created_at 
				 FROM worlds WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM worlds WHERE 1=1`

	args := make([]interface{}, 0)
	argPos := 1

	if query != "" {
		whereClause := fmt.Sprintf(" AND (name ILIKE $%d OR description ILIKE $%d)", argPos, argPos)
		baseQuery += whereClause
		countQuery += whereClause
		args = append(args, "%"+query+"%")
		argPos++
	}

	if theme != "" {
		whereClause := fmt.Sprintf(" AND theme = $%d", argPos)
		baseQuery += whereClause
		countQuery += whereClause
		args = append(args, theme)
		argPos++
	}

	if climate != "" {
		whereClause := fmt.Sprintf(" AND climate = $%d", argPos)
		baseQuery += whereClause
		countQuery += whereClause
		args = append(args, climate)
		argPos++
	}

	// First, get the total count
	var total int
	err := s.dbConfig.DB.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Then, get the paginated results
	selectQuery := baseQuery + " ORDER BY created_at DESC LIMIT $" + fmt.Sprint(argPos) + " OFFSET $" + fmt.Sprint(argPos+1)
	args = append(args, limit, offset)

	rows, err := s.dbConfig.DB.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var worlds []models.World
	for rows.Next() {
		var world models.World
		err := rows.Scan(&world.ID, &world.Name, &world.Description, &world.Population,
			&world.Climate, &world.Features, &world.Theme, &world.CreatedAt)
		if err != nil {
			continue
		}
		worlds = append(worlds, world)
	}

	return worlds, total, nil
}

// GetWorldHistory retrieves the history of generated worlds
func (s *WorldService) GetWorldHistory(ctx context.Context) ([]models.World, error) {
	var worlds []models.World
	if s.dbConfig.RedisClient == nil {
		return worlds, nil
	}

	historyKey := "world-history"
	worldsJSON, err := s.dbConfig.RedisClient.LRange(ctx, historyKey, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	for _, worldJSON := range worldsJSON {
		var world models.World
		if err := json.Unmarshal([]byte(worldJSON), &world); err != nil {
			log.Printf("Error deserializing world: %v", err)
			continue
		}
		worlds = append(worlds, world)
	}

	return worlds, nil
}

// Helper functions for content generation

var climates = []string{
	"Arid", "Temperate", "Tropical", "Arctic", "Mediterranean",
	"Alpine", "Oceanic", "Continental", "Monsoonal", "Polar",
	"Desert", "Savanna", "Rainforest", "Tundra", "Humid Subtropical",
}

var featuresByClimate = map[string][]string{
	"Arid":              {"Sand dunes", "Isolated oases", "Cracked earth", "Salt flats", "Dust storms", "Canyons", "Mesas", "Rock formations", "Stone arches", "Desert blooms"},
	"Temperate":         {"Conifer forests", "Green fields", "Rolling hills", "Rain showers", "Wildflowers", "Deciduous forests", "Rivers", "Lakes", "Meadows", "Vales"},
	"Tropical":          {"Dense forests", "Paradise islands", "Steamy jungles", "Monsoon storms", "Coral reefs", "Mangrove swamps", "Waterfalls", "Volcanic islands", "Pristine beaches", "Hidden caverns"},
	"Arctic":            {"Ice fields", "Snow-covered mountains", "Glacial caves", "Aurora borealis", "Permafrost plains", "Ice floes", "Frozen waterfalls", "Snow drifts", "Frozen lakes", "Hot springs"},
	"Mediterranean":     {"Vineyards", "Century-old olive trees", "Sun-drenched coasts", "Herb-scented breezes", "Terraced hillsides", "Rocky coves", "Azure waters", "Cypress trees", "Stone villages", "Coastal cliffs"},
	"Alpine":            {"Rugged peaks", "Flower meadows", "Evergreen forests", "Mountain lakes", "Glacier streams", "Rock slides", "Mountain passes", "Fog banks", "Stone cabins", "Avalanche paths"},
	"Oceanic":           {"Misty hillsides", "Green valleys", "Moss-covered stones", "Frequent showers", "Hedgerows", "Thatched cottages", "Windswept coasts", "Foggy mornings", "Peat bogs", "Heather fields"},
	"Continental":       {"Great plains", "Rich farmland", "Seasonal forests", "Summer storms", "River networks", "Grasslands", "Four distinct seasons", "Flood plains", "Limestone caves", "Ponds"},
	"Monsoonal":         {"Rice terraces", "Bamboo groves", "Seasonal flooding", "Dense fogs", "Orchid forests", "Tea plantations", "Stepped terrain", "Summer rains", "Mist-shrouded mountains", "Water gardens"},
	"Polar":             {"Ice sheets", "Frozen seas", "Midnight sun", "Polar night", "Icebergs", "Frozen mountains", "Ice caves", "Snow plains", "Crystal forests", "Shimmering lights"},
	"Desert":            {"Cacti forests", "Rock pillars", "Dry riverbeds", "Shifting sands", "Mirages", "Ancient meteorite craters", "Sand valleys", "Fossil beds", "Obsidian fields", "Hidden springs"},
	"Savanna":           {"Tall grasses", "Acacia trees", "Seasonal wetlands", "Termite mounds", "Baobab trees", "Watering holes", "Red soil", "Flat-topped mountains", "Valley systems", "Seasonal burns"},
	"Rainforest":        {"Emergent canopies", "Lianas", "Medicinal plants", "Mist curtains", "Forest floor", "Huge buttress roots", "Colorful birds", "Singing insects", "Epiphytic gardens", "Mossy banks"},
	"Tundra":            {"Lichen fields", "Dwarf shrubs", "Caribou moss", "Shallow lakes", "Hardy flowers", "Ice wedge polygons", "Stone circles", "Peat mounds", "Meltwater streams", "Bare ridges"},
	"Humid Subtropical": {"Spanish moss", "Swamp cypress", "Brick-red soil", "Magnolia trees", "Summer thunderstorms", "Azalea gardens", "Year-round greenery", "Morning mist", "Firefly fields", "Warm lagoons"},
}

var faunaByClimate = map[string]map[string][]string{
	"fantasy": {
		"Arid":          {"Sand drakes", "Dust sprites", "Mirage phoenixes", "Heat salamanders", "Crystal scorpions"},
		"Temperate":     {"Talking deer", "Sprite foxes", "Luminous rabbits", "Healing doves", "Enchanted wolves"},
		"Tropical":      {"Rainbow serpents", "Jeweled macaws", "Glow frogs", "Giant butterflies", "Fae panthers"},
		"Arctic":        {"Frost giants", "Ice wyverns", "Snow sphinxes", "Boreal phoenixes", "Glacial bears"},
		"Mediterranean": {"Oracle octopi", "Sea nymphs", "Sphinx lions", "Wine-loving fauns", "Sage owls"},
	},
	"sci-fi": {
		"Arid":          {"Silicon-based crawlers", "Photosynthetic predators", "Sand-phase organisms", "Heat-energy beings", "Metal-eating insects"},
		"Temperate":     {"Biomechanical deer", "Engineered canines", "Surveillance birds", "Camouflage symbiotes", "Pollen-collecting drones"},
		"Tropical":      {"Genetically-enhanced primates", "Bio-luminescent birds", "Engineered amphibians", "Data-collecting insects", "Hyper-evolved felines"},
		"Arctic":        {"Cryo-adapted lifeforms", "Thermal parasites", "Ice-boring worms", "Magnetic field sensors", "Thermophilic microbes"},
		"Mediterranean": {"Aquatic data collectors", "Water purifier organisms", "Coastal reconnaissance drones", "Energy-harvesting fish", "Terraforming coral"},
	},
	"post-apocalyptic": {
		"Arid":          {"Radiation-resistant lizards", "Mutated scorpions", "Sand piranhas", "Toxic hornets", "Dust wolves"},
		"Temperate":     {"Three-eyed deer", "Acid rain frogs", "Oversized insects", "Scavenger dogs", "Pack rats"},
		"Tropical":      {"Toxic-resistant monkeys", "Vegetation-fused birds", "Poison dart frogs", "Giant mosquitoes", "Jungle stalkers"},
		"Arctic":        {"White stalkers", "Frost wolves", "Cryo-adapted humans", "Radioactive polar bears", "Snow piercers"},
		"Mediterranean": {"Pollution-filtering fish", "Shoreline scavengers", "Mutated dolphins", "Plastic-eating crabs", "Acidic jellyfish"},
	},
}

var floraByClimate = map[string]map[string][]string{
	"fantasy": {
		"Arid":          {"Mirage blooms", "Phoenix feather cacti", "Singing sand lilies", "Time-slowing succulents", "Mana crystals"},
		"Temperate":     {"Whispering willows", "Memory moss", "Fae light flowers", "Healing herbs", "Talking oak trees"},
		"Tropical":      {"Dream fruit trees", "Waterfall orchids", "Sentient vines", "Rainbow palms", "Wish-granting flowers"},
		"Arctic":        {"Frost lilies", "Eternal ice roses", "Northern light flowers", "Snow essence trees", "Crystal pines"},
		"Mediterranean": {"Oracle olives", "Fate-weaving vines", "Divine laurel", "Prophetic herbs", "Immortality figs"},
	},
	"sci-fi": {
		"Arid":          {"Silicon flora", "Metal-absorbing cacti", "Bio-solar plants", "Data storage succulents", "Moisture harvesters"},
		"Temperate":     {"Oxygen hyperproducers", "Bio-luminescent trees", "Communication fungi", "Medicine-producing flowers", "Weather-controlling plants"},
		"Tropical":      {"Gene-altering fruits", "Bio-electronic vines", "Anti-gravity flowers", "Species-adapting trees", "Consciousness-expanding fungi"},
		"Arctic":        {"Thermal generator plants", "Cryo-preserving lichens", "Ice-penetrating roots", "Bio-antifreeze producers", "Data-storing crystals"},
		"Mediterranean": {"Desalination trees", "Current-generating seaweed", "Bio-filter reeds", "Holographic flowers", "Atmospheric adjusters"},
	},
	"post-apocalyptic": {
		"Arid":          {"Radiation-feeding cacti", "Metal-absorbing weeds", "Toxic spore producers", "Fallout-resistant shrubs", "Mutated yuccas"},
		"Temperate":     {"Glowing fungi", "Acid-resistant trees", "Carnivorous wildflowers", "Mutation-causing berries", "Oxygen-hoarding plants"},
		"Tropical":      {"Irradiated palms", "Rapidly-evolving vines", "Memory-altering fruit", "Hybrid fungi-animals", "Toxic paradise flowers"},
		"Arctic":        {"Heat-stealing lichen", "Nuclear winter trees", "Frozen time capsule flowers", "Radiation-preserving ice plants", "Mutated evergreens"},
		"Mediterranean": {"Oil-filtering reeds", "Plastic-decomposing algae", "Contamination indicator flowers", "Salt-purifying trees", "Human-repelling herbs"},
	},
}

var culturesByTheme = map[string][]string{
	"fantasy":          {"Ancient elven dynasties", "Dwarf mining guilds", "Nomadic halfling tribes", "Human kingdoms", "Dragonborn clans", "Magical academies", "Twilight courts", "Oracle temples", "Beast-people tribes", "Elemental communes"},
	"sci-fi":           {"Space mining corporations", "AI collectives", "Human resistance", "Genetic purists", "Cyborg syndicates", "Terraforming guilds", "Quantum researchers", "Alien embassies", "Data monks", "Void explorers"},
	"post-apocalyptic": {"Bunker dwellers", "Wasteland raiders", "Water barons", "Tech salvagers", "Radiation cultists", "Agricultural communes", "Trading caravans", "Stronghold cities", "Nomad tribes", "Memory keepers"},
}

var dangersByTheme = map[string]map[string][]string{
	"fantasy": {
		"Arid":          {"Ancient buried curses", "Sandstorm elementals", "Mirage demons", "Sun dragons", "Heat madness"},
		"Temperate":     {"Forest guardians", "Fae tricksters", "Cursed ruins", "Shapeshifting predators", "Living storms"},
		"Tropical":      {"Jungle spirits", "Carnivorous plants", "Temple guardians", "Venom sprites", "Quicksand portals"},
		"Arctic":        {"Frost giants", "Avalanche spirits", "Ice curses", "Soul-freezing winds", "Hunger madness"},
		"Mediterranean": {"Sirens", "Ancient sea monsters", "Cursed islands", "Wine enchantments", "Memory thieves"},
	},
	"sci-fi": {
		"Arid":          {"Rogue terraforming machines", "Sand-based nanobots", "Heat-activated mines", "Mirage defense systems", "Water thieves"},
		"Temperate":     {"Surveillance ecosystems", "Rogue bioweapons", "Perception filters", "Reality distortion fields", "Neural parasites"},
		"Tropical":      {"Gene-altering pollens", "Predatory plants", "Machine-jungle hybrids", "Bio-electronic hazards", "Memory-altering spores"},
		"Arctic":        {"Cryo-weapons", "Consciousness-stealing ice", "Sub-zero nanites", "White-out zones", "Thermal anomalies"},
		"Mediterranean": {"Water-borne data viruses", "Mind-controlling parasites", "Coastal defense systems", "Weather control malfunctions", "Reality bubbles"},
	},
	"post-apocalyptic": {
		"Arid":          {"Radiation zones", "Dust storms", "Cannibalistic tribes", "Ancient weapon caches", "Nuclear mirages"},
		"Temperate":     {"Toxic rain", "Mutated predators", "Bandit territories", "Collapsing infrastructure", "Disease zones"},
		"Tropical":      {"Poisoned water", "Predatory plant life", "Feral survivor camps", "Quicksand pits", "Hallucinogenic spores"},
		"Arctic":        {"Deadly blizzards", "Starvation", "Ice pirates", "Underground radiation", "Freezing fog"},
		"Mediterranean": {"Coastal raiders", "Polluted seas", "Resource wars", "Flooded ruins", "Water-borne diseases"},
	},
}

var languagesByTheme = map[string][]string{
	"fantasy":          {"Ancient Elvish", "Dwarven Runes", "Common Tongue", "Sylvan Whispers", "Draconic", "Abyssal", "Celestial", "Primordial", "Fae Speech", "Gnomish"},
	"sci-fi":           {"Galactic Standard", "Binary Code", "Quantum Script", "Neural Interface", "Alien Dialects", "Mathematical Patterns", "Light Pulses", "Sonic Patterns", "Encoded Transmissions", "Temporal Linguistics"},
	"post-apocalyptic": {"Wasteland Slang", "Old World English", "Trade Pidgin", "Signal Code", "Radiation Clicks", "Bunker Dialect", "Survivor's Cant", "Scavenger Signs", "Tech-Speech", "Brotherhood Code"},
}

func validateTheme(theme string) bool {
	validThemes := []string{"fantasy", "sci-fi", "post-apocalyptic"}
	for _, validTheme := range validThemes {
		if theme == validTheme {
			return true
		}
	}
	return false
}

func randomName(theme string) string {
	prefixes := map[string][]string{
		"fantasy":          {"Aure", "Eld", "Myth", "Zan", "Thaur", "Crystal", "Ever", "Fel", "Glimmer", "Iron"},
		"sci-fi":           {"Xen", "Nova", "Qar", "Zy", "Eco", "Neb", "Sol", "Astra", "Orb", "Pulse"},
		"post-apocalyptic": {"Ruina", "Ash", "Hollow", "Grim", "Waste", "Dead", "Lost", "Broken", "Rust", "Shadow"},
	}
	suffixes := map[string][]string{
		"fantasy":          {"ia", "or", "an", "eth", "haven", "wood", "vale", "gard", "heart", "realm"},
		"sci-fi":           {"-Prime", "-X", "-7", "-II", "-Nova", "-Core", "-Nexus", "-Sphere", "-Alpha", "-Zero"},
		"post-apocalyptic": {"fall", "land", "vale", "berg", "waste", "ruins", "haven", "outpost", "refuge", "pit"},
	}
	pre := prefixes[theme]
	suf := suffixes[theme]
	if pre == nil {
		pre = prefixes["fantasy"]
		suf = suffixes["fantasy"]
	}
	return fmt.Sprintf("%s%s", pre[rand.Intn(len(pre))], suf[rand.Intn(len(suf))])
}

func generateDescription(theme, climate string, features, fauna, flora []string) string {
	f1, f2 := features[0], features[1]
	animal := ""
	if len(fauna) > 0 {
		animal = fauna[0]
	}
	plant := ""
	if len(flora) > 0 {
		plant = flora[0]
	}

	templates := []string{
		"In this %s-themed world, the %s climate fosters %s and %s. %s roam among the %s.",
		"A world with %s climate, where %s and %s abound. Home to %s and magnificent %s.",
		"This %s world is defined by its %s climate and %s alongside %s. Travelers may encounter %s near the %s.",
		"Explore a %s realm under %s skies, with %s and %s. Beware of %s hiding within the %s.",
	}
	tmpl := templates[rand.Intn(len(templates))]
	return fmt.Sprintf(tmpl, theme, climate, f1, f2, animal, plant)
}

func randomClimate() string {
	return climates[rand.Intn(len(climates))]
}

func randomWithoutDuplicates(items []string, count int) []string {
	if count <= 0 {
		return []string{}
	}

	// Create a copy to avoid modifying the original
	itemsCopy := make([]string, len(items))
	copy(itemsCopy, items)

	// Shuffle the copy
	rand.Shuffle(len(itemsCopy), func(i, j int) {
		itemsCopy[i], itemsCopy[j] = itemsCopy[j], itemsCopy[i]
	})

	// Return the specified number of items or all if count > len
	if count > len(itemsCopy) {
		count = len(itemsCopy)
	}
	return itemsCopy[:count]
}

func randomFeatures(climate string) []string {
	feats := featuresByClimate[climate]
	if feats == nil {
		feats = featuresByClimate["Temperate"]
	}

	// Get 2-4 unique features
	count := 2 + rand.Intn(3) // 2, 3, or 4
	return randomWithoutDuplicates(feats, count)
}

func randomFauna(climate, theme string) []string {
	fauna := faunaByClimate[theme][climate]
	if fauna == nil {
		// Fallback to another climate or theme if specific combination not found
		for _, f := range faunaByClimate[theme] {
			fauna = f
			break
		}
		if fauna == nil {
			fauna = faunaByClimate["fantasy"]["Temperate"]
		}
	}

	count := 2 + rand.Intn(3) // 2-4 fauna
	return randomWithoutDuplicates(fauna, count)
}

func randomFlora(climate, theme string) []string {
	flora := floraByClimate[theme][climate]
	if flora == nil {
		// Fallback to another climate or theme if specific combination not found
		for _, f := range floraByClimate[theme] {
			flora = f
			break
		}
		if flora == nil {
			flora = floraByClimate["fantasy"]["Temperate"]
		}
	}

	count := 2 + rand.Intn(3) // 2-4 flora
	return randomWithoutDuplicates(flora, count)
}

func randomCultures(theme string) []string {
	cultures := culturesByTheme[theme]
	if cultures == nil {
		cultures = culturesByTheme["fantasy"]
	}

	count := 1 + rand.Intn(3) // 1-3 cultures
	return randomWithoutDuplicates(cultures, count)
}

func randomDangers(climate, theme string) []string {
	dangers := dangersByTheme[theme][climate]
	if dangers == nil {
		// Fallback to another climate or theme if specific combination not found
		for _, d := range dangersByTheme[theme] {
			dangers = d
			break
		}
		if dangers == nil {
			dangers = dangersByTheme["fantasy"]["Temperate"]
		}
	}

	count := 1 + rand.Intn(2) // 1-2 dangers
	return randomWithoutDuplicates(dangers, count)
}

func randomLanguages(theme string) []string {
	langs := languagesByTheme[theme]
	if langs == nil {
		langs = languagesByTheme["fantasy"]
	}

	count := 1 + rand.Intn(3) // 1-3 languages
	return randomWithoutDuplicates(langs, count)
}
