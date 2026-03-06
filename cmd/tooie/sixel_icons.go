package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	stdraw "image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"strings"
	"sync"

	"github.com/mattn/go-sixel"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/sys/unix"
)

const sixelNoScroll = "\x1b[?80l"

type cellDim struct {
	width  int
	height int
}

type sixelRenderResult struct {
	data     string
	widthPx  int
	heightPx int
}

type sixelCacheKey struct {
	iconPath    string
	modUnixNano int64
	widthCells  int
	heightCells int
	cellW       int
	cellH       int
}

type sixelOverlay struct {
	row  int
	col  int
	data string
}

type sixelCache struct {
	mu    sync.RWMutex
	items map[sixelCacheKey]sixelRenderResult
}

var pinnedSixelCache = sixelCache{items: map[sixelCacheKey]sixelRenderResult{}}

func sixelCellGeometry() (cellDim, bool) {
	if strings.TrimSpace(os.Getenv("TOOIE_DISABLE_SIXEL")) != "" {
		return cellDim{}, false
	}
	fd := int(os.Stdout.Fd())
	ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil {
		return cellDim{}, false
	}
	cols := int(ws.Col)
	rows := int(ws.Row)
	if cols <= 0 || rows <= 0 {
		return cellDim{}, false
	}
	cellW := int(ws.Xpixel) / cols
	cellH := int(ws.Ypixel) / rows
	if cellW < 1 {
		cellW = 10
	}
	if cellH < 1 {
		cellH = 20
	}
	return cellDim{width: cellW, height: cellH}, true
}

func renderPinnedAppSixel(app launchableApp, widthCells, heightCells int, geom cellDim) (sixelRenderResult, bool) {
	if strings.TrimSpace(app.IconCachePath) == "" {
		return sixelRenderResult{}, false
	}
	info, err := os.Stat(app.IconCachePath)
	if err != nil || info.Size() == 0 {
		return sixelRenderResult{}, false
	}
	key := sixelCacheKey{
		iconPath:    app.IconCachePath,
		modUnixNano: info.ModTime().UnixNano(),
		widthCells:  widthCells,
		heightCells: heightCells,
		cellW:       geom.width,
		cellH:       geom.height,
	}
	if cached, ok := pinnedSixelCache.get(key); ok {
		return cached, true
	}
	res, err := renderSixelFromFile(app.IconCachePath, widthCells, heightCells, geom)
	if err != nil || strings.TrimSpace(res.data) == "" {
		return sixelRenderResult{}, false
	}
	pinnedSixelCache.set(key, res)
	return res, true
}

func (c *sixelCache) get(key sixelCacheKey) (sixelRenderResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.items[key]
	return v, ok
}

func (c *sixelCache) set(key sixelCacheKey, value sixelRenderResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = value
}

func clearPinnedSixelCache() {
	pinnedSixelCache.mu.Lock()
	defer pinnedSixelCache.mu.Unlock()
	pinnedSixelCache.items = map[sixelCacheKey]sixelRenderResult{}
}

func renderSixelFromFile(path string, widthCells, heightCells int, geom cellDim) (sixelRenderResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return sixelRenderResult{}, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return sixelRenderResult{}, err
	}
	return renderSixelImage(img, widthCells, heightCells, geom)
}

func renderSixelImage(src image.Image, widthCells, heightCells int, geom cellDim) (sixelRenderResult, error) {
	targetW := max(1, widthCells*geom.width)
	targetH := max(1, heightCells*geom.height)
	stdSize := max(targetW, targetH)
	src = trimTransparentImage(src)
	standardized := standardizeImage(src, stdSize)
	scaled := scaleImageAspectFit(standardized, targetW, targetH)
	var buf bytes.Buffer
	enc := sixel.NewEncoder(&buf)
	if err := enc.Encode(scaled); err != nil {
		return sixelRenderResult{}, err
	}
	b := scaled.Bounds()
	return sixelRenderResult{
		data:     buf.String(),
		widthPx:  b.Dx(),
		heightPx: b.Dy(),
	}, nil
}

func scaleImage(src image.Image, targetW, targetH int) image.Image {
	if targetW <= 0 || targetH <= 0 {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), stdraw.Over, nil)
	return dst
}

func scaleImageAspectFit(src image.Image, maxW, maxH int) image.Image {
	if maxW <= 0 || maxH <= 0 {
		return src
	}
	b := src.Bounds()
	srcW := b.Dx()
	srcH := b.Dy()
	if srcW <= 0 || srcH <= 0 {
		return src
	}
	scaleW := float64(maxW) / float64(srcW)
	scaleH := float64(maxH) / float64(srcH)
	scale := scaleW
	if scaleH < scaleW {
		scale = scaleH
	}
	targetW := max(1, int(float64(srcW)*scale))
	targetH := max(1, int(float64(srcH)*scale))
	return scaleImage(src, targetW, targetH)
}

func standardizeImage(src image.Image, size int) image.Image {
	if size <= 0 {
		size = 128
	}
	b := src.Bounds()
	srcW := b.Dx()
	srcH := b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	transparent := color.RGBA{0, 0, 0, 0}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dst.Set(x, y, transparent)
		}
	}
	scaleW := float64(size) / float64(srcW)
	scaleH := float64(size) / float64(srcH)
	scale := scaleW
	if scaleH < scaleW {
		scale = scaleH
	}
	scaledW := max(1, int(float64(srcW)*scale))
	scaledH := max(1, int(float64(srcH)*scale))
	offsetX := (size - scaledW) / 2
	offsetY := (size - scaledH) / 2
	scaled := scaleImage(src, scaledW, scaledH)
	stdraw.Draw(dst, image.Rect(offsetX, offsetY, offsetX+scaledW, offsetY+scaledH), scaled, image.Point{}, stdraw.Over)
	return dst
}

func renderSixelOverlays(overlays []sixelOverlay) string {
	if len(overlays) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(sixelNoScroll)
	for _, ov := range overlays {
		if ov.row < 1 || ov.col < 1 || strings.TrimSpace(ov.data) == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("\x1b[%d;%dH", ov.row, ov.col))
		b.WriteString(ov.data)
	}
	return b.String()
}

func pixelToCellCeil(px, cellPx int) int {
	if px <= 0 || cellPx <= 0 {
		return 0
	}
	return int(math.Ceil(float64(px) / float64(cellPx)))
}
