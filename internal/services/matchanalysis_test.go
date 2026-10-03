package services

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ramonskie/oxicleanarr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMatchAnalysis pins the four live production fixtures plus the fallback,
// refusal, and inconclusive paths. The four live fixtures (Fullmetal Alchemist,
// Vanished, Long Distance, Fantastic Four) must each adjudicate to
// jellyfin_wrong with evidence naming the Jellyfin outlier identity; the
// synthetic cases cover the arr_wrong and ambiguous outcomes.
func TestMatchAnalysis(t *testing.T) {
	tests := []struct {
		name                 string
		in                   MatchAnalysisInput
		wantVerdict          MatchVerdict
		wantMinConfidence    float64
		wantMaxConfidence    float64
		wantEvidenceContains []string
		wantEvidenceAbsent   []string
	}{
		{
			name: "tv sonarr-97 Fullmetal Alchemist: agreeing episodes make Jellyfin Brotherhood the outlier",
			in: MatchAnalysisInput{
				MediaType:        models.MediaTypeTVShow,
				Arr:              Identity{Title: "Fullmetal Alchemist", Year: 2003, ProviderID: "tvdb:75579"},
				Jellyfin:         Identity{Title: "Fullmetal Alchemist: Brotherhood", Year: 2009, ProviderID: "tvdb:85249"},
				FilePath:         "/tv/Fullmetal Alchemist (2003)/Season 01/Fullmetal Alchemist S01E01.mkv",
				ArrEpisodes:      alchemistArrEpisodes(),
				JellyfinEpisodes: alchemistJellyfinEpisodes(),
			},
			wantVerdict:          VerdictJellyfinWrong,
			wantMinConfidence:    0.90,
			wantMaxConfidence:    1.0,
			wantEvidenceContains: []string{"episode titles agree", "Brotherhood", "path conflict"},
		},
		{
			name: "tv sonarr-179 Vanished: disagreeing episodes fall back to the 2026 filename",
			in: MatchAnalysisInput{
				MediaType:        models.MediaTypeTVShow,
				Arr:              Identity{Title: "Vanished", Year: 2026, ProviderID: "tvdb:461839"},
				Jellyfin:         Identity{Title: "Vanished", Year: 2006, ProviderID: "tvdb:79332"},
				FilePath:         "/tv/Vanished (2026)/Season 01/Vanished.2026.S01E01.1080p.WEB-DL.mkv",
				ArrEpisodes:      vanishedArrEpisodes(),
				JellyfinEpisodes: vanishedJellyfinEpisodes(),
			},
			wantVerdict:          VerdictJellyfinWrong,
			wantMinConfidence:    0.70,
			wantMaxConfidence:    1.0,
			wantEvidenceContains: []string{"falling back to filename", "2026"},
		},
		{
			name: "tv sonarr-179 Vanished: year found only in the directory still exposes the Jellyfin outlier",
			in: MatchAnalysisInput{
				MediaType: models.MediaTypeTVShow,
				Arr:       Identity{Title: "Vanished", Year: 2026, ProviderID: "tvdb:461839"},
				Jellyfin:  Identity{Title: "Vanished", Year: 2006, ProviderID: "tvdb:79332"},
				// The year lives only in the folder name; the basename has none,
				// so basename-only parsing would leave the year unknown to both sides.
				FilePath: "/tv/Vanished (2026)/Season 01/Vanished - S01E01 - Rosefinch.mkv",
				ArrEpisodes: []Episode{
					{Season: 1, Episode: 1, Title: "Rosefinch"},
				},
				JellyfinEpisodes: []Episode{
					{Season: 1, Episode: 1, Title: "Pilot"},
				},
			},
			wantVerdict:          VerdictJellyfinWrong,
			wantMinConfidence:    0.70,
			wantMaxConfidence:    1.0,
			wantEvidenceContains: []string{"2026", "outlier"},
		},
		{
			// Live sonarr-179: the media path is a bare, year-less series folder,
			// so the year can only come from the episode basename that
			// includeEpisodeFile=true now supplies. Episode titles disagree, so
			// the fallback filename evidence decides: 2026 file vs 2006 Jellyfin.
			name: "tv sonarr-179 Vanished: year in the basename under a year-less folder",
			in: MatchAnalysisInput{
				MediaType: models.MediaTypeTVShow,
				Arr:       Identity{Title: "Vanished", Year: 2026, ProviderID: "tvdb:461839"},
				Jellyfin:  Identity{Title: "Vanished", Year: 2006, ProviderID: "tvdb:79332"},
				FilePath: "/data/media/tv/Vanished/" +
					"Vanished.2026.S01E01.1080p.WEB.h264-ETHEL.mkv",
				ArrEpisodes: []Episode{
					{Season: 1, Episode: 1, Title: "Rosefinch"},
					{Season: 1, Episode: 2, Title: "Limerence"},
					{Season: 1, Episode: 3, Title: "Hollow"},
					{Season: 1, Episode: 4, Title: "Orbit"},
				},
				JellyfinEpisodes: []Episode{
					{Season: 1, Episode: 1, Title: "Pilot"},
					{Season: 1, Episode: 2, Title: "The Truth"},
					{Season: 1, Episode: 3, Title: "Betrayal"},
					{Season: 1, Episode: 4, Title: "Reunion"},
				},
			},
			wantVerdict:          VerdictJellyfinWrong,
			wantMinConfidence:    0.70,
			wantMaxConfidence:    1.0,
			wantEvidenceContains: []string{"falling back to filename", "2026", "outlier"},
		},
		{
			name: "movie radarr-306 Long Distance: filename beats Jellyfin Distant",
			in: MatchAnalysisInput{
				MediaType: models.MediaTypeMovie,
				Arr:       Identity{Title: "Long Distance", Year: 2024, ProviderID: "tmdb:605722"},
				Jellyfin:  Identity{Title: "Distant", Year: 2024, ProviderID: "tmdb:1395720"},
				FilePath:  "/movies/Long.Distance.2024.1080p.WEB-DL.x264.mkv",
			},
			wantVerdict:          VerdictJellyfinWrong,
			wantMinConfidence:    0.90,
			wantMaxConfidence:    1.0,
			wantEvidenceContains: []string{"Long.Distance.2024", "Distant", "outlier"},
		},
		{
			// Regression: the folder name is the WRONG identity ("Distant") while
			// the basename is the arr title ("Long Distance"). If title tokens are
			// taken from the whole path the folder matches Jellyfin perfectly and
			// both sides tie at 1.00, yielding ambiguous and refusing the fix.
			name: "movie radarr-306 Long Distance: wrong folder name must not beat the basename title",
			in: MatchAnalysisInput{
				MediaType: models.MediaTypeMovie,
				Arr:       Identity{Title: "Long Distance", Year: 2024, ProviderID: "tmdb:605722"},
				Jellyfin:  Identity{Title: "Distant", Year: 2024, ProviderID: "tmdb:1395720"},
				FilePath: "/data/media/movies/Distant (2024)/" +
					"Long.Distance.2024.2160p.HULU.WEB-DL.DDP.5.1.H.265-PiRaTeS.mkv",
			},
			wantVerdict:          VerdictJellyfinWrong,
			wantMinConfidence:    0.80,
			wantMaxConfidence:    1.0,
			wantEvidenceContains: []string{"Long.Distance.2024", "Distant", "outlier"},
		},
		{
			name: "movie radarr-311 Fantastic Four: filename beats Jellyfin World Premiere",
			in: MatchAnalysisInput{
				MediaType: models.MediaTypeMovie,
				Arr:       Identity{Title: "The Fantastic 4: First Steps", Year: 2025, ProviderID: "tmdb:617126"},
				Jellyfin:  Identity{Title: "First Steps - World Premiere", Year: 2025, ProviderID: "tmdb:1516738"},
				FilePath:  "/movies/The.Fantastic.Four.First.Steps.2025.2160p.WEB-DL.mkv",
			},
			wantVerdict:          VerdictJellyfinWrong,
			wantMinConfidence:    0.80,
			wantMaxConfidence:    1.0,
			wantEvidenceContains: []string{"Fantastic", "World Premiere", "outlier"},
		},
		{
			name: "movie: filename matching Jellyfin yields arr_wrong",
			in: MatchAnalysisInput{
				MediaType: models.MediaTypeMovie,
				Arr:       Identity{Title: "Distant", Year: 2024, ProviderID: "tmdb:111"},
				Jellyfin:  Identity{Title: "Long Distance", Year: 2024, ProviderID: "tmdb:222"},
				FilePath:  "/movies/Long.Distance.2024.1080p.mkv",
			},
			wantVerdict:          VerdictArrWrong,
			wantMinConfidence:    0.80,
			wantMaxConfidence:    1.0,
			wantEvidenceContains: []string{"Jellyfin identity", "arr", "outlier"},
		},
		{
			name: "movie: filename matching neither identity is ambiguous",
			in: MatchAnalysisInput{
				MediaType: models.MediaTypeMovie,
				Arr:       Identity{Title: "Alpha", Year: 2024, ProviderID: "tmdb:1"},
				Jellyfin:  Identity{Title: "Beta", Year: 2024, ProviderID: "tmdb:2"},
				FilePath:  "/movies/Gamma.2024.1080p.mkv",
			},
			wantVerdict:          VerdictAmbiguous,
			wantMinConfidence:    0.0,
			wantMaxConfidence:    0.5,
			wantEvidenceContains: []string{"too close"},
		},
		{
			name: "movie: empty path is ambiguous",
			in: MatchAnalysisInput{
				MediaType: models.MediaTypeMovie,
				Arr:       Identity{Title: "Alpha", Year: 2024, ProviderID: "tmdb:1"},
				Jellyfin:  Identity{Title: "Beta", Year: 2024, ProviderID: "tmdb:2"},
			},
			wantVerdict:          VerdictAmbiguous,
			wantMinConfidence:    0.0,
			wantMaxConfidence:    0.1,
			wantEvidenceContains: []string{"no filename evidence"},
		},
		{
			// Live radarr-306 numbers: the file is 87m, Radarr's "Long Distance"
			// is 87m and Jellyfin's "Distant" is 5m. The filename tokens tie (the
			// year-only path favours neither title), so runtime is the decisive
			// last-resort signal pinning the mismatch on Jellyfin.
			name: "movie radarr-306: tied filename resolved by runtime matching the arr identity",
			in: MatchAnalysisInput{
				MediaType:              models.MediaTypeMovie,
				Arr:                    Identity{Title: "Long Distance", Year: 2024, ProviderID: "tmdb:605722"},
				Jellyfin:               Identity{Title: "Distant", Year: 2024, ProviderID: "tmdb:1395720"},
				FilePath:               "/movies/2024/Unknown.mkv",
				FileRuntimeMinutes:     87,
				ArrRuntimeMinutes:      87,
				JellyfinRuntimeMinutes: 5,
			},
			wantVerdict:          VerdictJellyfinWrong,
			wantMinConfidence:    0.75,
			wantMaxConfidence:    0.90,
			wantEvidenceContains: []string{"runtime", "87m", "82m", "arr identity matches"},
		},
		{
			name: "movie: runtime also tied keeps the verdict ambiguous",
			in: MatchAnalysisInput{
				MediaType:              models.MediaTypeMovie,
				Arr:                    Identity{Title: "Long Distance", Year: 2024, ProviderID: "tmdb:605722"},
				Jellyfin:               Identity{Title: "Distant", Year: 2024, ProviderID: "tmdb:1395720"},
				FilePath:               "/movies/2024/Unknown.mkv",
				FileRuntimeMinutes:     87,
				ArrRuntimeMinutes:      87,
				JellyfinRuntimeMinutes: 87,
			},
			wantVerdict:          VerdictAmbiguous,
			wantMinConfidence:    0.0,
			wantMaxConfidence:    0.5,
			wantEvidenceContains: []string{"too close"},
		},
		{
			name: "movie: missing runtime keeps the verdict ambiguous",
			in: MatchAnalysisInput{
				MediaType:         models.MediaTypeMovie,
				Arr:               Identity{Title: "Long Distance", Year: 2024, ProviderID: "tmdb:605722"},
				Jellyfin:          Identity{Title: "Distant", Year: 2024, ProviderID: "tmdb:1395720"},
				FilePath:          "/movies/2024/Unknown.mkv",
				ArrRuntimeMinutes: 87,
			},
			wantVerdict:          VerdictAmbiguous,
			wantMinConfidence:    0.0,
			wantMaxConfidence:    0.5,
			wantEvidenceContains: []string{"too close"},
		},
		{
			// Media-type guard: a TV show must never use the movie runtime
			// tie-breaker, even when all three runtimes are populated. The file
			// runtime (100m) is well within maxRuntimeDriftMinutes of the arr
			// identity (100m) and the relative gap (95m) is decisive, so without
			// the media-type guard this would emit "runtime:" evidence and return
			// jellyfin_wrong. Fewer than minMatchedEpisodes shared pairs make the
			// episode rule inconclusive, so the filename fallback runs and its tie
			// reaches the runtime branch the guard must reject.
			name: "tv show: populated runtimes must not emit runtime evidence",
			in: MatchAnalysisInput{
				MediaType:              models.MediaTypeTVShow,
				Arr:                    Identity{Title: "Alpha", Year: 2024, ProviderID: "tvdb:1"},
				Jellyfin:               Identity{Title: "Beta", Year: 2024, ProviderID: "tvdb:2"},
				FilePath:               "/tv/Unknown/2024.mkv",
				ArrRuntimeMinutes:      100,
				JellyfinRuntimeMinutes: 5,
				FileRuntimeMinutes:     100,
				ArrEpisodes:            []Episode{{Season: 1, Episode: 1, Title: "Rosefinch"}},
				JellyfinEpisodes:       []Episode{{Season: 1, Episode: 1, Title: "Pilot"}},
			},
			wantVerdict:          VerdictAmbiguous,
			wantMinConfidence:    0.0,
			wantMaxConfidence:    0.5,
			wantEvidenceContains: []string{"too close"},
			wantEvidenceAbsent:   []string{"runtime:"},
		},
		{
			// Empty FilePath but populated runtimes: the "no filename evidence"
			// branch must still fall through to the movie runtime tie-breaker.
			name: "movie: empty path with runtimes uses the runtime branch",
			in: MatchAnalysisInput{
				MediaType:              models.MediaTypeMovie,
				Arr:                    Identity{Title: "Long Distance", Year: 2024, ProviderID: "tmdb:605722"},
				Jellyfin:               Identity{Title: "Distant", Year: 2024, ProviderID: "tmdb:1395720"},
				FileRuntimeMinutes:     87,
				ArrRuntimeMinutes:      87,
				JellyfinRuntimeMinutes: 5,
			},
			wantVerdict:          VerdictJellyfinWrong,
			wantMinConfidence:    0.75,
			wantMaxConfidence:    0.90,
			wantEvidenceContains: []string{"no filename evidence", "runtime", "arr identity matches"},
		},
		{
			// Boundary: a delta separation of exactly minRuntimeDeltaMinutes (10)
			// is decisive; one minute less must stay ambiguous.
			name: "movie: runtime gap exactly at the threshold is decisive",
			in: MatchAnalysisInput{
				MediaType:              models.MediaTypeMovie,
				Arr:                    Identity{Title: "Long Distance", Year: 2024, ProviderID: "tmdb:605722"},
				Jellyfin:               Identity{Title: "Distant", Year: 2024, ProviderID: "tmdb:1395720"},
				FilePath:               "/movies/2024/Unknown.mkv",
				FileRuntimeMinutes:     87,
				ArrRuntimeMinutes:      87,
				JellyfinRuntimeMinutes: 77,
			},
			wantVerdict:          VerdictJellyfinWrong,
			wantMinConfidence:    0.75,
			wantMaxConfidence:    0.76,
			wantEvidenceContains: []string{"runtime", "arr identity matches"},
		},
		{
			name: "movie: runtime gap one below the threshold stays ambiguous",
			in: MatchAnalysisInput{
				MediaType:              models.MediaTypeMovie,
				Arr:                    Identity{Title: "Long Distance", Year: 2024, ProviderID: "tmdb:605722"},
				Jellyfin:               Identity{Title: "Distant", Year: 2024, ProviderID: "tmdb:1395720"},
				FilePath:               "/movies/2024/Unknown.mkv",
				FileRuntimeMinutes:     87,
				ArrRuntimeMinutes:      87,
				JellyfinRuntimeMinutes: 78,
			},
			wantVerdict:          VerdictAmbiguous,
			wantMinConfidence:    0.0,
			wantMaxConfidence:    0.5,
			wantEvidenceContains: []string{"too close"},
			wantEvidenceAbsent:   []string{"runtime:"},
		},
		{
			// Absolute-drift guard: the file (200m) matches neither identity
			// (arr 87m, Jellyfin 5m). The relative gap is wide (82m >= 10m) and
			// would falsely pick arr under relative-gap-only logic, but the file
			// must be within maxRuntimeDriftMinutes of its winner, so the verdict
			// stays ambiguous.
			name: "movie: file far from both identities stays ambiguous",
			in: MatchAnalysisInput{
				MediaType:              models.MediaTypeMovie,
				Arr:                    Identity{Title: "Long Distance", Year: 2024, ProviderID: "tmdb:605722"},
				Jellyfin:               Identity{Title: "Distant", Year: 2024, ProviderID: "tmdb:1395720"},
				FilePath:               "/movies/2024/Unknown.mkv",
				FileRuntimeMinutes:     200,
				ArrRuntimeMinutes:      87,
				JellyfinRuntimeMinutes: 5,
			},
			wantVerdict:          VerdictAmbiguous,
			wantMinConfidence:    0.0,
			wantMaxConfidence:    0.5,
			wantEvidenceContains: []string{"too close"},
			wantEvidenceAbsent:   []string{"runtime:"},
		},
		{
			// Guardrail: a decisive title/year verdict is never overridden, even
			// when the runtime evidence would point the other way (Jellyfin 87m
			// matches the file while the arr identity claims 240m).
			name: "movie: decisive filename verdict is not overridden by runtime",
			in: MatchAnalysisInput{
				MediaType:              models.MediaTypeMovie,
				Arr:                    Identity{Title: "Long Distance", Year: 2024, ProviderID: "tmdb:605722"},
				Jellyfin:               Identity{Title: "Distant", Year: 2024, ProviderID: "tmdb:1395720"},
				FilePath:               "/movies/Long.Distance.2024.1080p.WEB-DL.x264.mkv",
				FileRuntimeMinutes:     87,
				ArrRuntimeMinutes:      240,
				JellyfinRuntimeMinutes: 87,
			},
			wantVerdict:          VerdictJellyfinWrong,
			wantMinConfidence:    0.90,
			wantMaxConfidence:    1.0,
			wantEvidenceContains: []string{"path matches the arr identity"},
		},
		{
			name: "tv: too few shared episodes fall back to path evidence",
			in: MatchAnalysisInput{
				MediaType: models.MediaTypeTVShow,
				Arr:       Identity{Title: "Vanished", Year: 2026, ProviderID: "tvdb:461839"},
				Jellyfin:  Identity{Title: "Vanished", Year: 2006, ProviderID: "tvdb:79332"},
				FilePath:  "/tv/Vanished (2026)/Season 01/Vanished.2026.S01E01.1080p.WEB-DL.mkv",
				ArrEpisodes: []Episode{
					{Season: 1, Episode: 1, Title: "Rosefinch"},
				},
				JellyfinEpisodes: []Episode{
					{Season: 1, Episode: 1, Title: "Pilot"},
				},
			},
			wantVerdict:          VerdictJellyfinWrong,
			wantMinConfidence:    0.70,
			wantMaxConfidence:    1.0,
			wantEvidenceContains: []string{"path", "2026"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got := AnalyzeMatch(tt.in)

			// Assert
			require.NotEmpty(t, got.Evidence)
			assert.Equal(t, tt.wantVerdict, got.Verdict, "evidence: %v", got.Evidence)
			assert.GreaterOrEqual(t, got.Confidence, tt.wantMinConfidence, "evidence: %v", got.Evidence)
			assert.LessOrEqual(t, got.Confidence, tt.wantMaxConfidence, "evidence: %v", got.Evidence)

			joined := strings.Join(got.Evidence, " | ")
			for _, want := range tt.wantEvidenceContains {
				assert.Contains(t, joined, want)
			}
			for _, absent := range tt.wantEvidenceAbsent {
				assert.NotContains(t, joined, absent)
			}
		})
	}
}

// TestExtractFileText pins the basename/full-path split and proves the two live
// inputs now score decisively for the arr side (arr beats Jellyfin by at least
// scoreMargin), which is what lets the adjudicator return jellyfin_wrong instead
// of a tie-driven ambiguous.
func TestExtractFileText(t *testing.T) {
	t.Run("title tokens come from the basename while years come from the full path", func(t *testing.T) {
		path := "/data/media/movies/Distant (2024)/" +
			"Long.Distance.2024.2160p.HULU.WEB-DL.DDP.5.1.H.265-PiRaTeS.mkv"
		file := extractFileText(path)

		// The wrong folder identity must not leak into the title tokens.
		assert.NotContains(t, file.tokens, "distant")
		assert.Contains(t, file.tokens, "long")
		assert.Contains(t, file.tokens, "distance")
		// The folder year is still visible because years parse the whole path
		// (the basename repeats 2024, so extractYears yields it twice).
		assert.Contains(t, file.years, 2024)
		// Source stays the full cleaned path so evidence shows the real input.
		assert.Equal(t, path, file.source)
	})

	t.Run("a year only in the basename under a year-less folder is still seen", func(t *testing.T) {
		file := extractFileText(
			"/data/media/tv/Vanished/Vanished.2026.S01E01.1080p.WEB.h264-ETHEL.mkv")

		assert.ElementsMatch(t, []int{2026}, file.years)
		assert.Contains(t, file.tokens, "vanished")
	})

	t.Run("a bare episode-code basename falls back to the full path", func(t *testing.T) {
		file := extractFileText("/tv/Fullmetal Alchemist/S01E01.mkv")

		assert.Contains(t, file.tokens, "fullmetal")
		assert.Contains(t, file.tokens, "alchemist")
	})

	t.Run("a spaced season/episode basename falls back to the named folder", func(t *testing.T) {
		// "S01.E01.mkv" tokenizes to ["s01", "e01"]; the bare "s01" marker was
		// previously mistaken for title content, so the folder title was lost.
		file := extractFileText("/tv/Fullmetal Alchemist (2003)/S01.E01.mkv")

		assert.Contains(t, file.tokens, "fullmetal")
		assert.Contains(t, file.tokens, "alchemist")
		assert.Contains(t, file.years, 2003)
	})

	t.Run("a repeated-episode basename falls back to the named folder", func(t *testing.T) {
		file := extractFileText("/tv/Vanished (2026)/S01E01E02.mkv")

		assert.Contains(t, file.tokens, "vanished")
		assert.Contains(t, file.years, 2026)
	})

	t.Run("a bare x-prefixed episode basename falls back to the named folder", func(t *testing.T) {
		file := extractFileText("/tv/Vanished (2026)/x01.mkv")

		assert.Contains(t, file.tokens, "vanished")
		assert.Contains(t, file.years, 2026)
	})

	t.Run("backslashes fold before basename extraction", func(t *testing.T) {
		file := extractFileText(
			`C:\Media\Distant (2024)\Long.Distance.2024.2160p.mkv`)

		assert.NotContains(t, file.tokens, "distant")
		assert.Contains(t, file.tokens, "long")
		assert.Contains(t, file.tokens, "distance")
		assert.Contains(t, file.years, 2024)
	})

	t.Run("live inputs score for the arr side by at least the decision margin", func(t *testing.T) {
		movie := extractFileText(
			"/data/media/movies/Distant (2024)/" +
				"Long.Distance.2024.2160p.HULU.WEB-DL.DDP.5.1.H.265-PiRaTeS.mkv")
		arrMovie := scoreIdentity(movie, Identity{Title: "Long Distance", Year: 2024})
		jellyfinMovie := scoreIdentity(movie, Identity{Title: "Distant", Year: 2024})
		assert.GreaterOrEqual(t, arrMovie-jellyfinMovie, scoreMargin,
			"Long Distance: arr=%.2f Jellyfin=%.2f", arrMovie, jellyfinMovie)

		tv := extractFileText(
			"/data/media/tv/Vanished/Vanished.2026.S01E01.1080p.WEB.h264-ETHEL.mkv")
		arrTV := scoreIdentity(tv, Identity{Title: "Vanished", Year: 2026})
		jellyfinTV := scoreIdentity(tv, Identity{Title: "Vanished", Year: 2006})
		assert.GreaterOrEqual(t, arrTV-jellyfinTV, scoreMargin,
			"Vanished: arr=%.2f Jellyfin=%.2f", arrTV, jellyfinTV)
	})
}

// TestParseRuntimeMinutes pins the clock-parsing helper that feeds the movie
// runtime tie-breaker, including the empty/garbage inputs that must read as "no
// runtime evidence" (0) rather than zero minutes.
func TestParseRuntimeMinutes(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"1:27:00", 87},
		{"01:27:00", 87},
		{"2:03:00", 123},
		{"27:00", 27},
		{"1:27:30", 87}, // seconds truncate
		{"87", 87},
		{"", 0},
		{"garbage", 0},
		{"1:xx:00", 0},
		{"1:2:3:4", 0},
		{"-1:00", 0},
		// A component that would overflow the int math on multiply must be
		// rejected as "no runtime evidence" rather than wrapping.
		{"9223372036854775807:00:00", 0},
		{"100001:00:00", 0},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, parseRuntimeMinutes(tt.in))
		})
	}
}

// TestMatchAnalysisNormalization covers the pure normalization helpers the
// adjudicator leans on.
func TestMatchAnalysisNormalization(t *testing.T) {
	t.Run("normalize title maps number words and strips punctuation", func(t *testing.T) {
		assert.Equal(t, "the fantastic 4 first steps", NormalizeTitle("The Fantastic Four: First Steps"))
	})

	t.Run("extracts plausible production years", func(t *testing.T) {
		assert.ElementsMatch(t, []int{2026}, extractYears("Vanished.2026.S01E01.1080p.WEB-DL"))
	})

	t.Run("ignores non-year numeric tokens", func(t *testing.T) {
		assert.Empty(t, extractYears("Movie.1080p.x265.2160p"))
	})

	t.Run("episode title similarity is 1 for identical and 0 for disjoint titles", func(t *testing.T) {
		assert.InDelta(t, 1.0, titleSimilarity("Fullmetal Alchemist", "fullmetal alchemist"), 0.0001)
		assert.InDelta(t, 0.0, titleSimilarity("Rosefinch", "Pilot"), 0.0001)
	})

	t.Run("a year-only title keeps its token instead of vanishing", func(t *testing.T) {
		assert.Equal(t, []string{"1923"}, titleTokens("1923"))
		// Ordinary titles still drop the year-like token.
		assert.Equal(t, []string{"blade", "runner"}, titleTokens("Blade Runner 2049"))
	})

	t.Run("provider ids only short-circuit when the scheme matches", func(t *testing.T) {
		agree := func(arr, jellyfin Identity) bool {
			return identitiesAgree(MatchAnalysisInput{Arr: arr, Jellyfin: jellyfin})
		}
		// Same scheme, different value => decisive disagreement.
		assert.False(t, agree(
			Identity{Title: "Same", Year: 2020, ProviderID: "tvdb:1"},
			Identity{Title: "Same", Year: 2020, ProviderID: "tvdb:2"}))
		// Different schemes are not comparable, so matching title+year wins.
		assert.True(t, agree(
			Identity{Title: "Dune", Year: 2021, ProviderID: "tvdb:1"},
			Identity{Title: "Dune", Year: 2021, ProviderID: "tmdb:2"}))
	})

	t.Run("paths compare after normalization", func(t *testing.T) {
		// Same file, differing separators and case.
		assert.True(t, sameFilePath(`C:\TV\Show\S01E01.mkv`, "c:/tv/show/s01e01.mkv"))
		// Same file exposed under different library roots.
		assert.True(t, sameFilePath("/tv/Show/S01E01.mkv", "/media/library/tv/Show/S01E01.mkv"))
		// Different files are never conflated.
		assert.False(t, sameFilePath("/tv/Show/S01E01.mkv", "/tv/Show/S01E02.mkv"))
		assert.False(t, sameFilePath("", "/tv/Show/S01E01.mkv"))
	})
}

// alchemistArrEpisodes and alchemistJellyfinEpisodes model the same 2003
// Fullmetal Alchemist files as each side records them. The episode titles differ
// only in casing and punctuation and the paths differ only in library root, so
// the normalized token sets and file names still agree exactly — the fixture
// fails if agreement is computed on raw strings or raw paths instead.
func alchemistArrEpisodes() []Episode {
	episodes := make([]Episode, 0, alchemistEpisodeCount)
	for i := 1; i <= alchemistEpisodeCount; i++ {
		episodes = append(episodes, Episode{
			Season:  1,
			Episode: i,
			Title:   alchemistBaseTitle(i),
			Path: fmt.Sprintf(
				"/tv/Fullmetal Alchemist (2003)/Season 01/Fullmetal Alchemist S01E%02d.mkv", i),
		})
	}
	return episodes
}

func alchemistJellyfinEpisodes() []Episode {
	episodes := make([]Episode, 0, alchemistEpisodeCount)
	for i := 1; i <= alchemistEpisodeCount; i++ {
		episodes = append(episodes, Episode{
			Season:  1,
			Episode: i,
			Title:   jellyfinSpelling(alchemistBaseTitle(i)),
			// Same files, reached through a different library root.
			Path: fmt.Sprintf(
				"/media/tv/Fullmetal Alchemist (2003)/Season 01/Fullmetal Alchemist S01E%02d.mkv", i),
		})
	}
	return episodes
}

const alchemistEpisodeCount = 51

// alchemistBaseTitle is the canonical (Sonarr) spelling of episode i.
func alchemistBaseTitle(i int) string {
	switch i {
	case 1:
		return "Fullmetal Alchemist"
	case alchemistEpisodeCount:
		return "The Immortal Legion"
	default:
		return fmt.Sprintf("Episode %02d", i)
	}
}

// jellyfinSpelling reproduces the casing/punctuation drift Jellyfin's metadata
// introduces without changing the normalized tokens.
func jellyfinSpelling(title string) string {
	switch title {
	case "Fullmetal Alchemist":
		return "fullmetal alchemist!"
	case "The Immortal Legion":
		return "The Immortal Legion."
	default:
		return strings.ToLower(title)
	}
}

// vanishedArrEpisodes is the 2026 series as Sonarr knows it.
func vanishedArrEpisodes() []Episode {
	titles := []string{"Rosefinch", "Limerence", "Hollow", "Orbit"}
	episodes := make([]Episode, 0, len(titles))
	for i, title := range titles {
		episodes = append(episodes, Episode{
			Season:  1,
			Episode: i + 1,
			Title:   title,
			Path: fmt.Sprintf(
				"/tv/Vanished (2026)/Season 01/Vanished.2026.S01E%02d.1080p.WEB-DL.mkv", i+1),
		})
	}
	return episodes
}

// vanishedJellyfinEpisodes is the 2006 series Jellyfin mis-identified it as.
func vanishedJellyfinEpisodes() []Episode {
	titles := []string{"Pilot", "The Truth", "Betrayal", "Reunion"}
	episodes := make([]Episode, 0, len(titles))
	for i, title := range titles {
		episodes = append(episodes, Episode{
			Season:  1,
			Episode: i + 1,
			Title:   title,
			Path: fmt.Sprintf(
				"/tv/Vanished (2006)/Season 01/Vanished.2006.S01E%02d.avi", i+1),
		})
	}
	return episodes
}
