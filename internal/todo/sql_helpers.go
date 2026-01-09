package todo

import "database/sql"

func nullableString(value *string) sql.NullString {
	// 将可选字符串转换为 SQL 可空类型
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func nullableBool(value *bool) sql.NullBool {
	// 将可选布尔转换为 SQL 可空类型
	if value == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{Bool: *value, Valid: true}
}
