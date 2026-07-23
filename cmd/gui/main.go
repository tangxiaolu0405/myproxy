package main

import (
	"fmt"
	"log"
	"path/filepath"

	"myproxy.com/p/internal/database"
	"myproxy.com/p/internal/ui"
	"myproxy.com/p/internal/utils"
)

func main() {
	if err := initDatabase(); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer database.CloseDB()

	appState := ui.NewAppState(version)
	if err := appState.Startup(); err != nil {
		log.Fatalf("应用启动失败: %v", err)
	}
	appState.Run()
}

func initDatabase() error {
	dbPath, err := utils.DBPath()
	if err != nil {
		return fmt.Errorf("解析数据目录失败: %w", err)
	}
	log.Printf("数据目录: %s", filepath.Dir(dbPath))

	if err := database.InitDB(dbPath); err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}
	if err := database.InitDefaultConfig(); err != nil {
		log.Printf("初始化默认配置失败: %v", err)
	}

	return nil
}
