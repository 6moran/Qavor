package repository

import "context"

// SystemSettingRepository 管理系统键值设置。
type SystemSettingRepository interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Upsert(ctx context.Context, key, value string) error
}
