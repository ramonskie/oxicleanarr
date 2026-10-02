package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/config"
	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/ramonskie/oxicleanarr/internal/services"
	"github.com/ramonskie/oxicleanarr/internal/services/analytics"
)

// AnalyticsHandler serves stale-content and ROI analytics derived from the
// synced media library. It is provider-agnostic: the underlying watch data
// comes from whichever stats provider is enabled.
type AnalyticsHandler struct {
	syncEngine *services.SyncEngine
}

// NewAnalyticsHandler creates a new AnalyticsHandler.
func NewAnalyticsHandler(syncEngine *services.SyncEngine) *AnalyticsHandler {
	return &AnalyticsHandler{syncEngine: syncEngine}
}

// staleItem is a single stale-content row.
type staleItem struct {
	ID                string     `json:"id"`
	Title             string     `json:"title"`
	Type              string     `json:"type"`
	Year              int        `json:"year,omitempty"`
	FileSize          int64      `json:"file_size"`
	AddedAt           time.Time  `json:"added_at"`
	LastWatched       *time.Time `json:"last_watched"`
	WatchCount        int        `json:"watch_count"`
	Category          string     `json:"category"`
	DaysStale         int        `json:"days_stale"`
	Excluded          bool       `json:"excluded"`
	ManualLeavingSoon bool       `json:"manual_leaving_soon"`
}

// staleCount is a count + size pair used in the stale summary.
type staleCount struct {
	Count     int   `json:"count"`
	SizeBytes int64 `json:"size_bytes"`
}

// staleSummary aggregates stale content by category.
type staleSummary struct {
	NeverWatched  staleCount `json:"never_watched"`
	Stale         staleCount `json:"stale"`
	Total         staleCount `json:"total"`
	ThresholdDays int        `json:"threshold_days"`
}

// staleResponse is the GET /api/analytics/stale payload.
type staleResponse struct {
	Enabled bool         `json:"enabled"`
	Items   []staleItem  `json:"items"`
	Summary staleSummary `json:"summary"`
}

// roiItem is a single ROI row.
type roiItem struct {
	ID                 string     `json:"id"`
	Title              string     `json:"title"`
	Type               string     `json:"type"`
	Year               int        `json:"year,omitempty"`
	FileSizeBytes      int64      `json:"file_size_bytes"`
	FileSizeGB         float64    `json:"file_size_gb"`
	WatchCount         int        `json:"watch_count"`
	GatedPlayCount     int        `json:"gated_play_count"`
	TotalWatchHours    float64    `json:"total_watch_hours"`
	LastWatched        *time.Time `json:"last_watched"`
	DaysSinceLastWatch int        `json:"days_since_last_watch"`
	WatchHoursPerGB    float64    `json:"watch_hours_per_gb"`
	ValueScore         float64    `json:"value_score"`
	ValueCategory      string     `json:"value_category"`
	SuggestDeletion    bool       `json:"suggest_deletion"`
	Excluded           bool       `json:"excluded"`
	ManualLeavingSoon  bool       `json:"manual_leaving_soon"`
}

// deadWeightItem is a single all-time never-watched row.
type deadWeightItem struct {
	ID                string    `json:"id"`
	Title             string    `json:"title"`
	Type              string    `json:"type"`
	Year              int       `json:"year,omitempty"`
	FileSize          int64     `json:"file_size"`
	AddedAt           time.Time `json:"added_at"`
	Excluded          bool      `json:"excluded"`
	ManualLeavingSoon bool      `json:"manual_leaving_soon"`
}

// deadWeightSummary aggregates all-time never-watched content.
type deadWeightSummary struct {
	Count          int   `json:"count"`
	TotalSizeBytes int64 `json:"total_size_bytes"`
}

// deadWeightResponse is the GET /api/analytics/dead-weight payload. Items are
// capped (like Tracearr's dead-weight module); the summary covers everything.
type deadWeightResponse struct {
	Enabled      bool              `json:"enabled"`
	HasWatchData bool              `json:"has_watch_data"`
	Items        []deadWeightItem  `json:"items"`
	Summary      deadWeightSummary `json:"summary"`
}

// deadWeightItemLimit caps the listed rows; the summary is uncapped.
const deadWeightItemLimit = 10

// roiSummary aggregates ROI across the library.
type roiSummary struct {
	TotalItems         int     `json:"total_items"`
	TotalStorageGB     float64 `json:"total_storage_gb"`
	TotalWatchHours    float64 `json:"total_watch_hours"`
	AvgWatchHoursPerGB float64 `json:"avg_watch_hours_per_gb"`
	LowValueItems      int     `json:"low_value_items"`
	LowValueStorageGB  float64 `json:"low_value_storage_gb"`
	PotentialSavingsGB float64 `json:"potential_savings_gb"`
}

// roiResponse is the GET /api/analytics/roi payload.
type roiResponse struct {
	Enabled bool       `json:"enabled"`
	Items   []roiItem  `json:"items"`
	Summary roiSummary `json:"summary"`
	// HasWatchData is false when no watch-history provider is configured, in
	// which case every item would look low-value and ROI is not meaningful.
	HasWatchData bool                         `json:"has_watch_data"`
	Thresholds   config.ValueThresholdsConfig `json:"thresholds"`
}

// GetStale handles GET /api/analytics/stale.
// Optional query param: category = all|never_watched|stale.
func (h *AnalyticsHandler) GetStale(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()
	if cfg == nil || h.syncEngine == nil {
		writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "Configuration not available"})
		return
	}

	categoryFilter := r.URL.Query().Get("category")
	threshold := cfg.Analytics.StaleDays
	enabled := cfg.Analytics.Enabled

	response := staleResponse{
		Enabled: enabled,
		Items:   []staleItem{},
		Summary: staleSummary{ThresholdDays: threshold},
	}

	if enabled {
		now := time.Now()
		for _, media := range h.syncEngine.GetMediaList() {
			result := analytics.ComputeStale(now, media.AddedAt, media.LastWatched, threshold)
			if !result.IsStale {
				continue
			}
			// Summary always covers the whole stale set; the category filter
			// only narrows the item list.
			accumulateStale(&response.Summary, result.Category, media.FileSize)
			if categoryFilter != "" && categoryFilter != "all" && string(result.Category) != categoryFilter {
				continue
			}
			response.Items = append(response.Items, newStaleItem(media, result))
		}
	}

	// Largest reclaimable content first.
	sort.Slice(response.Items, func(i, j int) bool {
		if response.Items[i].FileSize != response.Items[j].FileSize {
			return response.Items[i].FileSize > response.Items[j].FileSize
		}
		return response.Items[i].Title < response.Items[j].Title
	})

	writeJSON(w, http.StatusOK, response)
}

// GetROI handles GET /api/analytics/roi.
// Optional query param: value_category = all|low_value|moderate_value|high_value.
func (h *AnalyticsHandler) GetROI(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()
	if cfg == nil || h.syncEngine == nil {
		writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "Configuration not available"})
		return
	}

	categoryFilter := r.URL.Query().Get("value_category")
	enabled := cfg.Analytics.Enabled

	response := roiResponse{
		Enabled:      enabled,
		Items:        []roiItem{},
		Thresholds:   cfg.Analytics.ValueThresholds,
		HasWatchData: watchProviderEnabled(cfg),
	}

	if enabled && response.HasWatchData {
		now := time.Now()
		for _, media := range h.syncEngine.GetMediaList() {
			result := analytics.ROI(now, roiInputFor(media), cfg.Analytics)
			// Summary always covers the whole library; the filter only narrows
			// the item list.
			accumulateROI(&response.Summary, result)
			if categoryFilter != "" && categoryFilter != "all" && string(result.ValueCategory) != categoryFilter {
				continue
			}
			response.Items = append(response.Items, newROIItem(media, result))
		}
		if response.Summary.TotalStorageGB > 0 {
			response.Summary.AvgWatchHoursPerGB = roundTo(response.Summary.TotalWatchHours/response.Summary.TotalStorageGB, 2)
		}
	}

	// Deletion candidates first, then lowest value first.
	sort.Slice(response.Items, func(i, j int) bool {
		if response.Items[i].SuggestDeletion != response.Items[j].SuggestDeletion {
			return response.Items[i].SuggestDeletion
		}
		if response.Items[i].WatchHoursPerGB != response.Items[j].WatchHoursPerGB {
			return response.Items[i].WatchHoursPerGB < response.Items[j].WatchHoursPerGB
		}
		return response.Items[i].Title < response.Items[j].Title
	})

	writeJSON(w, http.StatusOK, response)
}

// GetDeadWeight handles GET /api/analytics/dead-weight: all-time never-watched
// titles (WatchCount == 0) and the storage they occupy. Like Tracearr, the
// summary is uncapped while the listed rows are limited to the largest few.
func (h *AnalyticsHandler) GetDeadWeight(w http.ResponseWriter, r *http.Request) {
	cfg := config.Get()
	if cfg == nil || h.syncEngine == nil {
		writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "Configuration not available"})
		return
	}

	response := deadWeightResponse{
		Enabled:      cfg.Analytics.Enabled,
		HasWatchData: watchDataAvailable(cfg),
		Items:        []deadWeightItem{},
	}

	if response.Enabled && response.HasWatchData {
		for _, media := range h.syncEngine.GetMediaList() {
			// WatchCount is only meaningful once the item is matched to a
			// Jellyfin entry; an unmatched item's zero count means "unknown",
			// not "never watched".
			if media.JellyfinID == "" || media.WatchCount > 0 {
				continue
			}
			response.Summary.Count++
			response.Summary.TotalSizeBytes += media.FileSize
			response.Items = append(response.Items, newDeadWeightItem(media))
		}

		// Largest reclaimable first; the summary already covers every row.
		sort.Slice(response.Items, func(i, j int) bool {
			if response.Items[i].FileSize != response.Items[j].FileSize {
				return response.Items[i].FileSize > response.Items[j].FileSize
			}
			return response.Items[i].Title < response.Items[j].Title
		})
		if len(response.Items) > deadWeightItemLimit {
			response.Items = response.Items[:deadWeightItemLimit]
		}
	}

	writeJSON(w, http.StatusOK, response)
}

func newStaleItem(media models.Media, result analytics.StaleResult) staleItem {
	return staleItem{
		ID:                media.ID,
		Title:             media.Title,
		Type:              string(media.Type),
		Year:              media.Year,
		FileSize:          media.FileSize,
		AddedAt:           media.AddedAt,
		LastWatched:       optionalTime(media.LastWatched),
		WatchCount:        media.WatchCount,
		Category:          string(result.Category),
		DaysStale:         result.DaysStale,
		Excluded:          media.IsExcluded,
		ManualLeavingSoon: media.IsManualLeavingSoon,
	}
}

func newDeadWeightItem(media models.Media) deadWeightItem {
	return deadWeightItem{
		ID:                media.ID,
		Title:             media.Title,
		Type:              string(media.Type),
		Year:              media.Year,
		FileSize:          media.FileSize,
		AddedAt:           media.AddedAt,
		Excluded:          media.IsExcluded,
		ManualLeavingSoon: media.IsManualLeavingSoon,
	}
}

func newROIItem(media models.Media, result analytics.ROIResult) roiItem {
	return roiItem{
		ID:                 media.ID,
		Title:              media.Title,
		Type:               string(media.Type),
		Year:               media.Year,
		FileSizeBytes:      media.FileSize,
		FileSizeGB:         roundTo(result.FileSizeGB, 2),
		WatchCount:         media.WatchCount,
		GatedPlayCount:     media.GatedPlayCount,
		TotalWatchHours:    roundTo(result.TotalWatchHours, 2),
		LastWatched:        optionalTime(media.LastWatched),
		DaysSinceLastWatch: result.DaysSinceLastWatch,
		WatchHoursPerGB:    result.WatchHoursPerGB,
		ValueScore:         result.ValueScore,
		ValueCategory:      string(result.ValueCategory),
		SuggestDeletion:    result.SuggestDeletion,
		Excluded:           media.IsExcluded,
		ManualLeavingSoon:  media.IsManualLeavingSoon,
	}
}

func accumulateStale(summary *staleSummary, category analytics.StaleCategory, size int64) {
	if category == analytics.CategoryNeverWatched {
		summary.NeverWatched.Count++
		summary.NeverWatched.SizeBytes += size
	} else {
		summary.Stale.Count++
		summary.Stale.SizeBytes += size
	}
	summary.Total.Count++
	summary.Total.SizeBytes += size
}

func accumulateROI(summary *roiSummary, result analytics.ROIResult) {
	summary.TotalItems++
	summary.TotalStorageGB += result.FileSizeGB
	summary.TotalWatchHours += result.TotalWatchHours
	if result.ValueCategory == analytics.ValueLow {
		summary.LowValueItems++
		summary.LowValueStorageGB += result.FileSizeGB
	}
	if result.SuggestDeletion {
		summary.PotentialSavingsGB += result.FileSizeGB
	}
}

// roiInputFor maps a media item to the analytics input. Movies and TV shows
// are classified as movie/show respectively (OxiCleanarr stores items at the
// show level, not per episode).
func roiInputFor(media models.Media) analytics.ROIInput {
	kind := analytics.KindShow
	if media.Type == models.MediaTypeMovie {
		kind = analytics.KindMovie
	}
	return analytics.ROIInput{
		FileSizeBytes:     media.FileSize,
		TotalWatchSeconds: media.TotalWatchSeconds,
		LastWatched:       media.LastWatched,
		AddedAt:           media.AddedAt,
		Kind:              kind,
	}
}

// watchDataAvailable reports whether any watch-data source is configured.
// Jellyfin supplies per-item PlayCount, so never-watched detection works with
// Jellyfin alone; ROI additionally needs summed playback time from a stats
// provider.
func watchDataAvailable(cfg *config.Config) bool {
	return cfg.Integrations.Jellyfin.Enabled || watchProviderEnabled(cfg)
}

// watchProviderEnabled reports whether any watch-history provider is
// configured. Without one, TotalWatchSeconds is always zero and ROI would
// classify the whole library as low-value.
func watchProviderEnabled(cfg *config.Config) bool {
	return cfg.Integrations.Tracearr.Enabled ||
		cfg.Integrations.Jellystat.Enabled ||
		cfg.Integrations.Streamystats.Enabled
}

// optionalTime returns a pointer to t, or nil when t is the zero value, so
// JSON emits null for never-watched items.
func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func roundTo(v float64, places int) float64 {
	scale := 1.0
	for i := 0; i < places; i++ {
		scale *= 10
	}
	return float64(int64(v*scale+0.5)) / scale
}
