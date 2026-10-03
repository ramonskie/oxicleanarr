package services

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/ramonskie/oxicleanarr/internal/models"
)

// MatchVerdict is the adjudicated outcome: which side (if any) holds the wrong
// media identity. It is a small closed set so callers can switch exhaustively.
type MatchVerdict string

const (
	// VerdictJellyfinWrong means the on-disk content matches the Sonarr/Radarr
	// identity, so Jellyfin's library metadata is the outlier and may be
	// re-identified with user confirmation.
	VerdictJellyfinWrong MatchVerdict = "jellyfin_wrong"
	// VerdictArrWrong means the on-disk content matches Jellyfin's identity, so
	// the arr (Sonarr/Radarr) entry is the outlier. Callers must NOT mutate
	// Jellyfin in this case; the fix belongs in Sonarr/Radarr.
	VerdictArrWrong MatchVerdict = "arr_wrong"
	// VerdictAmbiguous means the evidence does not clearly favour either side.
	VerdictAmbiguous MatchVerdict = "ambiguous"
)

// Verdict constants used by threshold logic. Named so the intent of each magic
// number is explicit and centrally tunable.
const (
	episodeAgreementHigh = 0.60 // >= this share of episode titles agree => content matches arr
	minMatchedEpisodes   = 3    // need this many shared (season, episode) keys to trust agreement
	scoreMargin          = 0.15 // identity score gap needed to call a winner
	titleWeight          = 0.70
	yearWeight           = 0.30
	unknownYearScore     = 0.50 // neutral year credit when either side lacks a year
)

// Identity is a caller-supplied name/year/provider tuple describing one side of
// the comparison. The adjudicator treats it as immutable input data.
type Identity struct {
	Title      string
	Year       int
	ProviderID string // e.g. "tvdb:75579" or "tmdb:605722"; informational only
}

// Episode is a normalized (season, episode) view of a single episode from either
// side. Path is optional and only used to detect a same-file path conflict.
type Episode struct {
	Season  int
	Episode int
	Title   string
	Path    string
}

// MatchAnalysisInput bundles every piece of evidence the adjudicator reasons
// over. Nothing here triggers I/O; the caller owns all fetching.
type MatchAnalysisInput struct {
	MediaType models.MediaType

	Arr      Identity
	Jellyfin Identity

	// FilePath is the on-disk path held by the arr side: the movie file, or a
	// representative episode file for TV. Used for filename title/year tokens.
	FilePath string

	// ArrEpisodes / JellyfinEpisodes are compared by (season, episode) for TV.
	ArrEpisodes      []Episode
	JellyfinEpisodes []Episode
}

// MatchAnalysis is the immutable adjudication result: a verdict, a bounded
// confidence in that verdict, and the human-readable evidence that produced it.
type MatchAnalysis struct {
	Verdict    MatchVerdict `json:"verdict"`
	Confidence float64      `json:"confidence"`
	Evidence   []string     `json:"evidence"`

	// matchedEpisodes is the number of (season, episode) pairs shared between
	// the arr and Jellyfin episode lists. It is unexported so it stays out of
	// the JSON contract consumed by the frontend; it exists only so a refused
	// Fix Match can log how much episode evidence backed the verdict.
	matchedEpisodes int
}

// AnalyzeMatch adjudicates which side holds the wrong identity from evidence the
// caller already has. It performs no network, filesystem, or client calls and
// has no side effects.
func AnalyzeMatch(in MatchAnalysisInput) MatchAnalysis {
	if identitiesAgree(in) {
		return MatchAnalysis{
			Verdict:    VerdictAmbiguous,
			Confidence: 0.50,
			Evidence: []string{fmt.Sprintf(
				"arr and Jellyfin identities already agree (%s)", describeIdentity(in.Arr))},
		}
	}

	if in.MediaType == models.MediaTypeTVShow {
		if analysis, ok := adjudicateTV(in); ok {
			return analysis
		}
	}

	return adjudicateByFileText(in)
}

// adjudicateTV compares episode titles by (season, episode). High agreement
// means the files match the arr identity, so Jellyfin is the outlier; low
// agreement is inconclusive and defers to filename evidence.
func adjudicateTV(in MatchAnalysisInput) (MatchAnalysis, bool) {
	agreement, matched, pathRatio := episodeAgreement(in.ArrEpisodes, in.JellyfinEpisodes)
	if matched < minMatchedEpisodes {
		return MatchAnalysis{}, false
	}

	if agreement >= episodeAgreementHigh {
		evidence := []string{fmt.Sprintf(
			"episode titles agree for %d shared (season, episode) pairs (%.0f%%)",
			matched, agreement*100)}
		if pathRatio >= 0.5 {
			evidence = append(evidence, "both sides resolve to the same episode files (path conflict)")
		}
		evidence = append(evidence, fmt.Sprintf(
			"Jellyfin identifies as %s while arr identifies as %s — Jellyfin metadata is the outlier",
			describeIdentity(in.Jellyfin), describeIdentity(in.Arr)))
		return MatchAnalysis{
			Verdict:    VerdictJellyfinWrong,
			Confidence: clamp01(0.5 + 0.5*agreement),
			Evidence:   evidence,
		}, true
	}

	fallback := adjudicateByFileText(in)
	fallback.Evidence = append([]string{fmt.Sprintf(
		"episode titles agree only %.0f%% across %d shared pairs; falling back to filename evidence",
		agreement*100, matched)}, fallback.Evidence...)
	return fallback, true
}

// adjudicateByFileText compares path (filename and folder-name) title tokens and
// year against each identity. It is the movie path and the TV fallback.
func adjudicateByFileText(in MatchAnalysisInput) MatchAnalysis {
	file := extractFileText(in.FilePath)
	if len(file.tokens) == 0 && len(file.years) == 0 {
		return MatchAnalysis{
			Verdict:    VerdictAmbiguous,
			Confidence: 0.10,
			Evidence:   []string{"no filename evidence available to compare identities"},
		}
	}

	arrScore := scoreIdentity(file, in.Arr)
	jellyfinScore := scoreIdentity(file, in.Jellyfin)
	margin := arrScore - jellyfinScore
	evidence := []string{fmt.Sprintf(
		"path %q scores arr %s=%.2f, Jellyfin %s=%.2f",
		file.source, describeIdentity(in.Arr), arrScore, describeIdentity(in.Jellyfin), jellyfinScore)}

	switch {
	case margin > scoreMargin:
		evidence = append(evidence, fmt.Sprintf(
			"path matches the arr identity; Jellyfin (%s) is the outlier", describeIdentity(in.Jellyfin)))
		return MatchAnalysis{Verdict: VerdictJellyfinWrong, Confidence: clamp01(0.5 + margin), Evidence: evidence}
	case margin < -scoreMargin:
		evidence = append(evidence, fmt.Sprintf(
			"path matches the Jellyfin identity; arr (%s) is the outlier", describeIdentity(in.Arr)))
		return MatchAnalysis{Verdict: VerdictArrWrong, Confidence: clamp01(0.5 - margin), Evidence: evidence}
	default:
		evidence = append(evidence, "path scores are too close to decide which side is wrong")
		return MatchAnalysis{Verdict: VerdictAmbiguous, Confidence: clamp01(0.5 - math.Abs(margin)), Evidence: evidence}
	}
}

// identitiesAgree reports whether both sides describe the same title. Provider
// IDs win only when they share a scheme (e.g. both tvdb), so their values are
// directly comparable; otherwise normalized title + year must match. Differing
// schemes (tvdb vs tmdb) are not comparable and fall through, so a matching
// title+year is not discarded just because the ID namespaces differ.
func identitiesAgree(in MatchAnalysisInput) bool {
	arrScheme, arrValue := splitProviderID(in.Arr.ProviderID)
	jellyfinScheme, jellyfinValue := splitProviderID(in.Jellyfin.ProviderID)
	if arrScheme != "" && arrScheme == jellyfinScheme {
		return arrValue == jellyfinValue
	}
	return NormalizeTitle(in.Arr.Title) == NormalizeTitle(in.Jellyfin.Title) &&
		in.Arr.Year == in.Jellyfin.Year
}

// splitProviderID splits a "scheme:value" provider id into its lowercased parts.
// An id without a colon is treated as a scheme with an empty value.
func splitProviderID(id string) (scheme, value string) {
	key, val, ok := strings.Cut(id, ":")
	if !ok {
		return strings.ToLower(strings.TrimSpace(id)), ""
	}
	return strings.ToLower(strings.TrimSpace(key)), strings.ToLower(strings.TrimSpace(val))
}

// episodeAgreement returns the mean title similarity over shared
// (season, episode) keys, how many pairs matched, and the share of matched pairs
// that resolve to the same file (normalized paths, root-independent).
func episodeAgreement(arr, jellyfin []Episode) (agreement float64, matched int, pathRatio float64) {
	if len(arr) == 0 || len(jellyfin) == 0 {
		return 0, 0, 0
	}

	type episodeKey struct{ season, episode int }
	jellyfinByKey := make(map[episodeKey]Episode, len(jellyfin))
	for _, ep := range jellyfin {
		jellyfinByKey[episodeKey{ep.Season, ep.Episode}] = ep
	}

	var similaritySum float64
	var comparablePaths, equalPaths int
	for _, ep := range arr {
		other, ok := jellyfinByKey[episodeKey{ep.Season, ep.Episode}]
		if !ok {
			continue
		}
		matched++
		similaritySum += titleSimilarity(ep.Title, other.Title)
		if ep.Path != "" && other.Path != "" {
			comparablePaths++
			if sameFilePath(ep.Path, other.Path) {
				equalPaths++
			}
		}
	}
	if matched == 0 {
		return 0, 0, 0
	}
	if comparablePaths > 0 {
		pathRatio = float64(equalPaths) / float64(comparablePaths)
	}
	return similaritySum / float64(matched), matched, pathRatio
}

// sameFilePath reports whether two non-empty paths reference the same file. It
// compares canonical paths first, then falls back to the normalized file name so
// the same file exposed under different library roots still counts as a conflict.
func sameFilePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if normalizePath(a) == normalizePath(b) {
		return true
	}
	aName, bName := pathFileName(a), pathFileName(b)
	return aName != "" && aName == bName
}

// normalizePath canonicalizes a path for comparison: backslashes become slashes,
// case is folded, redundant separators and "." segments are dropped, and ".."
// segments are resolved.
func normalizePath(p string) string {
	cleaned := strings.ReplaceAll(strings.TrimSpace(p), "\\", "/")
	segments := make([]string, 0, strings.Count(cleaned, "/")+1)
	for _, segment := range strings.Split(cleaned, "/") {
		switch segment {
		case "", ".":
			continue
		case "..":
			if len(segments) > 0 {
				segments = segments[:len(segments)-1]
			}
		default:
			segments = append(segments, strings.ToLower(segment))
		}
	}
	return strings.Join(segments, "/")
}

// pathFileName returns the final segment of a normalized path.
func pathFileName(p string) string {
	normalized := normalizePath(p)
	if idx := strings.LastIndex(normalized, "/"); idx >= 0 {
		return normalized[idx+1:]
	}
	return normalized
}

// titleSimilarity is the Jaccard overlap of two normalized title token sets.
func titleSimilarity(a, b string) float64 {
	left := uniqueTokens(titleTokens(a))
	right := uniqueTokens(titleTokens(b))
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	intersection := 0
	for token := range left {
		if _, ok := right[token]; ok {
			intersection++
		}
	}
	union := len(left) + len(right) - intersection
	if union == 0 {
		return 1
	}
	return float64(intersection) / float64(union)
}

// scoreIdentity scores how well a filename matches one identity. Title coverage
// dominates; the year is a strong tie-breaker (and a strong penalty when the
// filename carries a conflicting year).
func scoreIdentity(file fileText, id Identity) float64 {
	// A title that is itself a year (e.g. the series "1923") keeps its token
	// (see titleTokens), so the file's parsed years must be comparable too.
	have := file.tokens
	if len(file.years) > 0 {
		have = append(append(make([]string, 0, len(file.tokens)+len(file.years)), file.tokens...),
			yearTokens(file.years)...)
	}
	coverage := tokenCoverage(titleTokens(id.Title), have)
	return titleWeight*coverage + yearWeight*scoreYear(file.years, id.Year)
}

// tokenCoverage is the share of wanted tokens present in have.
func tokenCoverage(want, have []string) float64 {
	if len(want) == 0 {
		return 0
	}
	haveSet := make(map[string]struct{}, len(have))
	for _, token := range have {
		haveSet[token] = struct{}{}
	}
	matched := 0
	for _, token := range want {
		if _, ok := haveSet[token]; ok {
			matched++
		}
	}
	return float64(matched) / float64(len(want))
}

// scoreYear returns full credit on a year match, zero on a conflict, and neutral
// credit when either side has no year to compare.
func scoreYear(fileYears []int, idYear int) float64 {
	if len(fileYears) == 0 || idYear == 0 {
		return unknownYearScore
	}
	for _, year := range fileYears {
		if year == idYear {
			return 1.0
		}
	}
	return 0.0
}

// fileText is the parsed title/year evidence extracted from a path.
type fileText struct {
	source string
	tokens []string
	years  []int
}

// extractFileText pulls the full path, non-year title tokens, and candidate
// years out of a path. The whole path is parsed (not just the basename) so a
// year that lives only in a parent directory — e.g. Sonarr's
// ".../Vanished (2026)/Vanished - S01E01 - Rosefinch.mkv" — is still seen.
// Pure string handling; it never touches the filesystem.
func extractFileText(path string) fileText {
	cleaned := strings.TrimRight(strings.ReplaceAll(path, "\\", "/"), "/")
	return fileText{
		source: cleaned,
		tokens: titleTokens(cleaned),
		years:  extractYears(cleaned),
	}
}

// NormalizeTitle lowercases, strips punctuation, collapses whitespace, and maps
// number words to digits (so "Four" and "4" compare equal). Exported so callers
// reuse the exact same normalization as the adjudicator.
func NormalizeTitle(s string) string {
	return normalizeText(s)
}

// titleTokens returns normalized tokens with year candidates removed. A title
// that consists solely of a year-like token (e.g. the series "1923") keeps that
// token, otherwise it would contribute nothing and both identities would score
// zero coverage.
func titleTokens(s string) []string {
	fields := strings.Fields(normalizeText(s))
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if isYearToken(field) {
			continue
		}
		tokens = append(tokens, field)
	}
	if len(tokens) == 0 {
		return fields
	}
	return tokens
}

// yearTokens renders parsed years as decimal tokens so a year-only title can be
// matched against the years extracted from a path.
func yearTokens(years []int) []string {
	tokens := make([]string, 0, len(years))
	for _, year := range years {
		tokens = append(tokens, strconv.Itoa(year))
	}
	return tokens
}

// extractYears returns every plausible 4-digit production year in s.
func extractYears(s string) []int {
	var years []int
	for _, field := range strings.Fields(normalizeText(s)) {
		if !isYearToken(field) {
			continue
		}
		if year, err := strconv.Atoi(field); err == nil {
			years = append(years, year)
		}
	}
	return years
}

// isYearToken reports whether tok is a 4-digit year in [1900, 2099].
func isYearToken(tok string) bool {
	if len(tok) != 4 {
		return false
	}
	year, err := strconv.Atoi(tok)
	return err == nil && year >= 1900 && year <= 2099
}

// numberWords maps spelled-out digits to their numeric form so titles such as
// "Fantastic Four" and "Fantastic 4" are treated as identical.
var numberWords = map[string]string{
	"zero": "0", "one": "1", "two": "2", "three": "3", "four": "4",
	"five": "5", "six": "6", "seven": "7", "eight": "8", "nine": "9", "ten": "10",
}

// normalizeText lowercases, keeps only alphanumerics, collapses separators to
// single spaces, and maps number words to digits.
func normalizeText(s string) string {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return ' '
	}, s)

	fields := strings.Fields(cleaned)
	for i, field := range fields {
		if replacement, ok := numberWords[field]; ok {
			fields[i] = replacement
		}
	}
	return strings.Join(fields, " ")
}

// uniqueTokens collapses a token slice into a set.
func uniqueTokens(tokens []string) map[string]struct{} {
	set := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		set[token] = struct{}{}
	}
	return set
}

// describeIdentity renders an identity for evidence messages.
func describeIdentity(id Identity) string {
	parts := []string{fmt.Sprintf("%q", id.Title)}
	if id.Year != 0 {
		parts = append(parts, strconv.Itoa(id.Year))
	}
	if id.ProviderID != "" {
		parts = append(parts, id.ProviderID)
	}
	return strings.Join(parts, " ")
}

// clamp01 bounds a value to [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
