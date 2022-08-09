package demoexec

import (
	"fmt"

	"go.goblog.app/app/pkgs/plugintypes"
)

func GetPlugin() plugintypes.Exec {
	return &plugin{}
}

type plugin struct {
	app plugintypes.App
}

func (p *plugin) SetApp(app plugintypes.App) {
	p.app = app
}

func (*plugin) SetConfig(_ map[string]any) {
	// Ignore
}

func (p *plugin) Exec() {
	fmt.Println("Hello World from the demo plugin!")

	row, _ := p.app.GetDatabase().QueryRow("select count (*) from posts")
	var count int
	if err := row.Scan(&count); err != nil {
		fmt.Println(fmt.Errorf("failed to count posts: %w", err))
		return
	}

	fmt.Printf("Number of posts in database: %d", count)
	fmt.Println()
}
