package main

func (a *goBlog) cleanup() {
	if a.db != nil {
		_ = a.db.close()
	}
}
