package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// MonochromeTheme 提供黑白两套主题（Dark/Light），简化配色但保持可读性
type MonochromeTheme struct {
	variant fyne.ThemeVariant
}

// NewMonochromeTheme 创建黑白主题，variant 支持 fyne.ThemeVariantDark / fyne.ThemeVariantLight
func NewMonochromeTheme(variant fyne.ThemeVariant) fyne.Theme {
	return &MonochromeTheme{variant: variant}
}

// Color 返回自定义颜色，未覆盖的颜色使用默认主题
func (t *MonochromeTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	// 以传入 variant 优先，其次使用主题自身 variant
	if variant == fyne.ThemeVariant(0) {
		variant = t.variant
	}

	switch variant {
	case theme.VariantDark:
		switch name {
		case theme.ColorNameBackground, theme.ColorNameInputBackground:
			return color.NRGBA{R: 18, G: 18, B: 18, A: 255} // 近黑背景
		case theme.ColorNameForeground, theme.ColorNameButton, theme.ColorNamePrimary:
			return color.NRGBA{R: 245, G: 245, B: 245, A: 255} // 浅色前景
		case theme.ColorNameFocus, theme.ColorNameHover:
			return color.NRGBA{R: 255, G: 255, B: 255, A: 64} // 半透明高亮
		}
	case theme.VariantLight:
		switch name {
		case theme.ColorNameBackground, theme.ColorNameInputBackground:
			return color.NRGBA{R: 255, G: 255, B: 255, A: 255} // 白色背景
		case theme.ColorNameForeground, theme.ColorNameButton, theme.ColorNamePrimary:
			return color.NRGBA{R: 25, G: 25, B: 25, A: 255} // 深色前景
		case theme.ColorNameFocus, theme.ColorNameHover:
			return color.NRGBA{R: 0, G: 0, B: 0, A: 64} // 半透明高亮
		}
	}

	// 其他颜色使用默认主题
	return theme.DefaultTheme().Color(name, variant)
}

// Icon 使用默认主题图标
func (t *MonochromeTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

// Font 使用默认字体，保持兼容
func (t *MonochromeTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

// Size 使用默认尺寸
func (t *MonochromeTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
