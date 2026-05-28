package main

func (db *database) mediaOptimizedInsert(row *mediaOptimizedRow) error {
	_, err := db.Exec("insert or replace into media_optimized (original_hash, variant_type, optimized_hash, width, height) values (?, ?, ?, ?, ?)",
		row.OriginalHash, row.VariantType, row.OptimizedHash, row.Width, row.Height)
	return err
}

func (db *database) mediaOptimizedByOriginal(originalHash string) ([]*mediaOptimizedRow, error) {
	rows, err := db.Query("select original_hash, variant_type, optimized_hash, width, height from media_optimized where original_hash = ?", originalHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*mediaOptimizedRow
	for rows.Next() {
		r := &mediaOptimizedRow{}
		if err := rows.Scan(&r.OriginalHash, &r.VariantType, &r.OptimizedHash, &r.Width, &r.Height); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (db *database) mediaOptimizedByOptimized(optimizedHash string) ([]*mediaOptimizedRow, error) {
	rows, err := db.Query("select original_hash, variant_type, optimized_hash, width, height from media_optimized where optimized_hash = ?", optimizedHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*mediaOptimizedRow
	for rows.Next() {
		r := &mediaOptimizedRow{}
		if err := rows.Scan(&r.OriginalHash, &r.VariantType, &r.OptimizedHash, &r.Width, &r.Height); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (db *database) mediaOptimizedDeleteByOptimized(optimizedHash string) error {
	_, err := db.Exec("delete from media_optimized where optimized_hash = ?", optimizedHash)
	return err
}

func (db *database) mediaOptimizedDeleteByOriginal(originalHash string) error {
	_, err := db.Exec("delete from media_optimized where original_hash = ?", originalHash)
	return err
}

func (db *database) mediaOptimizedHashSets() (originals map[string]bool, variants map[string]bool, err error) {
	rows, err := db.Query("select original_hash, optimized_hash from media_optimized")
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	originals = map[string]bool{}
	variants = map[string]bool{}
	for rows.Next() {
		var originalHash, optimizedHash string
		if err := rows.Scan(&originalHash, &optimizedHash); err != nil {
			return nil, nil, err
		}
		originals[originalHash] = true
		variants[optimizedHash] = true
	}
	return originals, variants, rows.Err()
}
