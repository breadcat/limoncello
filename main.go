package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed static
var staticFiles embed.FS

var volumes = []int{25, 125, 187, 284, 330, 375, 440, 500, 568, 660, 750}

func calcUnits(ml int, abv float64) float64 {
	return float64(ml) * abv / 1000.0
}

// Data structures

// LoggedDrink stores enough to recalculate units at any time.
type LoggedDrink struct {
	VolumeMl int     `json:"volume_ml"`
	ABV      float64 `json:"abv"`
	Count    int     `json:"count"`
}

func (d LoggedDrink) Units() float64    { return calcUnits(d.VolumeMl, d.ABV) * float64(d.Count) }
func (d LoggedDrink) UnitEach() float64 { return calcUnits(d.VolumeMl, d.ABV) }
func (d LoggedDrink) Key() string       { return fmt.Sprintf("%d@%.2f", d.VolumeMl, d.ABV) }
func (d LoggedDrink) Label() string     { return fmt.Sprintf("%dml @ %.1f%%", d.VolumeMl, d.ABV) }

type DayLog struct {
	Date   string        `json:"date"` // YYYY-MM-DD
	Drinks []LoggedDrink `json:"drinks"`
}

// Database now only holds day logs — no drink type catalogue.
type Database struct {
	DayLogs []DayLog `json:"day_logs"`
}

var (
	dbPath string
	db     Database
)

// Persistence

func loadDB() error {
	data, err := os.ReadFile(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			db = Database{DayLogs: []DayLog{}}
			return saveDB()
		}
		return err
	}
	return json.Unmarshal(data, &db)
}

func saveDB() error {
	var lines []string
	for _, day := range db.DayLogs {
		b, err := json.Marshal(day)
		if err != nil {
			return err
		}
		lines = append(lines, "    "+string(b))
	}
	var sb strings.Builder
	sb.WriteString("{\n  \"day_logs\": [")
	if len(lines) > 0 {
		sb.WriteString("\n")
		sb.WriteString(strings.Join(lines, ",\n"))
		sb.WriteString("\n  ")
	}
	sb.WriteString("]\n}\n")
	return os.WriteFile(dbPath, []byte(sb.String()), 0644)
}

// Day log helpers

func totalUnits(day DayLog) float64 {
	var t float64
	for _, d := range day.Drinks {
		t += d.Units()
	}
	return t
}

func findDayLog(date string) *DayLog {
	for i := range db.DayLogs {
		if db.DayLogs[i].Date == date {
			return &db.DayLogs[i]
		}
	}
	return nil
}

func ensureDayLog(date string) *DayLog {
	for i := range db.DayLogs {
		if db.DayLogs[i].Date == date {
			return &db.DayLogs[i]
		}
	}
	db.DayLogs = append(db.DayLogs, DayLog{Date: date})
	return &db.DayLogs[len(db.DayLogs)-1]
}

func dayUnits(date string) float64 {
	dl := findDayLog(date)
	if dl == nil {
		return 0
	}
	return totalUnits(*dl)
}

func sortDayLogs() {
	sort.Slice(db.DayLogs, func(i, j int) bool {
		return db.DayLogs[i].Date < db.DayLogs[j].Date
	})
}

// Summary

func renderSummary() string {
	now := time.Now()
	today := now.Format("2006-01-02")

	var totalUnitsVal float64
	totalDrinks := 0
	freeDays := 0

	for i := 1; i <= 7; i++ {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		dl := findDayLog(date)
		if dl == nil || len(dl.Drinks) == 0 {
			freeDays++
		} else {
			for _, d := range dl.Drinks {
				totalDrinks += d.Count
				totalUnitsVal += d.Units()
			}
		}
	}

	// Also include today if it has drinks
	if dl := findDayLog(today); dl != nil {
		for _, d := range dl.Drinks {
			totalDrinks += d.Count
			totalUnitsVal += d.Units()
		}
	}

	return `<div class="summary-grid">` +
		`<div class="summary-card"><span class="summary-val">` + strconv.Itoa(totalDrinks) + `</span><span class="summary-label">drinks</span></div>` +
		`<div class="summary-card"><span class="summary-val">` + formatUnits(totalUnitsVal) + `</span><span class="summary-label">units</span></div>` +
		`<div class="summary-card"><span class="summary-val">` + strconv.Itoa(freeDays) + `<span class="summary-val-sub">/7</span></span><span class="summary-label">drink-free days</span></div>` +
		`</div>`
}

// Tile rendering

func dateColorClass(units float64, date string) string {
	today := time.Now().Format("2006-01-02")
	if date < today && units == 0 {
		return "empty-past"
	}
	switch {
	case units == 0:
		return "zero"
	case units < 2:
		return "blue"
	case units < 4:
		return "green"
	case units < 8:
		return "yellow"
	case units < 14:
		return "orange"
	case units < 20:
		return "red"
	case units < 30:
		return "purple"
	default:
		return "black"
	}
}

func formatUnits(u float64) string {
	if u == math.Trunc(u) {
		return fmt.Sprintf("%.0f", u)
	}
	return fmt.Sprintf("%.1f", u)
}

func renderTile(date, label string, units float64) string {
	cls := dateColorClass(units, date)
	todayCls := ""
	if date == time.Now().Format("2006-01-02") {
		todayCls = " today"
	}
	unitsSpan := ""
	if units > 0 {
		unitsSpan = fmt.Sprintf(`<span class="units">%s u</span>`, formatUnits(units))
	}
	return fmt.Sprintf(
		`<div class="tile %s%s" onclick="openDay('%s')" title="%s"><span class="date-label">%s</span>%s</div>`,
		cls, todayCls, date, date, label, unitsSpan,
	)
}

// Views

func renderDaysRow(offset int) string {
	today, _ := time.Parse("2006-01-02", time.Now().Format("2006-01-02"))
	var b strings.Builder
	start := today.AddDate(0, 0, -offset-2)
	for i := 0; i < 5; i++ {
		d := start.AddDate(0, 0, i)
		ds := d.Format("2006-01-02")
		b.WriteString(renderTile(ds, d.Format("Mon 2"), dayUnits(ds)))
	}
	return b.String()
}

func renderWeekRow(weekOffset int) string {
	now := time.Now()
	wd := int(now.Weekday())
	if wd == 0 {
		wd = 7
	}
	monday := now.AddDate(0, 0, -(wd-1)-(weekOffset*7))
	var b strings.Builder
	for i := 0; i < 7; i++ {
		d := monday.AddDate(0, 0, i)
		ds := d.Format("2006-01-02")
		b.WriteString(renderTile(ds, d.Format("Mon 2"), dayUnits(ds)))
	}
	return b.String()
}

func renderMonthGrid(monthOffset int) string {
	now := time.Now()
	first := time.Date(now.Year(), now.Month()-time.Month(monthOffset), 1, 0, 0, 0, 0, time.Local)
	last := first.AddDate(0, 1, -1)

	var b strings.Builder
	b.WriteString(`<div class="cal-grid">`)
	for _, h := range []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"} {
		b.WriteString(fmt.Sprintf(`<div class="cal-header">%s</div>`, h))
	}
	startWd := int(first.Weekday())
	if startWd == 0 {
		startWd = 7
	}
	for i := 1; i < startWd; i++ {
		b.WriteString(`<div class="cal-empty"></div>`)
	}
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		ds := d.Format("2006-01-02")
		b.WriteString(renderTile(ds, d.Format("2"), dayUnits(ds)))
	}
	b.WriteString(`</div>`)
	return b.String()
}

func weekLabel(offset int) string {
	now := time.Now()
	wd := int(now.Weekday())
	if wd == 0 {
		wd = 7
	}
	monday := now.AddDate(0, 0, -(wd-1)-(offset*7))
	sunday := monday.AddDate(0, 0, 6)
	return monday.Format("2 Jan") + " – " + sunday.Format("2 Jan")
}

func monthLabel(offset int) string {
	now := time.Now()
	return time.Date(now.Year(), now.Month()-time.Month(offset), 1, 0, 0, 0, 0, time.Local).Format("January 2006")
}

// Modal

func renderModal(date string) string {
	dl := findDayLog(date)
	t, _ := time.Parse("2006-01-02", date)

	// Logged drinks table
	var loggedHTML strings.Builder
	if dl != nil && len(dl.Drinks) > 0 {
		loggedHTML.WriteString(`<table class="drink-table">
<tr><th>Drink</th><th>×</th><th>Units</th><th></th></tr>`)
		for _, drink := range dl.Drinks {
			key := drink.Key()
			loggedHTML.WriteString(fmt.Sprintf(`
<tr>
  <td>%s</td>
  <td>%d</td>
  <td>%.2f</td>
  <td class="actions">
    <button class="btn-sm btn-minus" onclick="adjustDrink('%s','%s',-1)">−</button>
    <button class="btn-sm btn-plus"  onclick="adjustDrink('%s','%s', 1)">+</button>
    <button class="btn-sm btn-del"   onclick="removeDrink('%s','%s')">✕</button>
  </td>
</tr>`,
				drink.Label(), drink.Count, drink.Units(),
				date, key,
				date, key,
				date, key,
			))
		}
		loggedHTML.WriteString(`</table>`)
	} else {
		loggedHTML.WriteString(`<p class="no-drinks">No drinks logged.</p>`)
	}

	// Volume options
	var volOpts strings.Builder
	for _, v := range volumes {
		volOpts.WriteString(fmt.Sprintf(`<option value="%d">%dml</option>`, v, v))
	}

	totalU := 0.0
	if dl != nil {
		totalU = totalUnits(*dl)
	}

	// Use concatenation — never fmt.Sprintf — so that % signs inside
	// loggedHTML (e.g. "5.0%" in drink labels) can't corrupt the output.
	return `<div class="modal-overlay" id="day-modal" onclick="closeModal(event)">` +
		`<div class="modal-box">` +
		`<h2>` + t.Format("Monday 2 January 2006") + `</h2>` +
		`<p class="total-units">Total: <strong>` + formatUnits(totalU) + ` units</strong></p>` +
		`<div id="logged-drinks">` + loggedHTML.String() + `</div>` +
		`<div class="add-drink-form">` +
		`<h3>Add a drink</h3>` +
		`<div class="add-drink-row">` +
		`<div class="add-field"><label>Volume</label><select id="drink-volume">` + volOpts.String() + `</select></div>` +
		`<div class="add-field"><label>ABV %</label>` +
		`<input id="drink-abv" type="number" min="0.1" max="99" step="0.1" value="5.0" placeholder="e.g. 13.5"></div>` +
		`<button class="btn-add" onclick="addDrink('` + date + `')">Add</button>` +
		`</div>` +
		`<p class="abv-preview" id="abv-preview"></p>` +
		`</div>` +
		`<button class="btn-close" onclick="document.getElementById('day-modal').remove()">Close</button>` +
		`</div></div>`
}

// HTTP

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	tmpl, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "template not found", 500)
		return
	}
	page := string(tmpl)
	page = strings.ReplaceAll(page, "{{SUMMARY}}", renderSummary())
	page = strings.ReplaceAll(page, "{{DAYS_TILES}}", renderDaysRow(0))
	page = strings.ReplaceAll(page, "{{WEEK_TILES}}", renderWeekRow(0))
	page = strings.ReplaceAll(page, "{{MONTH_GRID}}", renderMonthGrid(0))
	page = strings.ReplaceAll(page, "{{WEEK_LABEL}}", weekLabel(0))
	page = strings.ReplaceAll(page, "{{MONTH_LABEL}}", monthLabel(0))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, page)
}


func handleSummary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, renderSummary())
}

func handleTilesDays(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, renderDaysRow(offset))
}

func handleTilesWeek(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, renderWeekRow(offset))
}

func handleTilesMonth(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, renderMonthGrid(offset))
}

func handleLabelWeek(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	fmt.Fprint(w, weekLabel(offset))
}

func handleLabelMonth(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	fmt.Fprint(w, monthLabel(offset))
}

func handleModal(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		http.Error(w, "missing date", 400)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, renderModal(date))
}

// parseKey splits a key like "330@5.00" into (330, 5.0).
func parseKey(key string) (int, float64, bool) {
	parts := strings.SplitN(key, "@", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	ml, err1 := strconv.Atoi(parts[0])
	abv, err2 := strconv.ParseFloat(parts[1], 64)
	return ml, abv, err1 == nil && err2 == nil
}

func handleAddDrink(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	date := r.FormValue("date")
	ml, err1 := strconv.Atoi(r.FormValue("volume_ml"))
	abv, err2 := strconv.ParseFloat(r.FormValue("abv"), 64)
	if err1 != nil || err2 != nil || ml <= 0 || abv <= 0 {
		http.Error(w, "invalid volume or abv", 400)
		return
	}

	day := ensureDayLog(date)
	// Match on volume+abv — increment count if already present
	for i := range day.Drinks {
		if day.Drinks[i].VolumeMl == ml && day.Drinks[i].ABV == abv {
			day.Drinks[i].Count++
			saveDB()
			w.WriteHeader(200)
			return
		}
	}
	day.Drinks = append(day.Drinks, LoggedDrink{VolumeMl: ml, ABV: abv, Count: 1})
	saveDB()
	w.WriteHeader(200)
}

func handleRemoveDrink(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	date := r.FormValue("date")
	ml, abv, ok := parseKey(r.FormValue("key"))
	if !ok {
		http.Error(w, "invalid key", 400)
		return
	}

	day := findDayLog(date)
	if day == nil {
		w.WriteHeader(200)
		return
	}
	kept := day.Drinks[:0]
	for _, d := range day.Drinks {
		if d.VolumeMl != ml || d.ABV != abv {
			kept = append(kept, d)
		}
	}
	day.Drinks = kept
	saveDB()
	w.WriteHeader(200)
}

func handleAdjustDrink(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	date := r.FormValue("date")
	ml, abv, ok := parseKey(r.FormValue("key"))
	delta, _ := strconv.Atoi(r.FormValue("delta"))
	if !ok {
		http.Error(w, "invalid key", 400)
		return
	}

	day := findDayLog(date)
	if day == nil {
		w.WriteHeader(200)
		return
	}
	var kept []LoggedDrink
	for _, d := range day.Drinks {
		if d.VolumeMl == ml && d.ABV == abv {
			d.Count += delta
			if d.Count > 0 {
				kept = append(kept, d)
			}
		} else {
			kept = append(kept, d)
		}
	}
	day.Drinks = kept
	saveDB()
	w.WriteHeader(200)
}

// Entry point

func main() {
	filePath := flag.String("f", "units.json", "Path to the JSON database file")
	port     := flag.Int("p", 8080, "Port to listen on")
	flag.Parse()

	dbPath = *filePath
	if err := loadDB(); err != nil {
		log.Fatalf("Failed to load database: %v", err)
	}
	sortDayLogs()

	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(staticFiles)))
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/summary",       handleSummary)
	mux.HandleFunc("/tiles/days",   handleTilesDays)
	mux.HandleFunc("/tiles/week",   handleTilesWeek)
	mux.HandleFunc("/tiles/month",  handleTilesMonth)
	mux.HandleFunc("/label/week",   handleLabelWeek)
	mux.HandleFunc("/label/month",  handleLabelMonth)
	mux.HandleFunc("/modal",        handleModal)
	mux.HandleFunc("/drink/add",    handleAddDrink)
	mux.HandleFunc("/drink/remove", handleRemoveDrink)
	mux.HandleFunc("/drink/adjust", handleAdjustDrink)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Unit Tracker running at http://localhost%s", addr)
	log.Printf("Database: %s", dbPath)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
