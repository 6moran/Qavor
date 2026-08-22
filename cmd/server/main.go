package main

import (
	"Qavor/internal/app"
	"fmt"
)

// 构建信息：由 Makefile / scripts/build.sh 通过 -ldflags -X 注入。
// 留空表示未注入（例如直接 `go run`）。
var (
	Version   string
	BuildTime string
	GoVersion string
)

func main() {
	// 创建应用实例
	application := app.NewApp()

	// 初始化应用
	if err := application.Initialize(); err != nil {
		panic(fmt.Sprintf("应用初始化失败: %v", err))
	}

	// 运行应用
	application.Run()
}
