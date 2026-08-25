package overlay

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// testPoster encodes a solid-color JPEG of the given size.
func testPoster(t *testing.T, w, h int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0x2a
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode test poster: %v", err)
	}
	return buf.Bytes()
}

func decodeJPEG(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode rendered image: %v", err)
	}
	return img
}

func TestRenderBanner_ValidJPEG(t *testing.T) {
	poster := testPoster(t, 300, 450)
	style := BannerStyle{
		Text:                "in 3 days",
		FontSizePercent:     5,
		FontColor:           "#ffffff",
		BackgroundColor:     "rgba(0,0,0,0.75)",
		PaddingPercent:      2,
		CornerRadiusPercent: 50,
	}

	out, err := RenderBanner(poster, style)
	if err != nil {
		t.Fatalf("RenderBanner returned error: %v", err)
	}

	img := decodeJPEG(t, out)
	if img.Bounds().Dx() != 300 || img.Bounds().Dy() != 450 {
		t.Fatalf("rendered image has wrong dimensions: got %dx%d, want 300x450", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestRenderBanner_TransparentBackground(t *testing.T) {
	poster := testPoster(t, 300, 450)
	style := BannerStyle{
		Text:                "today",
		FontSizePercent:     5,
		FontColor:           "#ffffff",
		BackgroundColor:     "transparent",
		PaddingPercent:      2,
		CornerRadiusPercent: 50,
	}

	out, err := RenderBanner(poster, style)
	if err != nil {
		t.Fatalf("RenderBanner returned error: %v", err)
	}
	decodeJPEG(t, out)
}

func TestRenderBanner_EmptyTextReturnsOriginal(t *testing.T) {
	poster := testPoster(t, 100, 150)
	out, err := RenderBanner(poster, BannerStyle{Text: "  "})
	if err != nil {
		t.Fatalf("RenderBanner returned error: %v", err)
	}
	if !bytes.Equal(out, poster) {
		t.Fatal("empty-text render must return the original bytes unchanged")
	}
}

func TestRenderBanner_SmallPosterDoesNotBlowUp(t *testing.T) {
	poster := testPoster(t, 20, 30)
	out, err := RenderBanner(poster, BannerStyle{Text: "in 30 days", FontSizePercent: 5})
	if err != nil {
		t.Fatalf("RenderBanner on tiny poster returned error: %v", err)
	}
	decodeJPEG(t, out)
}

func TestRenderBanner_BadPoster(t *testing.T) {
	if _, err := RenderBanner([]byte("not an image"), BannerStyle{Text: "in 3 days"}); err == nil {
		t.Fatal("RenderBanner must error on undecodable input")
	}
}

func TestBannerText(t *testing.T) {
	tests := []struct {
		name     string
		template string
		daysLeft int
		want     string
	}{
		{name: "today", template: "in {days} days", daysLeft: 0, want: "today"},
		{name: "one day", template: "in {days} days", daysLeft: 1, want: "in 1 day"},
		{name: "plural", template: "in {days} days", daysLeft: 3, want: "in 3 days"},
		{name: "custom template", template: "Leaving in {days} days", daysLeft: 7, want: "Leaving in 7 days"},
		{name: "custom template today", template: "Leaving in {days} days", daysLeft: 0, want: "today"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bannerText(tt.template, tt.daysLeft); got != tt.want {
				t.Fatalf("bannerText(%q, %d) = %q, want %q", tt.template, tt.daysLeft, got, tt.want)
			}
		})
	}
}

func TestParseColor(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  color.RGBA
		ok    bool
	}{
		{name: "hex rgb", input: "#ff0000", want: color.RGBA{R: 255, A: 255}, ok: true},
		{name: "hex rgba", input: "#ffffff80", want: color.RGBA{R: 255, G: 255, B: 255, A: 128}, ok: true},
		{name: "rgba", input: "rgba(0,0,0,0.75)", want: color.RGBA{A: 191}, ok: true},
		{name: "rgba clamped", input: "rgba(300,0,0,2)", want: color.RGBA{R: 255, A: 255}, ok: true},
		{name: "transparent", input: "transparent", ok: false},
		{name: "none", input: "none", ok: false},
		{name: "empty", input: "", ok: false},
		{name: "garbage", input: "not-a-color", ok: false},
		{name: "bad hex", input: "#zzz", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseColor(tt.input)
			if ok != tt.ok {
				t.Fatalf("parseColor(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Fatalf("parseColor(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}
