package domain

type User interface {
	AddData(n int64)
	IsOverDataLimit(limit int64) bool
	TryIncrementConnections(max int64) bool
	DecrementConnections()
}

type Repository interface {
	GetOrCreateUser(username string) User
	ValidateUser(username, password string) bool
	GetUserLimits(username string) (dataLimit int64, maxConnections int64)
}
