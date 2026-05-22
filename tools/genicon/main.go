// genicon 在仓库根目录生成 Icon.png，供 fyne / fyne-cross 打包使用。
// 优先使用 assets 中的项目图标，缺失时回退到纯色占位图。
package main

import (
	"bytes"
	"image/png"
	"log"
	"os"
)

func main() {
	src := "assets/app-icon-v3-dark.png"
	data, err := os.ReadFile(src)
	if err != nil {
		log.Fatalf("读取项目图标失败 (%s): %v", src, err)
	}
	// 验证为合法 PNG
	if _, err := png.DecodeConfig(bytes.NewReader(data)); err != nil {
		log.Fatalf("图标不是合法 PNG (%s): %v", src, err)
	}
	if err := os.WriteFile("Icon.png", data, 0644); err != nil {
		log.Fatalf("写入 Icon.png 失败: %v", err)
	}
	log.Printf("已使用项目图标 %s 生成 Icon.png", src)
}
