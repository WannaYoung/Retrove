package pegasus

import (
	"bufio"
	"os"
	"strings"
)

type Collection struct {
	Name       string
	SortBy     string
	Extensions string
	Launch     string
	Games      []Game
}

type Game struct {
	Title       string
	File        string
	SortBy      string
	Developer   string
	Description string
}

func ParseFile(path string) (Collection, error) {
	f, err := os.Open(path)
	if err != nil {
		return Collection{}, err
	}
	defer f.Close()

	var col Collection
	var cur *Game
	var launchLines []string
	inLaunch := false

	flushLaunch := func() {
		if inLaunch {
			col.Launch = strings.TrimSpace(strings.Join(launchLines, "\n"))
			launchLines = nil
			inLaunch = false
		}
	}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if inLaunch {
				launchLines = append(launchLines, strings.TrimSpace(line))
			}
			continue
		}
		key, val, ok := strings.Cut(trimmed, ":")
		if !ok {
			if inLaunch {
				launchLines = append(launchLines, trimmed)
			}
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		switch key {
		case "collection":
			flushLaunch()
			col.Name = val
		case "sort-by":
			if cur == nil {
				col.SortBy = val
			} else {
				cur.SortBy = val
			}
		case "extensions":
			flushLaunch()
			col.Extensions = val
		case "launch":
			flushLaunch()
			inLaunch = true
			if val != "" {
				launchLines = append(launchLines, val)
			}
		case "game":
			flushLaunch()
			if cur != nil {
				col.Games = append(col.Games, *cur)
			}
			cur = &Game{Title: val}
		case "file":
			if cur != nil {
				cur.File = val
			}
		case "developer":
			if cur != nil {
				cur.Developer = val
			}
		case "description":
			if cur != nil {
				cur.Description = val
			}
		default:
			if inLaunch {
				launchLines = append(launchLines, trimmed)
			}
		}
	}
	flushLaunch()
	if cur != nil {
		col.Games = append(col.Games, *cur)
	}
	return col, sc.Err()
}
