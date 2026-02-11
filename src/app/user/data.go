package user

import "time"

type User struct {
	Id        int
	Username  string
	Email     string
	FullName  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Member struct {
	Id            int
	UserId        int
	Name          string
	MonthlyIncome int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
