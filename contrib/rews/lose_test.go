package rews

import "github.com/surrealdb/surrealdb.go/pkg/connection"

// loseConnection does to tk what a WebSocket connection does when it loses
// its connection.
func loseConnection(tk *connection.Toolkit) { tk.DrainAllLiveNotifications() }
