package translate

import (
	"fmt"
	"strings"
)

// PlatformHint returns a platform description so the model maps Chinese titles
// to the English names scrapers (IGDB / SteamGridDB) expect.
func PlatformHint(platformID, platformName string) string {
	id := strings.ToLower(strings.TrimSpace(platformID))
	name := strings.TrimSpace(platformName)
	if hint, ok := platformHints[id]; ok {
		if name != "" && !strings.EqualFold(name, hint.short) {
			return fmt.Sprintf("%s（%s）— %s", name, hint.short, hint.detail)
		}
		return fmt.Sprintf("%s — %s", hint.short, hint.detail)
	}
	if name == "" {
		name = platformID
	}
	return fmt.Sprintf("%s — retro platform; use the common English title used by Western databases", name)
}

type platformHint struct {
	short  string
	detail string
}

var platformHints = map[string]platformHint{
	"fc":                {"FC / Famicom / NES", "Nintendo 8-bit. Map 大陆译名 to NES/Famicom English titles, e.g. 超级马里奥兄弟 → Super Mario Bros., 魂斗罗 → Contra"},
	"fc-hack":           {"FC ROM hack", "Based on NES/Famicom. Use the original English title; mention Hack only if that is the common English name"},
	"fc-hd":             {"FC HD", "HD remake of NES/Famicom. Prefer the original English title"},
	"sfc":               {"SFC / Super Famicom / SNES", "Nintendo 16-bit. Prefer SNES English titles (Final Fantasy III US = Final Fantasy VI)"},
	"sfc-hack":          {"SFC ROM hack", "Based on SNES. Use the original English title"},
	"md":                {"MD / Mega Drive / Genesis", "Sega 16-bit. Use common English titles, e.g. 索尼克 → Sonic the Hedgehog, 怒之铁拳 → Streets of Rage"},
	"md-hack":           {"MD ROM hack", "Based on Mega Drive / Genesis"},
	"md-hack-picodrive": {"MD ROM hack (PicoDrive)", "Based on Mega Drive / Genesis"},
	"md-32x":            {"Mega Drive 32X", "Sega 32X. Use Genesis/32X English titles"},
	"gb":                {"Game Boy", "Nintendo handheld. 口袋妖怪 → Pokémon"},
	"gbc":               {"Game Boy Color", "Nintendo handheld"},
	"gba":               {"Game Boy Advance", "Nintendo handheld"},
	"gg":                {"Game Gear", "Sega handheld"},
	"sms":               {"Master System", "Sega 8-bit"},
	"n64":               {"Nintendo 64", "Use common English titles"},
	"nds":               {"Nintendo DS", "Use common English titles"},
	"ngc":               {"GameCube", "Nintendo GameCube"},
	"wii":               {"Wii", "Nintendo Wii"},
	"wii-ware":          {"WiiWare", "Keep the original English digital-release title"},
	"virtual-boy":       {"Virtual Boy", "Nintendo Virtual Boy"},
	"ps1":               {"PlayStation", "Sony PS1. Prefer the English / international title used by IGDB"},
	"psp":               {"PSP", "Sony PSP"},
	"ss":                {"Sega Saturn", "Many Japan-only games; use the common English romanization or official English title"},
	"dc":                {"Dreamcast", "Sega Dreamcast"},
	"pce":               {"PC Engine / TurboGrafx-16", "Use the TurboGrafx or common English title"},
	"pce-cd":            {"PC Engine CD", "PC Engine CD / TurboGrafx-CD"},
	"pc-fx":             {"PC-FX", "NEC PC-FX"},
	"ngpc":              {"Neo Geo Pocket Color", "SNK handheld"},
	"ws":                {"WonderSwan", "Bandai WonderSwan"},
	"wsc":               {"WonderSwan Color", "Bandai WonderSwan Color"},
	"poke-mini":         {"Pokémon Mini", "Pokémon Mini"},
	"game-watch":        {"Game & Watch", "Nintendo Game & Watch"},
	"dos":               {"DOS / PC", "Classic PC games; use the common English title"},
	"naomi":             {"NAOMI", "Sega NAOMI arcade; use the English arcade title, not MAME short names"},
	"fbneo-act":         {"Arcade beat 'em up (FBNeo)", "Use the English arcade title, not the MAME romset name"},
	"fbneo-act-v":       {"Arcade beat 'em up vertical (FBNeo)", "Vertical arcade action"},
	"fbneo-act-hack":    {"Arcade beat 'em up hack (FBNeo)", "Arcade action hack"},
	"fbneo-stg":         {"Arcade shooter (FBNeo)", "Use English shmup titles"},
	"fbneo-stg-v":       {"Arcade shooter vertical (FBNeo)", "Vertical shmup"},
	"fbneo-stg-hack":    {"Arcade shooter hack (FBNeo)", "Shmup hack"},
	"fbneo-ftg":         {"Arcade fighter (FBNeo)", "e.g. 拳皇 → The King of Fighters, 街霸 → Street Fighter"},
	"fbneo-ftg-hack":    {"Arcade fighter hack (FBNeo)", "Fighter hack"},
	"fbneo-fly":         {"Arcade flight (FBNeo)", "Flight / aerial combat arcade"},
	"fbneo-fly-v":       {"Arcade flight vertical (FBNeo)", "Vertical flight arcade"},
	"fbneo-etc":         {"Arcade misc (FBNeo)", "Miscellaneous arcade"},
	"fbneo-etc-v":       {"Arcade misc vertical (FBNeo)", "Vertical miscellaneous arcade"},
	"fbneo-spo":         {"Arcade sports (FBNeo)", "Sports arcade"},
	"fbneo-rac":         {"Arcade racing (FBNeo)", "Racing arcade"},
	"mame-act":          {"Arcade beat 'em up (MAME)", "Use English arcade titles, not MAME short names"},
	"mame-stg":          {"Arcade shooter (MAME)", "Shmup"},
	"mame-stg-v":        {"Arcade shooter vertical (MAME)", "Vertical shmup"},
	"mame-ftg":          {"Arcade fighter (MAME)", "Fighting games"},
	"mame-ftg-hack":     {"Arcade fighter hack (MAME)", "Fighter hack"},
	"mame-fly":          {"Arcade flight (MAME)", "Flight / aerial combat"},
	"mame-fly-v":        {"Arcade flight vertical (MAME)", "Vertical flight"},
	"mame-etc":          {"Arcade misc (MAME)", "Miscellaneous arcade"},
	"mame-spo":          {"Arcade sports (MAME)", "Sports arcade"},
	"mame-rac":          {"Arcade racing (MAME)", "Racing arcade"},
}

func systemPrompt(platformHint string) string {
	return fmt.Sprintf(`You convert Chinese retro-game titles into the English titles used by Western scrapers (IGDB, SteamGridDB, ScreenScraper name search).

Current platform: %s

Rules:
1. Output the commonly used English / international game title. Prefer the name a scraper would match, not a literal word-for-word translation.
2. Do not add platform names, region codes, dump tags, disc numbers, or file extensions.
3. If the Chinese name is a well-known localized title, map it to the original English title (e.g. 魂斗罗 → Contra, 超级马里奥世界 → Super Mario World, 拳皇 97 → The King of Fighters '97).
4. If already a suitable English title, return it unchanged.
5. Reply with JSON only. No markdown, no commentary.`, platformHint)
}
