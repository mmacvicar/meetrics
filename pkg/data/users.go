package data

import (
	"log"
	"time"
)

type User struct {
	ID         uint
	Email      string `gorm:"not null;unique"`
	Name       string
	Department string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (mgr *manager) AddAllUsers(users []*User) {
	for _, user := range users {
		errors := mgr.db.Create(user).GetErrors()
		if len(errors) > 0 {
			log.Fatalf("Error adding user: %v", errors[0])
		}
	}
}

func (mgr *manager) GetAllUsers() []User {
	var users []User
	mgr.db.Find(&users)
	return users
}

func (mgr *manager) GetUserByEmail(email string) User {
	var user User
	mgr.db.First(&user, "email = ?", email)
	return user
}

func (mgr *manager) GetUsersWithMeetingsMinsbyDate(date time.Time) []User {
	var users []User
	if err := mgr.db.Select("users.*").Joins("INNER JOIN user_meeting_mins on user_meeting_mins.user_id=users.id").Where("user_meeting_mins.date=?", date.Format("2006-01-02")).Find(&users).Error; err != nil {
		log.Fatal(err)
	}
	return users
}

func (mgr *manager) GetUserById(id int) User {
	var user User
	mgr.db.First(&user, "id = ?", id)
	return user
}
