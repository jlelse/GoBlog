package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/samber/lo"
)

type mediaFormatCheckResult struct {
	Hash            string
	Extension       string
	MissingVariants []string
}

func (a *goBlog) checkMediaFormats(hasVariantsOnly bool) ([]*mediaFormatCheckResult, error) {
	if !a.mediaOptimizationEnabled() {
		return nil, fmt.Errorf("media optimization is not enabled")
	}

	_, variants, err := a.db.mediaOptimizedHashSets()
	if err != nil {
		return nil, fmt.Errorf("failed to get optimized hash sets: %w", err)
	}

	files, err := a.mediaFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to list media files: %w", err)
	}

	var results []*mediaFormatCheckResult

	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f.Name))
		if !isImageExtension(ext) {
			continue
		}

		hash := strings.TrimSuffix(f.Name, ext)
		if variants[hash] {
			continue
		}

		sourceFormat := strings.TrimPrefix(ext, ".")

		expected := lo.Map(
			lo.Filter(a.mediaOptimizationVariants, func(v variantType, _ int) bool {
				return variantTypeAllowedForSource(sourceFormat, v.Format)
			}),
			func(v variantType, _ int) string {
				return fmt.Sprintf("%s_%d", v.Format, v.Width)
			},
		)

		existingVariants, _ := a.db.mediaOptimizedByOriginal(hash)
		existing := lo.Map(existingVariants, func(v *mediaOptimizedRow, _ int) string {
			return v.VariantType
		})

		if hasVariantsOnly && len(existing) == 0 {
			continue
		}

		missing := lo.Without(expected, existing...)

		if len(missing) > 0 {
			sort.Strings(missing)
			results = append(results, &mediaFormatCheckResult{
				Hash:            hash,
				Extension:       ext,
				MissingVariants: missing,
			})
		}
	}

	return results, nil
}
