package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/carlmjohnson/requests"
	"github.com/kovidgoyal/imaging"
	"github.com/samber/lo"
	"go.goblog.app/app/pkgs/phash"
)

const migrationMaxExifLen = 128 * 1024

type migrationCache struct {
	Entries map[string]*migrationEntry `json:"entries"`
}

type migrationEntry struct {
	DHash    string `json:"dhash"`
	DHashRaw string `json:"dhashRaw"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Bytes    int64  `json:"bytes"`
	ModTime  string `json:"modTime"`
	HasExif  bool   `json:"hasExif"`
}

type migrationFile struct {
	Name     string
	DHash    uint64
	DHashRaw uint64
	Width    int
	Height   int
	Bytes    int64
	ModTime  time.Time
	HasExif  bool
}

type migrationGroup struct {
	ID       int
	Files    []*migrationFile
	Original *migrationFile
	Others   []*migrationFile
}

type migrationConfig struct {
	yes          bool
	dryRun       bool
	threshold    int
	discoverOnly bool
	limit        int
	preview      bool
}

func (a *goBlog) mediaMigrate(mc *migrationConfig) {
	if !a.mediaStorageEnabled() {
		fmt.Println("No media storage configured")
		return
	}

	if a.mediaOptimizationImgproxyConfigured() {
		if err := a.checkImgproxyReachable(); err != nil {
			fmt.Printf("Warning: imgproxy not reachable: %v\n", err)
			fmt.Println("Continuing anyway — optimization will be skipped for unreachable imgproxy.")
		}
	}

	fmt.Println("Phase 0: Loading state cache...")
	cache := a.migrationLoadCache()

	fmt.Println("Phase 1: Discovering files...")
	originals, variants, err := a.db.mediaOptimizedHashSets()
	if err != nil {
		a.error("Media migrate: failed to get optimized hash sets", "err", err)
		return
	}

	files := a.migrationDiscoverFiles(cache, originals, variants)

	groups := a.migrationGroupFiles(files, mc.threshold)
	if len(groups) == 0 {
		fmt.Println("No migration groups found.")
		return
	}

	if mc.discoverOnly {
		fmt.Printf("\nMigration report: %d groups found\n\n", len(groups))
		for _, g := range groups {
			a.migrationPrintGroup(g, mc.preview)
		}
		return
	}

	confirmed, quit := a.migrationVerify(groups, mc.yes, mc.preview)
	if quit {
		return
	}
	if len(confirmed) == 0 {
		fmt.Println("No groups confirmed for migration.")
		return
	}

	if mc.dryRun {
		fmt.Println("Dry run complete (no changes made).")
		return
	}

	if mc.limit > 0 && len(confirmed) > mc.limit {
		confirmed = confirmed[:mc.limit]
	}

	fmt.Println("Phase 2: Optimizing originals...")
	for i, g := range confirmed {
		ext := path.Ext(g.Original.Name)
		hash := strings.TrimSuffix(g.Original.Name, path.Ext(g.Original.Name))
		fmt.Printf("Optimizing originals... [%d/%d] %s", i+1, len(confirmed), g.Original.Name)
		a.optimizeMediaFile(hash, ext)
		fmt.Println()
	}

	fmt.Println("Phase 3: Replacing DB references...")
	a.migrationReplace(confirmed)

	fmt.Println("Phase 4: Cleaning up compressed files...")
	var totalBytes int64
	var cleanupFailed int
	for _, g := range confirmed {
		for _, c := range g.Others {
			if err := a.deleteMediaFile(c.Name); err != nil {
				a.error("Media migrate: failed to delete compressed file", "name", c.Name, "err", err)
				cleanupFailed++
				continue
			}
			totalBytes += c.Bytes
			fmt.Printf("Deleted: %s (%dB)\n", c.Name, c.Bytes)
		}
	}
	fmt.Printf("Freed: %dB total\n", totalBytes)
	if cleanupFailed > 0 {
		fmt.Printf("Cleanup completed with %d failures (see logs above)\n", cleanupFailed)
	}
}

func (a *goBlog) migrationLoadCache() *migrationCache {
	c := &migrationCache{Entries: map[string]*migrationEntry{}}
	f, err := os.Open(a.cfg.Db.MigrationCache)
	if err != nil {
		return c
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(c); err != nil {
		return &migrationCache{Entries: map[string]*migrationEntry{}}
	}
	return c
}

func (a *goBlog) migrationSaveCache(c *migrationCache) error {
	dir := filepath.Dir(a.cfg.Db.MigrationCache)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}
	tmpPath := a.cfg.Db.MigrationCache + ".tmp"
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write cache: %w", err)
	}
	if err := os.Rename(tmpPath, a.cfg.Db.MigrationCache); err != nil {
		return fmt.Errorf("rename cache: %w", err)
	}
	return nil
}

func (a *goBlog) migrationDiscoverFiles(cache *migrationCache, originals, variants map[string]bool) []*migrationFile {
	allFiles, err := a.mediaFiles()
	if err != nil {
		a.error("Media migrate: failed to list media files", "err", err)
		return nil
	}

	candidates := lo.Filter(allFiles, func(mf *mediaFile, _ int) bool {
		if !isImageExtension(path.Ext(mf.Name)) {
			return false
		}
		hash := strings.TrimSuffix(mf.Name, path.Ext(mf.Name))
		return !originals[hash] && !variants[hash]
	})

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Time.After(candidates[j].Time)
	})

	total := len(candidates)
	newCount := 0
	cachedCount := 0
	cacheDirty := false

	cachedFilenames := map[string]bool{}
	for name := range cache.Entries {
		cachedFilenames[name] = true
	}

	var result []*migrationFile
	for i, mf := range candidates {
		pct := (i + 1) * 100 / total
		fmt.Printf("\rHashing media files... [%d/%d] %d%%   (%d new, %d from cache)",
			i+1, total, pct, newCount, cachedCount)

		var entry *migrationEntry
		if ce, ok := cache.Entries[mf.Name]; ok && ce.ModTime == mf.Time.UTC().Format(time.RFC3339) {
			entry = ce
			cachedCount++
		} else {
			ne, err := a.migrationHashFile(mf)
			if err != nil {
				a.error("Media migrate: failed to hash file", "name", mf.Name, "err", err)
				continue
			}
			cache.Entries[mf.Name] = ne
			cacheDirty = true
			entry = ne
			newCount++
		}
		delete(cachedFilenames, mf.Name)

		if entry == nil {
			continue
		}

		dh, err := strconv.ParseUint(entry.DHash, 16, 64)
		if err != nil {
			continue
		}
		dhRaw, _ := strconv.ParseUint(entry.DHashRaw, 16, 64)

		mt, _ := time.Parse(time.RFC3339, entry.ModTime)
		if mt.IsZero() {
			mt = mf.Time
		}

		result = append(result, &migrationFile{
			Name:     mf.Name,
			DHash:    dh,
			DHashRaw: dhRaw,
			Width:    entry.Width,
			Height:   entry.Height,
			Bytes:    entry.Bytes,
			ModTime:  mt,
			HasExif:  entry.HasExif,
		})
	}
	fmt.Println()

	for name := range cachedFilenames {
		delete(cache.Entries, name)
		cacheDirty = true
	}

	if cacheDirty {
		if err := a.migrationSaveCache(cache); err != nil {
			a.error("Media migrate: failed to save cache", "err", err)
		}
	}

	return result
}

func (a *goBlog) migrationHashFile(mf *mediaFile) (*migrationEntry, error) {
	rc, err := a.mediaStorage.open(mf.Name)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer rc.Close()

	tmp, err := os.CreateTemp("", "goblog-migrate-*")
	if err != nil {
		return nil, fmt.Errorf("create temp: %w", err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	written, err := io.Copy(tmp, rc)
	if err != nil {
		return nil, fmt.Errorf("copy: %w", err)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	format := strings.ToLower(strings.TrimPrefix(path.Ext(mf.Name), "."))

	var exifBuf [migrationMaxExifLen]byte
	n, _ := tmp.Read(exifBuf[:])
	hasExif := migrationHasExif(format, exifBuf[:n])

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	img, err := imaging.Decode(tmp, imaging.AutoOrientation(true))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	bounds := img.Bounds()
	dhash := fmt.Sprintf("%x", phash.Hash(img))
	img = nil

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	imgRaw, err := imaging.Decode(tmp, imaging.AutoOrientation(false))
	if err != nil {
		return nil, fmt.Errorf("decode raw: %w", err)
	}
	dhashRaw := fmt.Sprintf("%x", phash.Hash(imgRaw))

	return &migrationEntry{
		DHash:    dhash,
		DHashRaw: dhashRaw,
		Width:    bounds.Dx(),
		Height:   bounds.Dy(),
		Bytes:    written,
		ModTime:  mf.Time.UTC().Format(time.RFC3339),
		HasExif:  hasExif,
	}, nil
}

func (a *goBlog) migrationGroupFiles(files []*migrationFile, threshold int) []*migrationGroup {
	type pair struct{ i, j int }
	var pairs []pair

	for i := range files {
		for j := i + 1; j < len(files); j++ {
			if !migrationSimilarAspectRatio(files[i], files[j]) {
				continue
			}
			if migrationMinDistance(files[i], files[j]) <= threshold {
				pairs = append(pairs, pair{i, j})
			}
		}
	}

	if len(pairs) == 0 {
		return nil
	}

	parent := make([]int, len(files))
	for i := range parent {
		parent[i] = i
	}
	var find func(x int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		parent[find(a)] = find(b)
	}

	for _, p := range pairs {
		union(p.i, p.j)
	}

	groupMap := map[int][]*migrationFile{}
	for i, f := range files {
		root := find(i)
		groupMap[root] = append(groupMap[root], f)
	}

	var groups []*migrationGroup
	groupID := 0
	for _, gfiles := range groupMap {
		if len(gfiles) < 2 {
			continue
		}
		original, others := migrationIdentifyOriginal(gfiles)
		if original == nil {
			continue
		}
		groupID++
		sort.Slice(gfiles, func(i, j int) bool {
			return gfiles[i].ModTime.After(gfiles[j].ModTime)
		})
		groups = append(groups, &migrationGroup{
			ID:       groupID,
			Files:    gfiles,
			Original: original,
			Others:   others,
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Original.ModTime.Equal(groups[j].Original.ModTime) {
			return groups[i].ID < groups[j].ID
		}
		return groups[i].Original.ModTime.After(groups[j].Original.ModTime)
	})

	return groups
}

func (a *goBlog) migrationVerify(groups []*migrationGroup, yesAll, preview bool) ([]*migrationGroup, bool) {
	if yesAll {
		return groups, false
	}

	var confirmed []*migrationGroup
	for _, g := range groups {
		a.migrationPrintGroup(g, preview)
		fmt.Printf("Process group %d? [Y]es / [n]o / [s]kip all / [q]uit: ", g.ID)

		var answer string
		_, _ = fmt.Scanln(&answer)
		answer = strings.ToLower(strings.TrimSpace(answer))

		if answer == "s" {
			return confirmed, false
		}
		if answer == "q" {
			return nil, true
		}
		if answer == "y" || answer == "yes" || answer == "" {
			confirmed = append(confirmed, g)
		} else {
			fmt.Println("Skipped.")
		}
		fmt.Println()
	}
	return confirmed, false
}

func (a *goBlog) migrationPrintGroup(g *migrationGroup, preview bool) {
	fmt.Printf("Group %d: %d files\n", g.ID, len(g.Files))
	origURL := a.getFullAddress(a.mediaFileLocation(g.Original.Name))
	fmt.Printf("  Original:  %s (%dx%d, %dB, EXIF: %v)\n",
		origURL, g.Original.Width, g.Original.Height, g.Original.Bytes, g.Original.HasExif)
	if preview {
		a.migrationPreviewFile(origURL)
	}
	// Show posts referencing original
	origHash := strings.TrimSuffix(g.Original.Name, path.Ext(g.Original.Name))
	origPosts, err := a.migrationPostsUsingFile(origHash)
	if err != nil {
		fmt.Printf("    Posts: error looking up posts: %v\n", err)
	} else if len(origPosts) > 0 {
		urls := lo.Map(origPosts, func(p string, _ int) string { return a.getFullAddress(p) })
		fmt.Printf("    Posts: %s\n", strings.Join(urls, ", "))
	}
	for _, c := range g.Others {
		d := migrationMinDistance(g.Original, c)
		compURL := a.getFullAddress(a.mediaFileLocation(c.Name))
		fmt.Printf("  Compressed: %s (%dx%d, %dB, EXIF: %v, distance: %d)\n",
			compURL, c.Width, c.Height, c.Bytes, c.HasExif, d)
		if preview {
			a.migrationPreviewFile(compURL)
		}
		// Show affected posts
		hash := strings.TrimSuffix(c.Name, path.Ext(c.Name))
		posts, err := a.migrationPostsUsingFile(hash)
		if err != nil {
			fmt.Printf("    Posts: error looking up posts: %v\n", err)
		} else if len(posts) > 0 {
			urls := lo.Map(posts, func(p string, _ int) string { return a.getFullAddress(p) })
			fmt.Printf("    Posts: %s\n", strings.Join(urls, ", "))
		}
	}
}

func (a *goBlog) migrationPreviewFile(fileURL string) {
	if _, err := exec.LookPath("timg"); err != nil {
		return
	}
	pr, pw := io.Pipe()
	go func() {
		err := requests.URL(fileURL).Client(a.httpClient).ToWriter(pw).Fetch(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Preview fetch error: %v\n", err)
		}
		pw.CloseWithError(err)
	}()
	cmd := exec.CommandContext(context.Background(), "timg", "-g", "40x20", "-") //nolint:gosec
	cmd.Stdin = pr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
	pr.Close()
}

func migrationMinDistance(a, b *migrationFile) int {
	return min(phash.Distance(a.DHash, b.DHash), phash.Distance(a.DHashRaw, b.DHashRaw))
}

func (a *goBlog) migrationReplace(groups []*migrationGroup) {
	// Build map of compressed -> original references with full URLs and paths
	type replacement struct {
		compFullURL string
		compRelPath string
		compName    string
		origFullURL string
		origRelPath string
		origName    string
	}
	var replacements []replacement
	for _, g := range groups {
		origFullURL := a.getFullAddress(a.mediaFileLocation(g.Original.Name))
		origRelPath := a.mediaFileLocation(g.Original.Name)
		for _, c := range g.Others {
			compFullURL := a.getFullAddress(a.mediaFileLocation(c.Name))
			compRelPath := a.mediaFileLocation(c.Name)
			replacements = append(replacements, replacement{
				compFullURL: compFullURL,
				compRelPath: compRelPath,
				compName:    c.Name,
				origFullURL: origFullURL,
				origRelPath: origRelPath,
				origName:    g.Original.Name,
			})
		}
	}

	var failed int
	for _, r := range replacements {
		compHash := strings.TrimSuffix(r.compName, path.Ext(r.compName))
		paths, err := a.migrationPostsUsingFile(compHash)
		if err != nil {
			a.error("Media migrate: failed to find posts using file", "hash", compHash, "err", err)
			failed++
			continue
		}

		for _, ppath := range paths {
			p, err := a.getPost(ppath)
			if err != nil {
				a.error("Media migrate: failed to get post", "path", ppath, "err", err)
				failed++
				continue
			}

			contentRepl := 0
			paramsRepl := 0

			// Replace in order: full URL first (longest match), then relative path, then bare filename
			newContent := p.Content
			newContent = strings.ReplaceAll(newContent, r.compFullURL, r.origFullURL)
			newContent = strings.ReplaceAll(newContent, r.compRelPath, r.origRelPath)
			newContent = strings.ReplaceAll(newContent, r.compName, r.origName)
			if newContent != p.Content {
				contentRepl = 1
			}

			newParams := map[string][]string{}
			for k, values := range p.Parameters {
				newValues := make([]string, len(values))
				for i, v := range values {
					nv := strings.ReplaceAll(v, r.compFullURL, r.origFullURL)
					nv = strings.ReplaceAll(nv, r.compRelPath, r.origRelPath)
					nv = strings.ReplaceAll(nv, r.compName, r.origName)
					newValues[i] = nv
					if nv != v {
						paramsRepl++
					}
				}
				newParams[k] = newValues
			}

			if contentRepl > 0 || paramsRepl > 0 {
				p.Content = newContent
				p.Parameters = newParams
				if err := a.db.savePost(p, &postCreationOptions{isNew: false, noUpdated: true, oldPath: ppath}); err != nil {
					a.error("Media migrate: failed to update post", "path", ppath, "err", err)
					failed++
					continue
				}
				fmt.Printf("Post %s: replaced references (content: %d, params: %d)\n",
					ppath, contentRepl, paramsRepl)
			}
		}
	}
	if failed > 0 {
		fmt.Printf("Replacement completed with %d failures (see logs above)\n", failed)
	}
}

func (a *goBlog) migrationPostsUsingFile(hash string) ([]string, error) {
	query := `
		select distinct p.path
		from posts p
		where p.path in (
			select ps.path from posts_fts ps where ps.content MATCH '"' || ? || '"'
			union all
			select pp.path from post_parameters pp where pp.value LIKE '%' || ? || '%'
		)`
	rows, err := a.db.Query(query, hash, hash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var ppath string
		if err := rows.Scan(&ppath); err != nil {
			return nil, err
		}
		result = append(result, ppath)
	}
	return result, rows.Err()
}

func migrationSimilarAspectRatio(a, b *migrationFile) bool {
	if a.Height == 0 || b.Height == 0 || a.Width == 0 || b.Width == 0 {
		return true
	}
	ra := float64(a.Width) / float64(a.Height)
	rb := float64(b.Width) / float64(b.Height)
	return max(ra, rb)/min(ra, rb)-1 <= 0.15
}

func migrationIdentifyOriginal(files []*migrationFile) (*migrationFile, []*migrationFile) {
	if len(files) < 2 {
		return nil, nil
	}

	others := func(original *migrationFile) []*migrationFile {
		return lo.Filter(files, func(f *migrationFile, _ int) bool { return f != original })
	}

	for _, f := range files {
		if f.HasExif {
			return f, others(f)
		}
	}

	maxDim := lo.MaxBy(files, func(a, b *migrationFile) bool {
		return a.Width*a.Height > b.Width*b.Height
	})
	for _, f := range files {
		if f.Width != maxDim.Width || f.Height != maxDim.Height {
			return maxDim, others(maxDim)
		}
	}

	maxBytes := lo.MaxBy(files, func(a, b *migrationFile) bool {
		return a.Bytes > b.Bytes
	})
	return maxBytes, others(maxBytes)
}

func migrationHasExif(format string, data []byte) bool {
	if len(data) > migrationMaxExifLen {
		data = data[:migrationMaxExifLen]
	}

	switch format {
	case "jpg", "jpeg":
		return bytes.Contains(data, []byte("Exif\x00\x00"))
	case "png":
		return bytes.Contains(data, []byte("eXIf"))
	default:
		return false
	}
}
