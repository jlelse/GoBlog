package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/carlmjohnson/requests"
	"github.com/samber/lo"
	"go.goblog.app/app/pkgs/bufferpool"
	"go.goblog.app/app/pkgs/htmlbuilder"
	"go.goblog.app/app/pkgs/plugintypes"
)

type variantType struct {
	Format string
	Width  int
}

type mediaOptimizedRow struct {
	OriginalHash  string
	VariantType   string
	OptimizedHash string
	Width         int
	Height        int
}

type mediaVariantGroup struct {
	byFormat       map[string][]*mediaOptimizedRow
	sortedFormats  []string
	fallbackFormat string
	fallbackRow    *mediaOptimizedRow
}

var (
	hexHashRe           = regexp.MustCompile(`^[0-9a-fA-F]+$`)
	imageFormatPriority = map[string]int{"avif": 0, "jpeg": 2, "jpg": 2, "png": 3}
)

func (a *goBlog) initMediaOptimization() {
	if !a.mediaOptimizationEnabled() {
		return
	}
	a.mediaOptimizationVariants = nil
	for _, format := range a.cfg.MediaOptimization.Formats {
		if format == "" {
			continue
		}
		for _, w := range a.cfg.MediaOptimization.Widths {
			a.mediaOptimizationVariants = append(a.mediaOptimizationVariants, variantType{
				Format: format,
				Width:  w,
			})
		}
	}
	w := 700
	if a.cfg.MediaOptimization.ContentMaxWidth > 0 {
		w = a.cfg.MediaOptimization.ContentMaxWidth
	}
	a.mediaOptimizationSizes = fmt.Sprintf("(max-width: %dpx) 100vw, %dpx", w, w)
}

func (a *goBlog) mediaOptimizationEnabled() bool {
	return a.cfg.MediaOptimization != nil && a.cfg.MediaOptimization.Enabled
}

func (a *goBlog) mediaOptimizationImgproxyConfigured() bool {
	return a.cfg.MediaOptimization != nil && a.cfg.MediaOptimization.ImgproxyURL != ""
}

func isImageExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png":
		return true
	default:
		return false
	}
}

func variantTypeAllowedForSource(sourceFormat, targetFormat string) bool {
	switch sourceFormat {
	case "jpg", "jpeg":
		return targetFormat == "jpg" || targetFormat == "jpeg" || targetFormat == "avif"
	case "png":
		return targetFormat == "png" || targetFormat == "avif"
	default:
		return false
	}
}

func (a *goBlog) extractMediaHashFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	filename := path.Base(u.Path)
	if filename == "" || strings.Count(filename, ".") != 1 {
		return ""
	}
	hash := strings.TrimSuffix(filename, path.Ext(filename))
	if !hexHashRe.MatchString(hash) {
		return ""
	}
	return hash
}

func (a *goBlog) optimizeMediaFile(originalHash, ext string) {
	if !a.mediaOptimizationEnabled() || !a.mediaOptimizationImgproxyConfigured() ||
		len(a.mediaOptimizationVariants) == 0 || !isImageExtension(ext) {
		return
	}

	sourceURL := a.mediaFileLocation(originalHash + ext)
	if sourceURL == "" {
		return
	}
	sourceURL = a.getFullAddress(sourceURL)
	sourceFormat := strings.ToLower(strings.TrimPrefix(ext, "."))

	// Delete existing variants for this original before regenerating
	if existing, err := a.db.mediaOptimizedByOriginal(originalHash); err == nil {
		for _, v := range existing {
			f, _ := parseVariantType(v.VariantType)
			variantFilename := v.OptimizedHash + "." + f
			_ = a.deleteMediaFileRaw(variantFilename)
		}
		_ = a.db.mediaOptimizedDeleteByOriginal(originalHash)
	} else {
		a.error("Media optimization: failed to query existing variants", "err", err, "hash", originalHash)
	}

	const concurrency = 4
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, variant := range a.mediaOptimizationVariants {
		if !variantTypeAllowedForSource(sourceFormat, variant.Format) {
			continue
		}

		sem <- struct{}{}
		wg.Add(1)

		go func(v variantType) {
			defer func() { <-sem }()
			defer wg.Done()

			buf := bufferpool.Get()
			h := sha256.New()
			w := io.MultiWriter(buf, h)

			if err := a.callImgproxy(sourceURL, &v, w); err != nil {
				bufferpool.Put(buf)
				a.error("Media optimization: imgproxy call failed", "err", err, "source", sourceURL, "variant", v.Format)
				return
			}

			// Decode actual image dimensions from the buffer for supported formats
			var width, height int
			switch v.Format {
			case "jpeg", "png":
				if cfg, _, decodeErr := image.DecodeConfig(bytes.NewReader(buf.Bytes())); decodeErr == nil {
					width = cfg.Width
					height = cfg.Height
				}
			}

			optimizedHash := fmt.Sprintf("%x", h.Sum(nil))
			optimizedFilename := optimizedHash + "." + v.Format

			if buf.Len() == 0 {
				bufferpool.Put(buf)
				a.error("Media optimization: empty response from imgproxy", "source", sourceURL, "variant", v.Format)
				return
			}

			_, saveErr := a.saveMediaFile(optimizedFilename, buf)
			bufferpool.Put(buf)
			if saveErr != nil {
				a.error("Media optimization: failed to save variant", "err", saveErr, "file", optimizedFilename)
				return
			}

			vt := fmt.Sprintf("%s_%d", v.Format, v.Width)
			if err := a.db.mediaOptimizedInsert(&mediaOptimizedRow{
				OriginalHash:  originalHash,
				VariantType:   vt,
				OptimizedHash: optimizedHash,
				Width:         width,
				Height:        height,
			}); err != nil {
				a.error("Media optimization: failed to insert variant record", "err", err, "hash", originalHash, "variant", vt)
			}
		}(variant)
	}

	wg.Wait()
}

func (a *goBlog) callImgproxy(sourceURL string, variant *variantType, w io.Writer) error {
	imgproxyURL := strings.TrimRight(a.cfg.MediaOptimization.ImgproxyURL, "/")
	u := fmt.Sprintf("%s/fit/w:%d/h:0/f:%s/plain/%s", imgproxyURL, variant.Width, variant.Format, sourceURL)

	err := requests.URL(u).
		Client(a.httpClient).
		ToWriter(w).
		Fetch(context.Background())
	if err != nil {
		return fmt.Errorf("imgproxy request failed: %w", err)
	}

	return nil
}

func (a *goBlog) deleteOptimizedMediaFile(filename string) error {
	if !a.mediaOptimizationEnabled() {
		return a.deleteMediaFileRaw(filename)
	}

	filename = filepath.Base(filename)
	if !isValidMediaFilename(filename) {
		return fmt.Errorf("invalid filename: %s", filename)
	}

	ext := path.Ext(filename)
	hash := strings.TrimSuffix(filename, ext)

	// Check if this hash is an optimized variant
	if variants, err := a.db.mediaOptimizedByOptimized(hash); err == nil && len(variants) > 0 {
		if derr := a.db.mediaOptimizedDeleteByOptimized(hash); derr != nil {
			return derr
		}
		return a.deleteMediaFileRaw(filename)
	}

	// Treat as original hash: delete all variants + DB records + source file
	variants, err := a.db.mediaOptimizedByOriginal(hash)
	if err != nil {
		return err
	}

	for _, v := range variants {
		f, _ := parseVariantType(v.VariantType)
		variantFilename := v.OptimizedHash + "." + f
		if derr := a.deleteMediaFileRaw(variantFilename); derr != nil {
			a.error("Failed to delete optimized media file", "err", derr, "file", variantFilename)
		}
	}

	if derr := a.db.mediaOptimizedDeleteByOriginal(hash); derr != nil {
		return derr
	}

	return a.deleteMediaFileRaw(filename)
}

func parseVariantType(vt string) (format string, width int) {
	parts := strings.SplitN(vt, "_", 2)
	if len(parts) >= 1 {
		format = parts[0]
	}
	if len(parts) >= 2 {
		w, _ := strconv.Atoi(parts[1])
		width = w
	}
	return
}

func groupMediaVariants(variants []*mediaOptimizedRow) *mediaVariantGroup {
	byFormat := lo.GroupBy(variants, func(v *mediaOptimizedRow) string {
		f, _ := parseVariantType(v.VariantType)
		return f
	})

	sortedFormats := lo.Keys(byFormat)
	sort.SliceStable(sortedFormats, func(i, j int) bool {
		return imageFormatPriority[sortedFormats[i]] < imageFormatPriority[sortedFormats[j]]
	})

	fallbackFormat := sortedFormats[len(sortedFormats)-1]
	fallbackRow := lo.MaxBy(byFormat[fallbackFormat], func(a, b *mediaOptimizedRow) bool {
		return a.Width > b.Width
	})

	return &mediaVariantGroup{
		byFormat:       byFormat,
		sortedFormats:  sortedFormats,
		fallbackFormat: fallbackFormat,
		fallbackRow:    fallbackRow,
	}
}

func (g *mediaVariantGroup) variantWidth(v *mediaOptimizedRow) int {
	if v.Width > 0 {
		return v.Width
	}
	_, cw := parseVariantType(v.VariantType)
	if cw == 0 {
		return 0
	}
	for _, fb := range g.byFormat[g.fallbackFormat] {
		if _, fw := parseVariantType(fb.VariantType); fw == cw {
			return fb.Width
		}
	}
	return 0
}

func (a *goBlog) mediaFallbackURL(originalURL string) string {
	originalHash := a.extractMediaHashFromURL(originalURL)
	if originalHash == "" || !a.mediaOptimizationEnabled() {
		return originalURL
	}

	variants, err := a.db.mediaOptimizedByOriginal(originalHash)
	if err != nil || len(variants) == 0 {
		return originalURL
	}

	g := groupMediaVariants(variants)
	if variantURL := a.mediaFileLocation(g.fallbackRow.OptimizedHash + "." + g.fallbackFormat); variantURL != "" {
		return variantURL
	}
	return originalURL
}

func (a *goBlog) writePictureElement(hb *htmlbuilder.HTMLBuilder, originalURL, alt, title, class, postPath string, simpleImages bool) {
	for _, p := range a.getPlugins(pluginUIImgAttributesType) {
		alt, title = p.(plugintypes.UIImgAttributes).ImgAttributes(originalURL, postPath, alt, title)
	}
	originalHash := a.extractMediaHashFromURL(originalURL)
	if originalHash == "" || !a.mediaOptimizationEnabled() {
		hb.WriteElementOpen("a", "href", originalURL)
		a.writeImgElement(hb, originalURL, alt, title, class, 0, 0, "")
		hb.WriteElementClose("a")
		return
	}

	variants, err := a.db.mediaOptimizedByOriginal(originalHash)
	if err != nil || len(variants) == 0 {
		hb.WriteElementOpen("a", "href", originalURL)
		a.writeImgElement(hb, originalURL, alt, title, class, 0, 0, "")
		hb.WriteElementClose("a")
		return
	}

	g := groupMediaVariants(variants)
	fallbackURL := a.mediaFileLocation(g.fallbackRow.OptimizedHash + "." + g.fallbackFormat)

	if simpleImages {
		hb.WriteElementOpen("a", "href", fallbackURL)
		a.writeImgElement(hb, fallbackURL, alt, title, class, g.fallbackRow.Width, g.fallbackRow.Height, "")
		hb.WriteElementClose("a")
		return
	}

	hb.WriteElementOpen("a", "href", fallbackURL)
	hb.WriteElementOpen("picture")

	for _, f := range g.sortedFormats {
		group := g.byFormat[f]
		sort.SliceStable(group, func(i, j int) bool {
			return group[i].Width > group[j].Width
		})

		srcsetParts := lo.Map(group, func(v *mediaOptimizedRow, _ int) string {
			return fmt.Sprintf("%s %dw", a.mediaFileLocation(v.OptimizedHash+"."+f), g.variantWidth(v))
		})
		hb.WriteElement("source", "type", "image/"+f, "srcset", strings.Join(srcsetParts, ", "), "sizes", a.mediaOptimizationSizes)
	}

	fallbackGroup := g.byFormat[g.fallbackFormat]
	fbSrcsetParts := lo.Map(fallbackGroup, func(v *mediaOptimizedRow, _ int) string {
		return fmt.Sprintf("%s %dw", a.mediaFileLocation(v.OptimizedHash+"."+g.fallbackFormat), v.Width)
	})

	a.writeImgElement(hb, fallbackURL, alt, title, class, g.fallbackRow.Width, g.fallbackRow.Height, strings.Join(fbSrcsetParts, ", "))
	hb.WriteElementClose("picture")
	hb.WriteElementClose("a")
}

func (a *goBlog) writeImgElement(hb *htmlbuilder.HTMLBuilder, src, alt, title, class string, width, height int, srcset string) {
	imgAttrs := []any{"src", src, "alt", alt, "loading", "lazy", "class", class}
	if title != "" {
		imgAttrs = append(imgAttrs, "title", title)
	}
	if srcset != "" {
		imgAttrs = append(imgAttrs, "srcset", srcset, "sizes", a.mediaOptimizationSizes)
	}
	if width > 0 && height > 0 {
		imgAttrs = append(imgAttrs, "width", fmt.Sprintf("%d", width), "height", fmt.Sprintf("%d", height))
	}
	hb.WriteElement("img", imgAttrs...)
}
