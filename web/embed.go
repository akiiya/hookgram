package web

import "embed"

// Dist 内嵌前端构建产物，构建脚本会先生成 web/dist。
//
//go:embed dist
var Dist embed.FS
