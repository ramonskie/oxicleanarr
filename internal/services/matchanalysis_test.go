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
