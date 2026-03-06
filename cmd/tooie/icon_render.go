package main

import (
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
	"sync"
	"time"
)

type renderedIconKey struct {
	packageName string
	iconPath    string
	modUnixNano int64
	widthCells  int
	heightRows  int
}

type renderedIconCache struct {
	mu    sync.RWMutex
	items map[renderedIconKey]string
}

var pinnedIconRenderCache = renderedIconCache{
	items: map[renderedIconKey]string{},
}

func renderPinnedAppIcon(app launchableApp, widthCells, heightRows int) string {
	widthCells = max(2, widthCells)
	heightRows = max(1, heightRows)

	if strings.TrimSpace(app.IconCachePath) == "" {
		return renderIconBadge(app, widthCells, heightRows)
	}
	info, err := os.Stat(app.IconCachePath)
	if err != nil || info.Size() == 0 {
		return renderIconBadge(app, widthCells, heightRows)
	}

	key := renderedIconKey{
		packageName: app.PackageName,
		iconPath:    app.IconCachePath,
		modUnixNano: info.ModTime().UnixNano(),
		widthCells:  widthCells,
		heightRows:  heightRows,
	}
	if cached, ok := pinnedIconRenderCache.get(key); ok {
		return cached
	}

	rendered, err := renderIconFile(app.IconCachePath, widthCells, heightRows)
	if err != nil || strings.TrimSpace(rendered) == "" {
		rendered = renderIconBadge(app, widthCells, heightRows)
	}
	pinnedIconRenderCache.set(key, rendered)
	return rendered
}

func (c *renderedIconCache) get(key renderedIconKey) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.items[key]
	return v, ok
}

func (c *renderedIconCache) set(key renderedIconKey, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = value
}

func renderIconFile(path string, widthCells, heightRows int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}
	img = trimTransparentImage(img)
	return renderTinyImageANSI(img, widthCells, heightRows), nil
}

func renderTinyImageANSI(img image.Image, widthCells, heightRows int) string {
	widthCells = max(2, widthCells)
	heightRows = max(1, heightRows)
	targetW := widthCells
	targetH := heightRows * 2

	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return ""
	}

	rows := make([]string, 0, heightRows)
	for y := 0; y < targetH; y += 2 {
		var line strings.Builder
		for x := 0; x < targetW; x++ {
			top := sampleScaledColor(img, x, y, targetW, targetH)
			bottom := sampleScaledColor(img, x, y+1, targetW, targetH)
			line.WriteString(ansiFGColor(top))
			line.WriteString(ansiBGColor(bottom))
			line.WriteRune('▀')
		}
		line.WriteString("\x1b[0m")
		rows = append(rows, line.String())
	}
	return strings.Join(rows, "\n")
}

func sampleScaledColor(img image.Image, x, y, targetW, targetH int) color.RGBA {
	b := img.Bounds()
	if targetW <= 0 || targetH <= 0 || b.Dx() <= 0 || b.Dy() <= 0 {
		return color.RGBA{}
	}
	srcX := b.Min.X + (x*b.Dx())/targetW
	srcY := b.Min.Y + (y*b.Dy())/targetH
	r, g, bl, a := img.At(srcX, srcY).RGBA()
	if a == 0 {
		return color.RGBA{0, 0, 0, 0}
	}
	if a < 0xffff {
		r = (r * 0xffff) / a
		g = (g * 0xffff) / a
		bl = (bl * 0xffff) / a
	}
	return color.RGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(bl >> 8),
		A: uint8(a >> 8),
	}
}

func ansiFGColor(c color.RGBA) string {
	if c.A == 0 {
		return "\x1b[39m"
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B)
}

func ansiBGColor(c color.RGBA) string {
	if c.A == 0 {
		return "\x1b[49m"
	}
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.R, c.G, c.B)
}

func renderIconBadge(app launchableApp, widthCells, heightRows int) string {
	widthCells = max(2, widthCells)
	heightRows = max(1, heightRows)

	text := badgeText(app)
	bg := badgeColor(app.PackageName)
	fg := badgeForeground(bg)
	cell := fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm", fg.R, fg.G, fg.B, bg.R, bg.G, bg.B)

	contentWidth := max(widthCells, len([]rune(text))+2)
	lineText := centerBadgeText(text, contentWidth)
	line := cell + lineText + "\x1b[0m"

	rows := make([]string, 0, heightRows)
	for i := 0; i < heightRows; i++ {
		rows = append(rows, line)
	}
	return strings.Join(rows, "\n")
}

func badgeText(app launchableApp) string {
	base := strings.TrimSpace(app.Label)
	if base == "" {
		base = app.PackageName
	}
	parts := strings.FieldsFunc(base, func(r rune) bool {
		return r == ' ' || r == '.' || r == '-' || r == '_' || r == ':'
	})
	runes := make([]rune, 0, 2)
	for _, part := range parts {
		rs := []rune(strings.TrimSpace(part))
		if len(rs) == 0 {
			continue
		}
		runes = append(runes, []rune(strings.ToUpper(string(rs[0])))...)
		if len(runes) >= 2 {
			return string(runes[:2])
		}
	}
	all := []rune(strings.ToUpper(strings.ReplaceAll(base, " ", "")))
	if len(all) >= 2 {
		return string(all[:2])
	}
	if len(all) == 1 {
		return string(all)
	}
	return "?"
}

func centerBadgeText(text string, width int) string {
	runes := []rune(text)
	if len(runes) > width {
		runes = runes[:width]
	}
	pad := width - len(runes)
	left := pad / 2
	right := pad - left
	return strings.Repeat(" ", left) + string(runes) + strings.Repeat(" ", right)
}

func badgeColor(seed string) color.RGBA {
	if strings.TrimSpace(seed) == "" {
		return color.RGBA{90, 110, 160, 255}
	}
	var h uint32 = 2166136261
	for _, b := range []byte(seed) {
		h ^= uint32(b)
		h *= 16777619
	}
	return color.RGBA{
		R: uint8(64 + (h & 0x3f)),
		G: uint8(72 + ((h >> 8) & 0x3f)),
		B: uint8(112 + ((h >> 16) & 0x3f)),
		A: 255,
	}
}

func badgeForeground(bg color.RGBA) color.RGBA {
	luma := (299*int(bg.R) + 587*int(bg.G) + 114*int(bg.B)) / 1000
	if luma >= 140 {
		return color.RGBA{20, 24, 32, 255}
	}
	return color.RGBA{240, 244, 248, 255}
}

func pruneRenderedIconCache(maxAge time.Duration) {
	if maxAge <= 0 {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	pinnedIconRenderCache.mu.Lock()
	defer pinnedIconRenderCache.mu.Unlock()
	for key := range pinnedIconRenderCache.items {
		if time.Unix(0, key.modUnixNano).Before(cutoff) {
			delete(pinnedIconRenderCache.items, key)
		}
	}
}
