// cc-econ: mission control for your Claude Code token economics.
//
// It parses the raw session transcripts Claude Code writes to
// ~/.claude/projects/**/*.jsonl (the ground truth: every assistant turn
// carries message.usage + message.model + a timestamp), prices each turn with
// real per-model rates, and reports a spend breakdown by day / project /
// model, plus what prompt caching saved you.
//
//	go run .            # CLI summary
//	go run . -serve :7777   # local dashboard at http://localhost:7777
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
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

// ---- report (JSON-serializable, shared by CLI and dashboard) ----
type nameRow struct {
	Name string  `json:"name"`
	Cost float64 `json:"cost"`
}
type sessRow struct {
	Session string  `json:"session"`
	Project string  `json:"project"`
	Cost    float64 `json:"cost"`
}
type report struct {
	Files        int       `json:"files"`
	Total        float64   `json:"total"`
	Month        float64   `json:"month"`
	Today        float64   `json:"today"`
	CacheSavings float64   `json:"cacheSavings"`
	Turns        int       `json:"turns"`
	In           int       `json:"in"`
	Out          int       `json:"out"`
	CW           int       `json:"cw"`
	CR           int       `json:"cr"`
	ByDay        []nameRow `json:"byDay"` // chronological
	ByProject    []nameRow `json:"byProject"`
	ByModel      []nameRow `json:"byModel"`
	TopSessions  []sessRow `json:"topSessions"`
}

func analyze(dir string) (*report, error) {
	files, _ := filepath.Glob(filepath.Join(dir, "*", "*.jsonl"))
	if len(files) == 0 {
		return nil, fmt.Errorf("no transcripts under %s", dir)
	}
	r := &report{Files: len(files)}
	day := map[string]float64{}
	proj := map[string]float64{}
	model := map[string]float64{}
	sess := map[string]*sessRow{}

	price := func(u usage, rt rate) float64 {
		return float64(u.Input)/1e6*rt.in + float64(u.Output)/1e6*rt.out +
			float64(u.CacheWrite)/1e6*rt.cacheWrite + float64(u.CacheRead)/1e6*rt.cacheRead
	}

	for _, f := range files {
		project := filepath.Base(filepath.Dir(f))
		session := strings.TrimSuffix(filepath.Base(f), ".jsonl")
		fh, err := os.Open(f)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(fh)
		sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
		for sc.Scan() {
			line := sc.Bytes()
			if len(line) == 0 {
				continue
			}
			var e entry
			if json.Unmarshal(line, &e) != nil || e.Type != "assistant" {
				continue
			}
			rt, ok := rateFor(e.Message.Model)
			if !ok {
				continue
			}
			u := e.Message.Usage
			c := price(u, rt)

			r.Total += c
			r.CacheSavings += float64(u.CacheRead) / 1e6 * (rt.in - rt.cacheRead)
			r.Turns++
			r.In += u.Input
			r.Out += u.Output
			r.CW += u.CacheWrite
			r.CR += u.CacheRead

			day[e.Timestamp.Local().Format("2006-01-02")] += c
			proj[project] += c
			model[e.Message.Model] += c
			if sess[session] == nil {
				sess[session] = &sessRow{Session: session, Project: project}
			}
			sess[session].Cost += c
		}
		fh.Close()
	}

	now := time.Now()
	monthPrefix := now.Format("2006-01")
	today := now.Format("2006-01-02")
	for d, c := range day {
		if strings.HasPrefix(d, monthPrefix) {
			r.Month += c
		}
		if d == today {
			r.Today += c
		}
		r.ByDay = append(r.ByDay, nameRow{d, c})
	}
	sort.Slice(r.ByDay, func(i, j int) bool { return r.ByDay[i].Name < r.ByDay[j].Name })

	r.ByProject = sortedRows(proj)
	r.ByModel = sortedRows(model)
	for _, s := range sess {
		r.TopSessions = append(r.TopSessions, *s)
	}
	sort.Slice(r.TopSessions, func(i, j int) bool { return r.TopSessions[i].Cost > r.TopSessions[j].Cost })
	if len(r.TopSessions) > 12 {
		r.TopSessions = r.TopSessions[:12]
	}
	return r, nil
}

func sortedRows(m map[string]float64) []nameRow {
	var s []nameRow
	for k, v := range m {
		s = append(s, nameRow{k, v})
	}
	sort.Slice(s, func(i, j int) bool { return s[i].Cost > s[j].Cost })
	return s
}

func main() {
	def := "/mnt/c/Users/aamanbek/.claude/projects"
	if _, err := os.Stat(def); err != nil {
		home, _ := os.UserHomeDir()
		def = filepath.Join(home, ".claude", "projects")
	}
	dir := flag.String("dir", def, "Claude Code projects directory")
	serve := flag.String("serve", "", "serve dashboard at this address, e.g. :7777")
	flag.Parse()

	if *serve != "" {
		http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
			r, err := analyze(*dir)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			b, _ := json.Marshal(r)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(strings.Replace(dashboardHTML, "__DATA__", string(b), 1)))
		})
		fmt.Printf("cc-econ dashboard: http://localhost%s  (reading %s)\n", *serve, *dir)
		if err := http.ListenAndServe(*serve, nil); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		return
	}

	r, err := analyze(*dir)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	printCLI(r)
}

func printCLI(r *report) {
	fmt.Printf("=== Claude Code economics (%d transcripts) ===\n\n", r.Files)
	fmt.Printf("  TOTAL spend   : $%8.2f   (%s turns)\n", r.Total, commas(r.Turns))
	fmt.Printf("  this month    : $%8.2f\n", r.Month)
	fmt.Printf("  today         : $%8.2f\n", r.Today)
	fmt.Printf("  cache savings : $%8.2f\n\n", r.CacheSavings)
	fmt.Printf("  tokens: in %s | out %s | cache-write %s | cache-read %s\n\n",
		commas(r.In), commas(r.Out), commas(r.CW), commas(r.CR))

	byDayDesc := append([]nameRow(nil), r.ByDay...)
	sort.Slice(byDayDesc, func(i, j int) bool { return byDayDesc[i].Name > byDayDesc[j].Name })
	printRows("by day (recent)", byDayDesc, 10)
	printRows("by project", r.ByProject, 10)
	printRows("by model", r.ByModel, 10)

	fmt.Println("--- top costly sessions ---")
	for i, s := range r.TopSessions {
		if i >= 8 {
			break
		}
		fmt.Printf("  $%8.2f  %-22s  %s\n", s.Cost, trunc(s.Project, 22), s.Session[:8])
	}
	fmt.Println()
}

func printRows(title string, rows []nameRow, n int) {
	fmt.Printf("--- %s ---\n", title)
	for i, e := range rows {
		if i >= n {
			break
		}
		fmt.Printf("  %-28s $%9.2f\n", trunc(e.Name, 28), e.Cost)
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
