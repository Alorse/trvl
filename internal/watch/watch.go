// Package watch provides price tracking for flights and hotels.
// It stores watch definitions and price history as JSON files
// under ~/.trvl/ and supports threshold-based alerting.
package watch

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Watch represents a price tracking rule for a flight or hotel route.
//
// Three watch modes:
//   - Specific date: DepartDate set, no DepartFrom/DepartTo → checks one date
//   - Date range: DepartFrom + DepartTo set → checks cheapest across range
//   - Route watch: no dates at all → checks next 60 days for cheapest on any date
type Watch struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"` // "flight" or "hotel"
	Origin       string    `json:"origin"`
	Destination  string    `json:"destination"`
	DepartDate   string    `json:"depart_date,omitempty"`
	ReturnDate   string    `json:"return_date,omitempty"`
	DepartFrom   string    `json:"depart_from,omitempty"` // date range start (YYYY-MM-DD)
	DepartTo     string    `json:"depart_to,omitempty"`   // date range end (YYYY-MM-DD)
	BelowPrice   float64   `json:"below_price"`
	Currency     string    `json:"currency"`
	CreatedAt    time.Time `json:"created_at"`
	LastCheck    time.Time `json:"last_check"`
	LastPrice    float64   `json:"last_price"`
	LowestPrice  float64   `json:"lowest_price"`
	CheapestDate string    `json:"cheapest_date,omitempty"` // which date had the lowest price
}

// IsRouteWatch returns true if this watch monitors a route without specific dates.
func (w Watch) IsRouteWatch() bool {
	return w.DepartDate == "" && w.DepartFrom == "" && w.DepartTo == ""
}

// IsDateRange returns true if this watch monitors a date range.
func (w Watch) IsDateRange() bool {
	return w.DepartFrom != "" && w.DepartTo != ""
}

const watchDateLayout = "2006-01-02"

// Validate rejects malformed or ambiguous watch date combinations before they
// get persisted and later fail during background checks.
func (w Watch) Validate() error {
	if err := validateWatchDate("depart date", w.DepartDate); err != nil {
		return err
	}
	if err := validateWatchDate("return date", w.ReturnDate); err != nil {
		return err
	}
	if err := validateWatchDate("date range start", w.DepartFrom); err != nil {
		return err
	}
	if err := validateWatchDate("date range end", w.DepartTo); err != nil {
		return err
	}

	if w.DepartDate != "" && (w.DepartFrom != "" || w.DepartTo != "") {
		return fmt.Errorf("cannot combine a specific depart date with a date range")
	}
	if (w.DepartFrom == "") != (w.DepartTo == "") {
		return fmt.Errorf("date range requires both start and end dates")
	}
	if w.IsDateRange() {
		from, _ := time.Parse(watchDateLayout, w.DepartFrom)
		to, _ := time.Parse(watchDateLayout, w.DepartTo)
		if from.After(to) {
			return fmt.Errorf("date range start must be on or before end")
		}
	}
	return nil
}

func validateWatchDate(label, value string) error {
	if value == "" {
		return nil
	}
	if _, err := time.Parse(watchDateLayout, value); err != nil {
		return fmt.Errorf("%s must use YYYY-MM-DD", label)
	}
	return nil
}

// PricePoint records a single price observation for a watch.
type PricePoint struct {
	WatchID   string    `json:"watch_id"`
	Price     float64   `json:"price"`
	Currency  string    `json:"currency"`
	Timestamp time.Time `json:"timestamp"`
}

// Store manages persistence of watches and price history to disk.
// All methods are safe for concurrent use.
type Store struct {
	mu      sync.Mutex
	dir     string
	watches []Watch
	history []PricePoint
}

// NewStore creates a store rooted at the given directory (typically ~/.trvl/).
// The directory is created on first write if it does not exist.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// DefaultStore returns a store at ~/.trvl/.
func DefaultStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	return NewStore(filepath.Join(home, ".trvl")), nil
}

func (s *Store) watchesPath() string {
	return filepath.Join(s.dir, "watches.json")
}

func (s *Store) historyPath() string {
	return filepath.Join(s.dir, "price-history.json")
}

func (s *Store) ensureDir() error {
	return os.MkdirAll(s.dir, 0o755)
}

// Load reads watches and history from disk. If the files do not exist,
// the store starts empty (not an error).
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.watches = nil
	s.history = nil

	if err := loadJSON(s.watchesPath(), &s.watches); err != nil {
		return fmt.Errorf("load watches: %w", err)
	}
	if err := loadJSON(s.historyPath(), &s.history); err != nil {
		return fmt.Errorf("load history: %w", err)
	}
	return nil
}

// Save writes watches and history to disk atomically.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	if err := s.ensureDir(); err != nil {
		return fmt.Errorf("create storage dir: %w", err)
	}
	if err := saveJSON(s.watchesPath(), s.watches); err != nil {
		return fmt.Errorf("save watches: %w", err)
	}
	if err := saveJSON(s.historyPath(), s.history); err != nil {
		return fmt.Errorf("save history: %w", err)
	}
	return nil
}

// Add inserts a new watch and persists to disk. Returns the assigned ID.
func (s *Store) Add(w Watch) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := w.Validate(); err != nil {
		return "", err
	}

	w.ID = shortID()
	w.CreatedAt = time.Now()
	s.watches = append(s.watches, w)

	if err := s.saveLocked(); err != nil {
		return "", err
	}
	return w.ID, nil
}

// List returns all active watches.
func (s *Store) List() []Watch {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Watch, len(s.watches))
	copy(out, s.watches)
	return out
}

// Get returns a single watch by ID, or false if not found.
func (s *Store) Get(id string) (Watch, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, w := range s.watches {
		if w.ID == id {
			return w, true
		}
	}
	return Watch{}, false
}

// Remove deletes a watch by ID. Returns true if found and removed.
func (s *Store) Remove(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, w := range s.watches {
		if w.ID == id {
			s.watches = append(s.watches[:i], s.watches[i+1:]...)
			if err := s.saveLocked(); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

// UpdateWatch replaces a watch in-place by ID and persists.
func (s *Store) UpdateWatch(updated Watch) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, w := range s.watches {
		if w.ID == updated.ID {
			s.watches[i] = updated
			return s.saveLocked()
		}
	}
	return fmt.Errorf("watch %s not found", updated.ID)
}

// RecordPrice appends a price point to history and persists.
func (s *Store) RecordPrice(watchID string, price float64, currency string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.history = append(s.history, PricePoint{
		WatchID:   watchID,
		Price:     price,
		Currency:  currency,
		Timestamp: time.Now(),
	})
	return s.saveLocked()
}

// History returns all price points for a given watch ID, ordered by time.
func (s *Store) History(watchID string) []PricePoint {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []PricePoint
	for _, p := range s.history {
		if p.WatchID == watchID {
			out = append(out, p)
		}
	}
	return out
}

// shortID generates a 4-byte hex string (8 characters).
func shortID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// Fallback: use timestamp-based ID
		return fmt.Sprintf("%08x", time.Now().UnixNano()&0xFFFFFFFF)
	}
	return hex.EncodeToString(b)
}

// loadJSON reads a JSON file into dst. Returns nil if file does not exist.
func loadJSON(path string, dst interface{}) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, dst)
}

// saveJSON writes data as pretty-printed JSON.
func saveJSON(path string, data interface{}) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
