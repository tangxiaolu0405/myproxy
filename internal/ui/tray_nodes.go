package ui

import (
	"sort"

	"myproxy.com/p/internal/model"
)

const trayTopAlternativesCount = 5

type trayNodeEntry struct {
	id    string
	name  string
	delay int
}

// pickTopAlternatives 从测速结果中选取延迟最低的前 n 个节点，排除当前节点与无效延迟。
func pickTopAlternatives(servers []model.Node, delays map[string]int, currentID string, n int) []trayNodeEntry {
	candidates := make([]trayNodeEntry, 0, len(servers))
	for _, s := range servers {
		if s.ID == currentID {
			continue
		}
		delay, ok := delays[s.ID]
		if !ok || delay <= 0 {
			continue
		}
		candidates = append(candidates, trayNodeEntry{id: s.ID, name: s.Name, delay: delay})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].delay < candidates[j].delay
	})
	if len(candidates) > n {
		candidates = candidates[:n]
	}
	return candidates
}
