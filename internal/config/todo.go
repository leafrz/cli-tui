package config

// TodoItem ist ein einzelner Eintrag im Todo-Modul.
type TodoItem struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}
