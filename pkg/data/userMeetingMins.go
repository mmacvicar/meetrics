package data

import (
	"time"

	log "github.com/sirupsen/logrus"
)

// Data

func (mgr *manager) CreateUserMeetingMins(date time.Time, user User, meetingMins map[string]uint) {
	ormObj := UserMeetingMins{
		Date:        date,
		UserID:      user.ID,
		Mins0:       meetingMins["mins0"],
		Mins1:       meetingMins["mins1"],
		Mins2:       meetingMins["mins2"],
		Mins3:       meetingMins["mins3"],
		Mins4:       meetingMins["mins4"],
		Mins5:       meetingMins["mins5"],
		Mins6:       meetingMins["mins6"],
		Mins7:       meetingMins["mins7"],
		Mins8:       meetingMins["mins8"],
		Mins9:       meetingMins["mins9"],
		Mins10Plus:  meetingMins["mins10plus"],
		Count0:      meetingMins["count0"],
		Count1:      meetingMins["count1"],
		Count2:      meetingMins["count2"],
		Count3:      meetingMins["count3"],
		Count4:      meetingMins["count4"],
		Count5:      meetingMins["count5"],
		Count6:      meetingMins["count6"],
		Count7:      meetingMins["count7"],
		Count8:      meetingMins["count8"],
		Count9:      meetingMins["count9"],
		Count10Plus: meetingMins["count10plus"],
	}

	errors := mgr.db.Create(&ormObj).GetErrors()
	if len(errors) > 0 {
		log.Error("Error adding user meeting mins: %v", errors[0])
	}
}

func (mgr *manager) ClearUserMeetingMins() {
	errors := mgr.db.Delete(UserMeetingMins{}).GetErrors()
	if len(errors) > 0 {
		log.Fatalf("Error clearing user meeting mins: %v", errors[0])
	}
}

type UserMeetingMins struct {
	Date        time.Time `gorm:"primary_key" sql:"type:date"`
	UserID      uint      `gorm:"primary_key" sql:"type:int unsigned"`
	User        User
	Mins0       uint
	Mins1       uint
	Mins2       uint
	Mins3       uint
	Mins4       uint
	Mins5       uint
	Mins6       uint
	Mins7       uint
	Mins8       uint
	Mins9       uint
	Mins10Plus  uint
	Count0      uint
	Count1      uint
	Count2      uint
	Count3      uint
	Count4      uint
	Count5      uint
	Count6      uint
	Count7      uint
	Count8      uint
	Count9      uint
	Count10Plus uint
	CreatedAt   time.Time
}
