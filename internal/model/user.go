package model

import "github.com/gotd/td/tg"

type User struct {
	ID        int64
	Username  string
	Phone     string
	FirstName string
	LastName  string
	Premium   bool
	WasOnline int
}

func NewUser(tgUser *tg.User) *User {
	user := &User{
		ID:        tgUser.ID,
		Username:  tgUser.Username,
		Phone:     tgUser.Phone,
		FirstName: tgUser.FirstName,
		LastName:  tgUser.LastName,
		Premium:   tgUser.Premium,
	}

	if status, ok := tgUser.Status.(*tg.UserStatusOffline); ok {
		user.WasOnline = status.WasOnline
	} else {
		user.WasOnline = 0
	}

	return user
}
