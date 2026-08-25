package overlay

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"strconv"
	"strings"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

// BannerStyle configures the "X days until deletion" banner drawn onto a poster.
type BannerStyle struct {
	// Text is the final banner text (already resolved, e.g. "in 3 days").
	Text string
	// FontSizePercent is the font size as a percentage of the poster's short side.
	FontSizePercent float64
	// FontColor is the text color (#RRGGBB, #RRGGBBAA, rgba(), or "transparent"/"none").
	FontColor string
	// BackgroundColor is the pill color; "transparent"/"none" draws no pill.
	BackgroundColor string
	// PaddingPercent is the pill padding as a percentage of the short side.
	PaddingPercent float64
	// CornerRadiusPercent is the pill corner radius as a percentage of the short side.
	CornerRadiusPercent float64
	// FontPath optionally loads a TTF/OTF font from disk; empty uses the bundled font.
	FontPath string
}

// WidthBudget is the maximum fraction of the poster width the banner may occupy
// before the font is shrunk to fit (mirrors Maintainerr's shrink-to-fit).
const widthBudget = 0.88

// RenderBanner draws the banner onto the poster and returns the composite as
// JPEG bytes (quality 92, matching Maintainerr). An empty Text returns the
// original bytes untouched.
func RenderBanner(poster []byte, style BannerStyle) ([]byte, error) {
	if strings.TrimSpace(style.Text) == "" {
		return poster, nil
	}

	src, _, err := image.Decode(bytes.NewReader(poster))
	if err != nil {
		return nil, fmt.Errorf("decode poster: %w", err)
	}

	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == 0 || h == 0 {
		return nil, fmt.Errorf("poster has empty bounds")
	}

	short := float64(min(w, h))

	// Defensive clamps: config values can arrive negative via a direct API
	// call, which would otherwise produce a degenerate pill geometry.
	fontSize := short * clamp(style.FontSizePercent, 1, 20) / 100
	minSize := 10.0
	maxSize := short * 0.15
	fontSize = clamp(fontSize, minSize, maxSize)

	face, err := loadFace(style.FontPath, fontSize)
	if err != nil {
		return nil, fmt.Errorf("load font: %w", err)
	}

	dc := gg.NewContextForImage(src)
	dc.SetFontFace(face)

	// Measure the TIGHT glyph bounds, not the font's line height: the pill
	// should hug the text, with padding adding the space around it. Line
	// height (ascent+descent) is ~1.4x the em and makes the pill look half
	// empty above and below the text.
	measure := func() (float64, float64) {
		bounds, _ := font.BoundString(face, style.Text)
		tw := float64(bounds.Max.X-bounds.Min.X) / 64.0
		th := float64(bounds.Max.Y-bounds.Min.Y) / 64.0
		if tw <= 0 {
			tw = fontSize
		}
		if th <= 0 {
			th = fontSize
		}
		return tw, th
	}

	textW, textH := measure()

	// Shrink-to-fit so the banner never exceeds the poster width budget.
	for {
		if textW <= float64(w)*widthBudget || fontSize <= minSize {
			break
		}
		fontSize = max(minSize, fontSize*0.9)
		face, err = loadFace(style.FontPath, fontSize)
		if err != nil {
			return nil, fmt.Errorf("load font: %w", err)
		}
		dc.SetFontFace(face)
		textW, textH = measure()
	}

	pad := short * clamp(style.PaddingPercent, 0, 10) / 100
	pillW := textW + 2*pad
	pillH := textH + 2*pad
	radius := min(short*clamp(style.CornerRadiusPercent, 0, 50)/100, pillH/2)

	// Dock at the bottom edge, horizontally centered.
	x := max(0, (float64(w)-pillW)/2)
	y := max(0, float64(h)-pillH-pad)

	if bg, ok := parseColor(style.BackgroundColor); ok {
		dc.SetColor(bg)
		dc.DrawRoundedRectangle(x, y, pillW, pillH, radius)
		dc.Fill()
	}

	fg := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	if c, ok := parseColor(style.FontColor); ok {
		fg = c
	}
	dc.SetColor(fg)

	// Draw the text with the baseline placed so the tight glyph box (not the
	// line-height metrics) is vertically centered inside the pill.
	gb, _ := font.BoundString(face, style.Text)
	baselineY := y + pad + textH/2 - float64(gb.Min.Y+gb.Max.Y)/128.0
	dc.DrawString(style.Text, x+pad, baselineY)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dc.Image(), &jpeg.Options{Quality: 92}); err != nil {
		return nil, fmt.Errorf("encode overlay: %w", err)
	}

	return buf.Bytes(), nil
}

// bannerText resolves the banner text for a day count. Day 0 is "today",
// day 1 is "in 1 day", and 2+ fills the template's "{days}" placeholder.
func bannerText(template string, daysLeft int) string {
	switch daysLeft {
	case 0:
		return "today"
	case 1:
		return "in 1 day"
	default:
		return strings.ReplaceAll(template, "{days}", strconv.Itoa(daysLeft))
	}
}

// loadFace loads a font face from disk (custom TTF/OTF) or the bundled font.
func loadFace(fontPath string, size float64) (font.Face, error) {
	if fontPath != "" {
		face, err := gg.LoadFontFace(fontPath, size)
		if err != nil {
			return nil, fmt.Errorf("load %q: %w", fontPath, err)
		}
		return face, nil
	}

	parsed, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, fmt.Errorf("parse bundled font: %w", err)
	}
	return opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}

// parseColor converts a color string into a color.RGBA. Supports #RRGGBB,
// #RRGGBBAA (alpha last, matching gg and CSS Color 4), rgba(r,g,b,a) with alpha
// 0-1, and the CSS color names "transparent"/"none" (reported as not-ok so
// callers skip painting).
func parseColor(s string) (color.RGBA, bool) {
	t := strings.TrimSpace(strings.ToLower(s))
	if t == "" || t == "transparent" || t == "none" {
		return color.RGBA{}, false
	}

	if strings.HasPrefix(t, "#") {
		hex := strings.TrimPrefix(t, "#")
		if len(hex) == 6 || len(hex) == 8 {
			if v, err := strconv.ParseUint(hex, 16, 32); err == nil {
				if len(hex) == 8 {
					// #RRGGBBAA: the alpha byte is last.
					return color.RGBA{
						R: uint8(v >> 24),
						G: uint8(v >> 16),
						B: uint8(v >> 8),
						A: uint8(v),
					}, true
				}
				return color.RGBA{
					R: uint8(v >> 16),
					G: uint8(v >> 8),
					B: uint8(v),
					A: 255,
				}, true
			}
		}
		return color.RGBA{}, false
	}

	if strings.HasPrefix(t, "rgba(") && strings.HasSuffix(t, ")") {
		inner := strings.TrimSuffix(strings.TrimPrefix(t, "rgba("), ")")
		parts := strings.Split(inner, ",")
		if len(parts) == 4 {
			r, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
			g, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
			b, err3 := strconv.Atoi(strings.TrimSpace(parts[2]))
			a, err4 := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
			if err1 == nil && err2 == nil && err3 == nil && err4 == nil {
				return color.RGBA{
					R: uint8(clamp(float64(r), 0, 255)),
					G: uint8(clamp(float64(g), 0, 255)),
					B: uint8(clamp(float64(b), 0, 255)),
					A: uint8(clamp(a, 0, 1) * 255),
				}, true
			}
		}
		return color.RGBA{}, false
	}

	// Unknown color string: report not-ok (callers fall back to white text /
	// no pill rather than failing the whole render).
	return color.RGBA{}, false
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
