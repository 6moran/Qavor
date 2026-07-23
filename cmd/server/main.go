package main

import (
	"fmt"

	"Qavor/internal/app"
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
