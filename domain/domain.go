package domain

type User interface {
	AddData(n int64)
	IsOverDataLimit(limit int64) bool
	TryIncrementConnections(max int64) bool
	DecrementConnections()
}

type Repository interface {
	GetOrCreateUser(username string) User
	GetCredentials() map[string]string
}
