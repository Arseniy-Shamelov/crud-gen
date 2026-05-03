package models

import "time"

// User is an example model used to demonstrate crud-gen.
type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Age       int       `json:"age"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Password  string    `json:"-"` // excluded from CRUD generation
	Bio       *string   `json:"bio"`
}
