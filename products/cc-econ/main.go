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
	Prefix  int     `json:"prefix"` // first-turn cache-write tokens ~ loaded context
}
type insight struct {
	Level string `json:"level"` // good | watch | info
	Text  string `json:"text"`
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

	// cost composition (for insights)
	CostIn  float64 `json:"costIn"`
	CostOut float64 `json:"costOut"`
	CostCW  float64 `json:"costCW"`
	CostCR  float64 `json:"costCR"`

	Sessions int `json:"sessions"`

	// derived
	MonthProjected float64   `json:"monthProjected"` // run-rate to end of month
	YearRate       float64   `json:"yearRate"`
	CacheHitRate   float64   `json:"cacheHitRate"`   // 0..1
	CacheChurnPct  float64   `json:"cacheChurnPct"`  // cache-write cost / total
	OutputSharePct float64   `json:"outputSharePct"` // output cost / total
	IfSonnet       float64   `json:"ifSonnet"`       // same tokens, Sonnet rates
	IfHaiku        float64   `json:"ifHaiku"`
	PrefixTax      float64   `json:"prefixTax"`    // avg cache-write $ per session
	PrefixMedian   int       `json:"prefixMedian"` // median first-turn cache-write tokens
	Insights       []insight `json:"insights"`

	KZT float64 `json:"kzt"` // USD->KZT rate for tenge display
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
			cIn := float64(u.Input) / 1e6 * rt.in
			cOut := float64(u.Output) / 1e6 * rt.out
			cCW := float64(u.CacheWrite) / 1e6 * rt.cacheWrite
			cCR := float64(u.CacheRead) / 1e6 * rt.cacheRead
			c := cIn + cOut + cCW + cCR

			r.Total += c
			r.CostIn += cIn
			r.CostOut += cOut
			r.CostCW += cCW
			r.CostCR += cCR
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
				sess[session] = &sessRow{Session: session, Project: project, Prefix: u.CacheWrite}
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
	r.Sessions = len(sess)
	var prefixes []int
	for _, s := range sess {
		if s.Prefix > 0 {
			prefixes = append(prefixes, s.Prefix)
		}
	}
	sort.Ints(prefixes)
	if n := len(prefixes); n > 0 {
		r.PrefixMedian = prefixes[n/2]
	}
	if len(r.TopSessions) > 12 {
		r.TopSessions = r.TopSessions[:12]
	}

	// run-rate projection to end of month
	daysElapsed := float64(now.Day())
	daysInMonth := float64(time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day())
	if daysElapsed > 0 {
		r.MonthProjected = r.Month / daysElapsed * daysInMonth
	}
	r.YearRate = r.MonthProjected * 12

	// efficiency metrics
	if r.CR+r.In > 0 {
		r.CacheHitRate = float64(r.CR) / float64(r.CR+r.In)
	}
	if r.Total > 0 {
		r.CacheChurnPct = r.CostCW / r.Total * 100
		r.OutputSharePct = r.CostOut / r.Total * 100
	}
	// what-if: same tokens, different model rates (cache rates scale with input)
	rescale := func(rt rate) float64 {
		return float64(r.In)/1e6*rt.in + float64(r.Out)/1e6*rt.out +
			float64(r.CW)/1e6*rt.cacheWrite + float64(r.CR)/1e6*rt.cacheRead
	}
	r.IfSonnet = rescale(rates["sonnet"])
	r.IfHaiku = rescale(rates["haiku"])
	if r.Sessions > 0 {
		r.PrefixTax = r.CostCW / float64(r.Sessions)
	}
	r.Insights = buildInsights(r)
	return r, nil
}

func money(x float64) string { return "$" + fmt.Sprintf("%.2f", x) }

func buildInsights(r *report) []insight {
	var out []insight
	add := func(lvl, s string) { out = append(out, insight{lvl, s}) }
	pct := func(f float64) string { return fmt.Sprintf("%.0f%%", f) }
	hit := r.CacheHitRate * 100

	switch {
	case hit >= 85:
		add("good", fmt.Sprintf("Cache hit rate %s — healthy. The context prefix stays stable across turns, so caching earns its keep (saved %s).", pct(hit), money(r.CacheSavings)))
	case hit >= 60:
		add("watch", fmt.Sprintf("Cache hit rate %s — some slack. Something early in the prefix changes between turns; check for timestamps/IDs before the last cache breakpoint.", pct(hit)))
	default:
		add("watch", fmt.Sprintf("Cache hit rate only %s — the prefix breaks often. Hunt for volatile content (dates, UUIDs, unsorted JSON) high in the context.", pct(hit)))
	}

	if r.CacheChurnPct >= 15 {
		add("watch", fmt.Sprintf("Cache-write is %s of spend — you pay the 1.25x write premium a lot. A large or frequently-changing prefix (heavy plugin stack) drives this.", pct(r.CacheChurnPct)))
	} else {
		add("good", fmt.Sprintf("Cache-write only %s of spend — prefix is stable, writes amortize well.", pct(r.CacheChurnPct)))
	}

	if r.OutputSharePct >= 30 {
		add("watch", fmt.Sprintf("Output is %s of spend (bills at 5x input). Lower effort or terser replies (caveman mode) is the lever.", pct(r.OutputSharePct)))
	} else {
		add("info", fmt.Sprintf("Output is %s of spend — the bill is dominated by context, not generation.", pct(r.OutputSharePct)))
	}

	if r.IfSonnet > 0 && r.Total > 0 {
		sv := (1 - r.IfSonnet/r.Total) * 100
		hv := (1 - r.IfHaiku/r.Total) * 100
		add("info", fmt.Sprintf("Model lever: the same tokens on Sonnet ~%s (-%.0f%%), on Haiku ~%s (-%.0f%%). A quality tradeoff, not free — the price of staying on Opus.", money(r.IfSonnet), sv, money(r.IfHaiku), hv))
	}

	if r.PrefixTax > 0 {
		add("info", fmt.Sprintf("Prefix tax: ~%s per session goes to cache-writes before any real work — the cost of your loaded context/plugin stack.", money(r.PrefixTax)))
	}
	if r.PrefixMedian > 0 {
		add("info", fmt.Sprintf("Loaded prefix ~%s tokens/session (system + tools + plugin stack) written to cache at each start. Trimming the stack shrinks this — the experiment lever.", commas(r.PrefixMedian)))
	}
	return out
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
	kzt := flag.Float64("kzt", 472.0, "USD->KZT fallback rate (0 to hide tenge)")
	fx := flag.Bool("fx", true, "fetch live USD->KZT rate (fallback to -kzt)")
	flag.Parse()

	rateKZT := *kzt
	if *fx && *kzt > 0 {
		rateKZT = liveKZT(*kzt)
	}

	if *serve != "" {
		http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
			r, err := analyze(*dir)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			r.KZT = rateKZT
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
	r.KZT = rateKZT
	printCLI(r)
}

func tenge(usd, rate float64) string {
	if rate <= 0 {
		return ""
	}
	return "₸" + commas(int(usd*rate))
}

// liveKZT fetches the current USD->KZT rate; returns fallback on any error.
func liveKZT(fallback float64) float64 {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("https://open.er-api.com/v6/latest/USD")
	if err != nil {
		return fallback
	}
	defer resp.Body.Close()
	var out struct {
		Rates map[string]float64 `json:"rates"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil {
		return fallback
	}
	if r, ok := out.Rates["KZT"]; ok && r > 0 {
		return r
	}
	return fallback
}

func printCLI(r *report) {
	fmt.Printf("=== Claude Code economics (%d transcripts) ===\n", r.Files)
	fmt.Printf("    shadow price (Max plan): API-list-equivalent, not a bill · USD/KZT %.1f\n\n", r.KZT)
	fmt.Printf("  TOTAL spend   : $%8.2f   %-12s (%s turns)\n", r.Total, tenge(r.Total, r.KZT), commas(r.Turns))
	fmt.Printf("  this month    : $%8.2f   %s\n", r.Month, tenge(r.Month, r.KZT))
	fmt.Printf("  today         : $%8.2f   %s\n", r.Today, tenge(r.Today, r.KZT))
	fmt.Printf("  proj. month   : $%8.2f   %-12s (~$%s/yr)\n", r.MonthProjected, tenge(r.MonthProjected, r.KZT), commas(int(r.YearRate)))
	fmt.Printf("  cache savings : $%8.2f   (hit rate %.0f%%)\n", r.CacheSavings, r.CacheHitRate*100)
	fmt.Printf("  loaded prefix : ~%s tok/session (system+tools+plugins)\n\n", commas(r.PrefixMedian))
	fmt.Printf("  tokens: in %s | out %s | cache-write %s | cache-read %s\n\n",
		commas(r.In), commas(r.Out), commas(r.CW), commas(r.CR))

	byDayDesc := append([]nameRow(nil), r.ByDay...)
	sort.Slice(byDayDesc, func(i, j int) bool { return byDayDesc[i].Name > byDayDesc[j].Name })
	printRows("by day (recent)", byDayDesc, 10)
	printRows("by project", r.ByProject, 10)
	printRows("by model", r.ByModel, 10)

	fmt.Println("--- top costly sessions (cost · prefix tokens) ---")
	for i, s := range r.TopSessions {
		if i >= 8 {
			break
		}
		fmt.Printf("  $%8.2f  %9s tok  %-20s  %s\n", s.Cost, commas(s.Prefix), trunc(s.Project, 20), s.Session[:8])
	}
	fmt.Println()

	fmt.Println("--- insights ---")
	for _, s := range r.Insights {
		fmt.Printf("  • %s\n", s.Text)
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
