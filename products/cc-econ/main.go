// cc-econ: mission control for your Claude Code token economics.
//
// It parses the raw session transcripts Claude Code writes to
// ~/.claude/projects/**/*.jsonl (the ground truth: every assistant turn
// carries message.usage + message.model + a timestamp), prices each turn with
// real per-model rates, and prints a spend breakdown by day / project /
// model, plus what prompt caching saved you.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ---- pricing (USD per 1M tokens) ----
// cacheWrite = 1.25x input (5-minute ephemeral, Claude Code's default),
// cacheRead = 0.1x input. Opus 4.8 has no >200K long-context premium.
type rate struct{ in, out, cacheWrite, cacheRead float64 }

var rates = map[string]rate{
	"opus":   {in: 5.0, out: 25.0, cacheWrite: 6.25, cacheRead: 0.50},
	"sonnet": {in: 3.0, out: 15.0, cacheWrite: 3.75, cacheRead: 0.30},
	"haiku":  {in: 1.0, out: 5.0, cacheWrite: 1.25, cacheRead: 0.10},
}

func rateFor(model string) (rate, bool) {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "opus"):
		return rates["opus"], true
	case strings.Contains(m, "haiku"):
		return rates["haiku"], true
	case strings.Contains(m, "sonnet"):
		return rates["sonnet"], true
	default:
		return rate{}, false // synthetic / unknown — skip
	}
}

// ---- JSONL shapes we care about ----
type usage struct {
	Input      int `json:"input_tokens"`
	Output     int `json:"output_tokens"`
	CacheWrite int `json:"cache_creation_input_tokens"`
	CacheRead  int `json:"cache_read_input_tokens"`
}

type entry struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Message   struct {
		Model string `json:"model"`
		Usage usage  `json:"usage"`
	} `json:"message"`
}

type agg struct {
	cost            float64
	in, out, cw, cr int
	turns           int
}

func (a *agg) add(u usage, r rate) {
	a.cost += float64(u.Input)/1e6*r.in +
		float64(u.Output)/1e6*r.out +
		float64(u.CacheWrite)/1e6*r.cacheWrite +
		float64(u.CacheRead)/1e6*r.cacheRead
	a.in += u.Input
	a.out += u.Output
	a.cw += u.CacheWrite
	a.cr += u.CacheRead
	a.turns++
}

func main() {
	home, _ := os.UserHomeDir()
	// Claude Code data lives on the Windows side; from WSL that's /mnt/c/...
	def := "/mnt/c/Users/aamanbek/.claude/projects"
	if _, err := os.Stat(def); err != nil {
		def = filepath.Join(home, ".claude", "projects")
	}
	dir := flag.String("dir", def, "Claude Code projects directory")
	flag.Parse()

	var total agg
	byDay := map[string]*agg{}
	byProject := map[string]*agg{}
	byModel := map[string]*agg{}
	bySession := map[string]*agg{}
	sessionProject := map[string]string{}
	var cacheSavings float64 // vs paying full input rate for cache-read tokens

	files, _ := filepath.Glob(filepath.Join(*dir, "*", "*.jsonl"))
	if len(files) == 0 {
		fmt.Printf("no transcripts under %s\n", *dir)
		os.Exit(1)
	}

	for _, f := range files {
		project := filepath.Base(filepath.Dir(f))
		session := strings.TrimSuffix(filepath.Base(f), ".jsonl")
		fh, err := os.Open(f)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(fh)
		sc.Buffer(make([]byte, 1024*1024), 16*1024*1024) // lines can be big
		for sc.Scan() {
			line := sc.Bytes()
			if len(line) == 0 {
				continue
			}
			var e entry
			if json.Unmarshal(line, &e) != nil || e.Type != "assistant" {
				continue
			}
			r, ok := rateFor(e.Message.Model)
			if !ok {
				continue
			}
			u := e.Message.Usage

			total.add(u, r)
			cacheSavings += float64(u.CacheRead) / 1e6 * (r.in - r.cacheRead)

			get := func(m map[string]*agg, k string) *agg {
				if m[k] == nil {
					m[k] = &agg{}
				}
				return m[k]
			}
			day := e.Timestamp.Local().Format("2006-01-02")
			get(byDay, day).add(u, r)
			get(byProject, project).add(u, r)
			get(byModel, e.Message.Model).add(u, r)
			get(bySession, session).add(u, r)
			sessionProject[session] = project
		}
		fh.Close()
	}

	now := time.Now()
	monthPrefix := now.Format("2006-01")
	today := now.Format("2006-01-02")
	var monthCost, todayCost float64
	for day, a := range byDay {
		if strings.HasPrefix(day, monthPrefix) {
			monthCost += a.cost
		}
		if day == today {
			todayCost += a.cost
		}
	}

	fmt.Printf("=== Claude Code economics (%d transcripts) ===\n\n", len(files))
	fmt.Printf("  TOTAL spend   : $%8.2f   (%s turns)\n", total.cost, commas(total.turns))
	fmt.Printf("  this month    : $%8.2f\n", monthCost)
	fmt.Printf("  today         : $%8.2f\n", todayCost)
	fmt.Printf("  cache savings : $%8.2f   (cache-reads billed at 0.1x instead of full input)\n\n", cacheSavings)

	fmt.Printf("  tokens: in %s | out %s | cache-write %s | cache-read %s\n\n",
		commas(total.in), commas(total.out), commas(total.cw), commas(total.cr))

	printTop("by day (recent)", byDay, 10, true)
	printTop("by project", byProject, 10, false)
	printTop("by model", byModel, 10, false)
	printTopSessions("top costly sessions", bySession, sessionProject, 8)
}

func printTop(title string, m map[string]*agg, n int, chrono bool) {
	type kv struct {
		k string
		a *agg
	}
	var s []kv
	for k, a := range m {
		s = append(s, kv{k, a})
	}
	if chrono {
		sort.Slice(s, func(i, j int) bool { return s[i].k > s[j].k })
	} else {
		sort.Slice(s, func(i, j int) bool { return s[i].a.cost > s[j].a.cost })
	}
	fmt.Printf("--- %s ---\n", title)
	for i, e := range s {
		if i >= n {
			break
		}
		fmt.Printf("  %-28s $%9.2f\n", trunc(e.k, 28), e.a.cost)
	}
	fmt.Println()
}

func printTopSessions(title string, m map[string]*agg, proj map[string]string, n int) {
	type kv struct {
		k string
		a *agg
	}
	var s []kv
	for k, a := range m {
		s = append(s, kv{k, a})
	}
	sort.Slice(s, func(i, j int) bool { return s[i].a.cost > s[j].a.cost })
	fmt.Printf("--- %s ---\n", title)
	for i, e := range s {
		if i >= n {
			break
		}
		fmt.Printf("  $%8.2f  %-22s  %s\n", e.a.cost, trunc(proj[e.k], 22), e.k[:8])
	}
	fmt.Println()
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func commas(n int) string {
	s := fmt.Sprintf("%d", n)
	if n < 0 {
		return s
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}
