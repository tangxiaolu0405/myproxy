package main

// version 由构建时写入或 -ldflags "-X main.version=..." 注入；本地直接运行默认为 dev。
// CI 发版会覆盖本文件内容，请勿依赖仓库中的字面量作为正式版本号。
var version = "dev"
