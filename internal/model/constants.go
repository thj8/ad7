package model

// Boolean constants 用于数据库中 TINYINT(1) 类型的布尔值字段。
// MySQL 不支持原生 bool 类型，使用 0/1 表示 false/true。
const (
	// False 表示布尔假值（0）
	False = 0
	// True 表示布尔真值（1）
	True = 1
)
