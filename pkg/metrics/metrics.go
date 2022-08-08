package metrics

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"

	_ "github.com/jinzhu/gorm/dialects/mysql"
	"google.golang.org/api/calendar/v3"

	"math/rand"

	"github.com/chasdevs/meetrics/pkg/apis"
	"github.com/chasdevs/meetrics/pkg/data"
	"github.com/chasdevs/meetrics/pkg/util"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"
)

// Types

type UserEvent struct {
	user  *data.User
	event *calendar.Event
}

//	Template: Mon Jan 2 15:04:05 -0700 MST 2006
const EventDateTimeFormat = "2006-01-02T15:04:05-07:00"

// Private Functions

func CompileMetrics(date time.Time) {
	eventChans := compileMetricsForAllUsersAndGetEventChannels(date)
	eventMap := generateEventMap(eventChans)
	computeEventMetrics(eventMap)
}

func compileMetricsForAllUsersAndGetEventChannels(date time.Time) []<-chan UserEvent {
	ctxLog := log.WithFields(log.Fields{"date": date.Format("2006-01-02")})
	// All Users
	users := data.Mgr.GetAllUsers()

	existingUsersForDate := data.Mgr.GetUsersWithMeetingsMinsbyDate(date)
	existingUsersMap := make(map[uint]bool)
	for i := 0; i < len(existingUsersForDate); i += 1 {
		existingUsersMap[existingUsersForDate[i].ID] = true
	}

	// Limit the number of requests we make at once
	concurrency := 20
	sem := make(chan bool, concurrency)
	eventChans := make([]<-chan UserEvent, len(users))

	for i, user := range users {

		// check if file exist and load it
		cacheOldPath := getCacheFilenameOldPathForUser(date, user)
		cachePath := getCacheFilenamePathForUser(date, user)
		os.Rename(cacheOldPath, cachePath)

		eventChan := make(chan UserEvent, 10000)
		if _, ok := existingUsersMap[user.ID]; ok {
			ctxLog.Info("Skipping ", user.Email, " because it already has data")
			close(eventChan)
		} else {
			ctxLog.Info("Not skipping ", user.Email, " because it does not have data")
			sem <- true // Start reserving space in the semaphore.
			go compileMetricsForUserWithSemaphore(date, user, eventChan, sem)
		}
		eventChans[i] = eventChan
	}

	// Block here until all jobs are finished
	for i := 0; i < cap(sem); i++ {
		sem <- true // Attempt to send to semaphore; will only work when the async jobs have popped from it.
	}
	return eventChans
}

func generateEventMap(eventChans []<-chan UserEvent) map[string]*calendar.Event {

	eventMap := make(map[string]*calendar.Event)
	for userEvent := range merge(eventChans...) {
		event := userEvent.event
		// Store the event in the map
		if _, ok := eventMap[event.Id]; !ok {
			eventMap[event.Id] = event
		}
	}
	return eventMap
}

func compileMetricsForUserWithSemaphore(date time.Time, user data.User, eventChan chan UserEvent, sem chan bool) {
	defer func() {
		<-sem
	}()
	CompileMetricsForUser(date, user, eventChan)
}

func CompileMetricsForUser(date time.Time, user data.User, eventChan chan<- UserEvent) {
	ctxLog := log.WithFields(log.Fields{"email": user.Email, "date": date.Format("2006-01-02")})
	defer close(eventChan)

	ctxLog.Debug("Compiling events for user.")

	//events := getDummyEventsForUser(date, user)
	events := getEventsForUser(date, user)
	ctxLog.WithField("numEvents", len(events)).Debug("Got Events.")

	meetingMins := map[string]uint{
		"mins0":       0,
		"mins1":       0,
		"mins2":       0,
		"mins3":       0,
		"mins4":       0,
		"mins5":       0,
		"mins6":       0,
		"mins7":       0,
		"mins8":       0,
		"mins9":       0,
		"mins10Plus":  0,
		"count0":      0,
		"count1":      0,
		"count2":      0,
		"count3":      0,
		"count4":      0,
		"count5":      0,
		"count6":      0,
		"count7":      0,
		"count8":      0,
		"count9":      0,
		"count10Plus": 0,
	}

	for _, event := range events {

		// filter unwanted events
		if !shouldProcessEvent(event) {
			continue
		}

		// process event
		attendees := numAttendees(event)
		mins := getEventLengthMins(event)

		ctxLog.WithFields(log.Fields{
			"id":        event.Id,
			"summary":   event.Summary,
			"attendees": attendees,
			"mins":      mins,
		}).Debug("Event Meta.")

		// store metrics
		switch {
		case attendees == 0:
			meetingMins["mins0"] += mins
			meetingMins["count0"] += 1
		case attendees == 1:
			meetingMins["mins1"] += mins
			meetingMins["count1"] += 1
		case attendees == 2:
			meetingMins["mins2"] += mins
			meetingMins["count2"] += 1
		case attendees == 3:
			meetingMins["mins3"] += mins
			meetingMins["count3"] += 1
		case attendees == 4:
			meetingMins["mins4"] += mins
			meetingMins["count4"] += 1
		case attendees == 5:
			meetingMins["mins5"] += mins
			meetingMins["count5"] += 1
		case attendees == 6:
			meetingMins["mins6"] += mins
			meetingMins["count6"] += 1
		case attendees == 7:
			meetingMins["mins7"] += mins
			meetingMins["count7"] += 1
		case attendees == 8:
			meetingMins["mins8"] += mins
			meetingMins["count8"] += 1
		case attendees == 9:
			meetingMins["mins9"] += mins
			meetingMins["count9"] += 1
		case attendees > 9:
			meetingMins["mins10Plus"] += mins
			meetingMins["count10Plus"] += 1
		}

		// send event to channel
		if shouldSaveEvent(event) {
			eventChan <- UserEvent{&user, event}
		}

	}

	// Store in database
	data.Mgr.CreateUserMeetingMins(date, user, meetingMins)

	ctxLog.Debugf("Finished compiling metrics for user. Map: %v", meetingMins)

}

func shouldProcessEvent(event *calendar.Event) bool {

	isCancelled := event.Status == "cancelled"
	hasStartAndEnd := event.Start != nil && event.End != nil && event.Start.DateTime != "" && event.End.DateTime != ""
	//belongsToRecurringEvent := event.RecurringEventId != ""

	if isCancelled || !hasStartAndEnd {
		return false
	}

	var maxHrs uint = 6
	maxMins := maxHrs * 60
	isTooLong := getEventLengthMins(event) > maxMins

	return !isTooLong
}

func shouldSaveEvent(event *calendar.Event) bool {

	// isRecurring := len(event.Recurrence) > 0
	// multipleAttendees := numAttendees(event) > 1
	// isRootEvent := !regexp.MustCompile(`(\w+)_\w+`).MatchString(event.Id) // ID is not one of recurring event like "2389fhdicvn_R20170310T200000"

	// return isRecurring && isRootEvent && multipleAttendees
	return false
}

func numAttendees(event *calendar.Event) uint8 {
	var num uint8 = 0

	for _, attendee := range event.Attendees {
		if !isRoom(attendee) && attendee.ResponseStatus == "accepted" {
			num++
		}
	}

	return num
}

func isRoom(attendee *calendar.EventAttendee) bool {
	rooms := map[string]int{
		"Green Conference Room":                      0,
		"Zen Conference Room":                        0,
		"Phone Room- Airstream (Interior) (2 seats)": 2,
	}

	_, ok := rooms[attendee.DisplayName]

	isResource := attendee.Resource

	return ok || isResource
}

func getEventsForUser(date time.Time, user data.User) []*calendar.Event {

	ctxLog := log.WithFields(log.Fields{
		"email": user.Email,
		"date":  date.Format("2006-01-02"),
	})

	// Form time range
	timeMin := date.Format(time.RFC3339)
	timeMax := date.AddDate(0, 0, 1).Format(time.RFC3339)

	ctxLog.WithFields(log.Fields{
		"timeMax": timeMax,
		"timeMin": timeMin,
	}).Debug("Querying user calendar for Events")

	// TODO: Implemnt local caching
	// Key email and date

	// check if file exist and load it
	cachePath := getCacheFilenamePathForUser(date, user)
	cacheFile, err := os.OpenFile(cachePath, os.O_RDONLY, 0600)
	var eventList = &calendar.Events{}
	cached := false
	if err == nil {
		target := &eventList
		if err := json.NewDecoder(cacheFile).Decode(target); err != nil {
			ctxLog.Error("Could not read cache file.", err)
		} else {
			ctxLog.Info("Read cached events ", cachePath)
			cached = true
		}
		defer func() {
			if err := cacheFile.Close(); err != nil {
				panic(err)
			}
		}()
	}
	if !cached {
		ctxLog.Info("Querying Google Calendar for events for user ", user.Email, " and date ", date.Format("2006-01-02"))
		calendarApi := apis.Calendar(user.Email)
		eventList, err = calendarApi.Events.List(user.Email).TimeMin(timeMin).TimeMax(timeMax).SingleEvents(true).Do()
	}

	if err != nil {
		ctxLog.Error("Could not fetch events for user.", err)
		return []*calendar.Event{}
	}

	json, err := eventList.MarshalJSON()
	if err != nil {
		ctxLog.Error("Could not marshal events for user.", err)
		return []*calendar.Event{}
	}

	// open output file
	fo, err := os.Create(cachePath)
	if err != nil {
		panic(err)
	}
	// close fo on exit and check for its returned error
	defer func() {
		if err := fo.Close(); err != nil {
			panic(err)
		}
	}()
	if _, err := fo.Write(json); err != nil {
		panic(err)
	}

	return eventList.Items
}

func getCacheFilenameOldPathForUser(date time.Time, user data.User) string {
	return fmt.Sprintf("cache/cache_%s_%s.json", user.Email, date.Format("2006-01-02"))
}

func getCacheFilenamePathForUser(date time.Time, user data.User) string {
	return fmt.Sprintf("cache/%d/%02d/cache_%s_%s.json", date.Year(), date.Month(), user.Email, date.Format("2006-01-02"))
}

func getDummyEventsForUser(date time.Time, user data.User) []*calendar.Event {

	rand.Seed(time.Now().UnixNano())

	numEvents := 10

	events := make([]*calendar.Event, numEvents)

	for i := 0; i < numEvents; i++ {
		id := cast.ToString(i)

		// randomize data
		numAttendees := rand.Intn(4)
		lengthMins := rand.Intn(60)

		// create start/end times
		hrsUntilBusinessStart := 8
		hrsUntilStart := hrsUntilBusinessStart + i
		minsUntilEnd := hrsUntilStart*60 + lengthMins
		start := date.Add(time.Duration(hrsUntilStart * int(time.Hour)))
		end := date.Add(time.Duration(minsUntilEnd * int(time.Minute)))
		startString := start.Format(EventDateTimeFormat)
		endString := end.Format(EventDateTimeFormat)

		events[i] = &calendar.Event{
			Id:          id,
			Description: "Dummy Event " + id,
			Start:       &calendar.EventDateTime{DateTime: startString},
			End:         &calendar.EventDateTime{DateTime: endString},
			Attendees:   make([]*calendar.EventAttendee, numAttendees),
		}
	}

	return events
}

func computeEventMetrics(eventMap map[string]*calendar.Event) {
	// Restriction: Only save recurring meetings with 3plus attendees

	log.WithField("len", len(eventMap)).Debug("Created Event Map")

	// For each event in the event map, store the event in the database as a meeting
	for eventId, event := range eventMap {

		frequency, err := frequencyFromEvent(event)
		if err != nil {
			continue
		}

		meeting := data.Meeting{
			ID:                eventId,
			Name:              event.Summary,
			Description:       util.StripCtlAndExtFromUTF8(event.Description),
			Attendees:         numAttendees(event),
			Mins:              uint8(getEventLengthMins(event)),
			FrequencyPerMonth: frequency,
			StartDate:         parseEventDateTime(event.Start),
			EndDate:           parseEventDateTime(event.End),
		}

		data.Mgr.CreateMeeting(&meeting)
	}

}

func frequencyFromEvent(event *calendar.Event) (uint8, error) {

	// https://regex-golang.appspot.com/assets/html/index.html
	regex := regexp.MustCompile(`FREQ=(\w+)(.*INTERVAL(\d+))?`)

	var result []string
	for _, recurrence := range event.Recurrence {
		result = regex.FindStringSubmatch(recurrence)
		if len(result) > 0 {
			break
		}
	}

	if len(result) < 1 {
		log.WithFields(log.Fields{
			"result": result,
			"event":  event,
		}).Error("Could not match regex for frequency in event.")
		return 0, errors.New("could not match regex from event")
	}

	freq := result[1]
	interval := result[3]

	switch freq + interval {
	case "WEEKLY":
		return 4, nil
	case "WEEKLY2":
		return 2, nil
	case "MONTHLY":
		return 1, nil
	default:
		log.WithFields(log.Fields{
			"summary":         event.Summary,
			"recurrence":      event.Recurrence,
			"freq + interval": freq + interval,
		}).Error("Could not parse frequency for event.")
		return 0, errors.New("could not parse frequency from event")
	}

}

// UTIL

func beginningOfYesterday() time.Time {
	return beginningOfDay(1)
}

func beginningOfDay(daysAgo int) time.Time {
	now := time.Now()
	year, month, yesterday := now.AddDate(0, 0, -1*daysAgo).Date()
	return time.Date(year, month, yesterday, 0, 0, 0, 0, now.Location())
}

func getEventLengthMins(event *calendar.Event) uint {
	end := parseEventDateTime(event.End)
	start := parseEventDateTime(event.Start)
	return uint(end.Sub(start).Minutes())
}

func parseEventDateTime(e *calendar.EventDateTime) time.Time {

	result, err := time.Parse(time.RFC3339, e.DateTime)

	if err != nil {
		log.WithFields(log.Fields{
			"DateTime": e.DateTime,
		}).Error("Could not parse EventDateTime.")
		panic("killing")
	}

	return result
}

func merge(chans ...<-chan UserEvent) <-chan UserEvent {
	var wg sync.WaitGroup
	out := make(chan UserEvent)

	// Start an output goroutine for each input channel in chans.  output
	// copies values from c to out until c is closed, then calls wg.Done.
	output := func(c <-chan UserEvent) {
		for n := range c {
			out <- n
		}
		wg.Done()
	}

	// Start routines for collecting output
	wg.Add(len(chans))
	for _, c := range chans {
		go output(c)
	}

	// Start a goroutine to close out once all the output goroutines are
	// done.  This must start after the wg.Add call.
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}
