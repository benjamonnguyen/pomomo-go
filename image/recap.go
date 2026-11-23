// Package image renders Pomomo's stats cards as PNGs.
package image

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"time"
	"unicode"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// recap card size in logical pixels; rendered at scale for crisp display
const (
	recapW, recapH = 900, 440
	scale          = 2
)

var (
	//go:embed assets/NotoSans-Regular.ttf
	regularTTF []byte
	//go:embed assets/NotoSans-SemiBold.ttf
	semiboldTTF []byte
	//go:embed assets/NotoSans-Bold.ttf
	boldTTF []byte
	//go:embed assets/fire.png
	firePNG []byte

	colBg     = hex(0x1b1c1f)
	colCard   = hex(0x232428)
	colTile   = hex(0x161719)
	colAccent = hex(0xed6a43)
	colText   = hex(0xf2f3f5)
	colMuted  = hex(0xb5bac1)
	colFaint  = hex(0x80848e)
)

// Recap is a user's week in a server. Days are UTC days starting Monday.
type Recap struct {
	Name      string
	WeekStart time.Time // Monday 00:00 UTC
	Now       time.Time
	Days      [7]time.Duration
	Streak    int // 0–3 fire tiers
	Month     time.Duration
	Lifetime  time.Duration
}

func (r Recap) Week() (total time.Duration) {
	for _, d := range r.Days {
		total += d
	}
	return total
}

var streakCaptions = []string{"NO STREAK YET", "4+ DAYS THIS WEEK", "16+ DAYS THIS MONTH", "64+ DAYS THIS QUARTER"}

// RenderRecap draws the weekly recap card as a PNG.
func RenderRecap(r Recap) ([]byte, error) {
	c := newCanvas(recapW, recapH)
	fire, err := png.Decode(bytes.NewReader(firePNG))
	if err != nil {
		return nil, fmt.Errorf("failed to decode fire: %w", err)
	}
	r.Streak = min(max(r.Streak, 0), 3)

	// background, card, accent bar
	c.roundRect(0, 0, recapW, recapH, 0, colBg)
	c.roundRect(24, 24, recapW-48, recapH-48, 10, colCard)
	c.roundRect(24, 24, 8, recapH-48, 0, colAccent)

	// header: initial badge, title, date range
	name := drawable(r.Name)
	initial, title := "?", "Your Weekly Recap"
	if name != "" {
		initial = strings.ToUpper(string([]rune(name)[:1]))
		titleFace := c.face(bold, 29)
		title = c.ellipsize(titleFace, name, 680-156-c.measure(titleFace, "'s Weekly Recap")) + "'s Weekly Recap"
	}
	c.roundRect(58, 55, 76, 76, 38, colAccent)
	c.text(c.face(bold, 34), initial, 96, 105, colText, alignCenter)
	c.text(c.face(bold, 29), title, 156, 85, colText, alignLeft)
	weekEnd := r.WeekStart.AddDate(0, 0, 6)
	c.text(c.face(regular, 15), r.WeekStart.Format("Mon, Jan 2")+" – "+weekEnd.Format("Mon, Jan 2")+" UTC", 156, 112, colMuted, alignLeft)

	// streak: earned tiers in color, the rest as grey ghosts, right aligned
	ghost := silhouette(fire, colMuted)
	fireH := 34.0
	fireW := fireH * float64(fire.Bounds().Dx()) / float64(fire.Bounds().Dy())
	for i := range 3 {
		x := 845 - fireW*float64(3-i) - 6*float64(2-i)
		if i >= 3-r.Streak {
			c.image(fire, x, 62, fireW, fireH, 1)
		} else {
			c.image(ghost, x, 62, fireW, fireH, 0.18)
		}
	}
	c.text(c.face(regular, 12), streakCaptions[r.Streak], 845, 116, colMuted, alignRight)

	// weekly stats: Mon–Sun bars scaled to the best day, at least 1h
	c.text(c.face(bold, 14), "WEEKLY STATS", 58, 180, colMuted, alignLeft)
	peak := time.Hour
	for _, d := range r.Days {
		peak = max(peak, d)
	}
	today := int(r.Now.Sub(r.WeekStart).Hours() / 24)
	dayFace, todayFace, valueFace := c.face(regular, 11), c.face(bold, 11), c.face(semibold, 11)
	const top, bottom = 196.0, 320.0
	for i, d := range r.Days {
		x := 58 + float64(i)*79.3
		future := i > today
		track := colTile
		if future {
			track = fade(colTile, 0.5)
		}
		c.roundRect(x, top, 64, bottom-top, 6, track)
		switch {
		case d > 0:
			h := math.Max(6, (bottom-top)*float64(d)/float64(peak))
			c.roundRect(x, bottom-h, 64, h, 6, colAccent)
			if h > 26 {
				c.text(valueFace, FormatDuration(d), x+32, bottom-h+17, colText, alignCenter)
			} else {
				c.text(valueFace, FormatDuration(d), x+32, bottom-h-6, colMuted, alignCenter)
			}
		case !future:
			c.roundRect(x, bottom-3, 64, 3, 1, colAccent)
		}
		day := strings.ToUpper(r.WeekStart.AddDate(0, 0, i).Format("Mon"))
		switch {
		case i == today:
			c.text(todayFace, day, x+32, 346, colAccent, alignCenter)
		case future:
			c.text(dayFace, day, x+32, 346, colFaint, alignCenter)
		default:
			c.text(dayFace, day, x+32, 346, colMuted, alignCenter)
		}
	}

	// total focus panel
	c.roundRect(630, 180, 214, 176, 8, colTile)
	c.text(c.face(bold, 11), "TOTAL FOCUS", 646, 206, colMuted, alignLeft)
	label, value := c.face(regular, 11), c.face(bold, 13)
	for i, row := range [][2]string{
		{"This week", FormatDuration(r.Week())},
		{"This month", FormatDuration(r.Month)},
		{"Lifetime", FormatHours(r.Lifetime)},
	} {
		y := 236 + float64(i)*42
		c.text(label, row[0], 646, y, colText, alignLeft)
		c.text(value, row[1], 646, y+18, colText, alignLeft)
	}

	// footer
	foot := c.face(regular, 12)
	tagline := "A fresh week is a fresh start."
	if best, i := bestDay(r.Days); best > 0 {
		tagline = fmt.Sprintf("Best day: %s · %s", r.WeekStart.AddDate(0, 0, i).Format("Monday"), FormatDuration(best))
	}
	c.text(foot, tagline, 58, 392, colText, alignLeft)
	c.text(foot, "Pomomo", 845, 392, colText, alignRight)

	var buf bytes.Buffer
	if err := png.Encode(&buf, c.img); err != nil {
		return nil, fmt.Errorf("failed to encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// FormatDuration formats d as "2h 40m", "2h" or "45m".
func FormatDuration(d time.Duration) string {
	d = d.Truncate(time.Minute)
	h, m := int(d.Hours()), int(d.Minutes())%60
	switch {
	case h == 0:
		return fmt.Sprintf("%dm", m)
	case m == 0:
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

// FormatHours formats d in whole hours with thousands separators once it's at least an hour.
func FormatHours(d time.Duration) string {
	if d < time.Hour {
		return FormatDuration(d)
	}
	h := int(d.Hours())
	if h >= 1000 {
		return fmt.Sprintf("%d,%03dh", h/1000, h%1000)
	}
	return fmt.Sprintf("%dh", h)
}

func bestDay(days [7]time.Duration) (best time.Duration, idx int) {
	for i, d := range days {
		if d > best {
			best, idx = d, i
		}
	}
	return best, idx
}

func hex(v uint32) color.RGBA {
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 0xff}
}

func fade(c color.RGBA, a float64) color.RGBA {
	return color.RGBA{uint8(float64(c.R) * a), uint8(float64(c.G) * a), uint8(float64(c.B) * a), uint8(255 * a)}
}

// silhouette paints src's shape in col
func silhouette(src image.Image, col color.RGBA) image.Image {
	b := src.Bounds()
	out := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := src.At(x, y).RGBA()
			out.SetNRGBA(x, y, color.NRGBA{col.R, col.G, col.B, uint8(a >> 8)})
		}
	}
	return out
}

type align int

const (
	alignLeft align = iota
	alignCenter
	alignRight
)

var (
	regular  = mustParse(regularTTF)
	semibold = mustParse(semiboldTTF)
	bold     = mustParse(boldTTF)
)

func mustParse(ttf []byte) *opentype.Font {
	f, err := opentype.Parse(ttf)
	if err != nil {
		panic(fmt.Errorf("failed to parse embedded font: %w", err))
	}
	return f
}

// canvas draws in logical coordinates onto an image scaled by scale.
type canvas struct {
	img *image.RGBA
}

func newCanvas(w, h int) *canvas {
	return &canvas{img: image.NewRGBA(image.Rect(0, 0, w*scale, h*scale))}
}

func (c *canvas) face(f *opentype.Font, size float64) font.Face {
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size * scale, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err) // only fails on invalid options
	}
	return face
}

// drawable drops runes the fonts have no glyph for (CJK, emoji, joiners) and trims the result.
func drawable(s string) string {
	var buf sfnt.Buffer
	var b strings.Builder
	for _, r := range s {
		if unicode.IsSpace(r) {
			b.WriteRune(' ')
			continue
		}
		if i, err := bold.GlyphIndex(&buf, r); err == nil && i != 0 && unicode.IsPrint(r) {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func (c *canvas) roundRect(x, y, w, h, r float64, col color.Color) {
	r = math.Min(r, math.Min(w, h)/2)
	// rasterize only the rect's bounding box, in its own coordinates
	bounds := image.Rect(int(math.Floor(x*scale)), int(math.Floor(y*scale)), int(math.Ceil((x+w)*scale)), int(math.Ceil((y+h)*scale)))
	X, Y := float32(x*scale)-float32(bounds.Min.X), float32(y*scale)-float32(bounds.Min.Y)
	W, H, R := float32(w*scale), float32(h*scale), float32(r*scale)
	const k = 0.5523 // cubic bezier approximation of a quarter circle
	z := vector.NewRasterizer(bounds.Dx(), bounds.Dy())
	z.MoveTo(X+R, Y)
	z.LineTo(X+W-R, Y)
	z.CubeTo(X+W-R+R*k, Y, X+W, Y+R-R*k, X+W, Y+R)
	z.LineTo(X+W, Y+H-R)
	z.CubeTo(X+W, Y+H-R+R*k, X+W-R+R*k, Y+H, X+W-R, Y+H)
	z.LineTo(X+R, Y+H)
	z.CubeTo(X+R-R*k, Y+H, X, Y+H-R+R*k, X, Y+H-R)
	z.LineTo(X, Y+R)
	z.CubeTo(X, Y+R-R*k, X+R-R*k, Y, X+R, Y)
	z.ClosePath()
	z.Draw(c.img, bounds, image.NewUniform(col), image.Point{})
}

func (c *canvas) image(src image.Image, x, y, w, h, alpha float64) {
	dst := image.Rect(int(x*scale), int(y*scale), int((x+w)*scale), int((y+h)*scale))
	scaled := image.NewRGBA(image.Rect(0, 0, dst.Dx(), dst.Dy()))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), src, src.Bounds(), xdraw.Src, nil)
	var mask image.Image
	if alpha < 1 {
		mask = image.NewUniform(color.Alpha{uint8(255 * alpha)})
	}
	xdraw.DrawMask(c.img, dst, scaled, image.Point{}, mask, image.Point{}, xdraw.Over)
}

func (c *canvas) measure(f font.Face, s string) float64 {
	return float64(font.MeasureString(f, s)) / 64 / scale
}

// text draws s with its baseline at y, aligned relative to x
func (c *canvas) text(f font.Face, s string, x, y float64, col color.Color, a align) {
	switch a {
	case alignCenter:
		x -= c.measure(f, s) / 2
	case alignRight:
		x -= c.measure(f, s)
	}
	d := font.Drawer{Dst: c.img, Src: image.NewUniform(col), Face: f, Dot: fixed.P(int(x*scale), int(y*scale))}
	d.DrawString(s)
}

// ellipsize trims s with … until it fits in maxW
func (c *canvas) ellipsize(f font.Face, s string, maxW float64) string {
	if c.measure(f, s) <= maxW {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && c.measure(f, string(r)+"…") > maxW {
		r = r[:len(r)-1]
	}
	return strings.TrimSpace(string(r)) + "…"
}
