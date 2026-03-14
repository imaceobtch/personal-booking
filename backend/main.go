package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/apognu/gocal"
	"github.com/emersion/go-webdav/caldav"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gopkg.in/gomail.v2"
)

// BookingRequest struct
type BookingRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Time     string `json:"time" binding:"required"`     // ISO8601 string, e.g. 2026-03-14T10:00:00Z
	Platform string `json:"platform" binding:"required"` // Google Meet, Telegram, etc.
}

type Booking struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Time      string `json:"time"`
	Platform  string `json:"platform"`
	CreatedAt time.Time `json:"created_at"`
}

type RescheduleRequest struct {
	Time string `json:"time" binding:"required"`
}

var (
	bookings      = make(map[string]Booking)
	bookingsMutex sync.RWMutex
)

func main() {
	r := gin.Default()

	// CORS configuration
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	r.Use(cors.New(config))

	r.GET("/api/availability", getAvailability)
	r.POST("/api/book", bookMeeting)

	// Booking management routes
	r.GET("/api/booking/:id", getBooking)
	r.PUT("/api/booking/:id", rescheduleBooking)
	r.DELETE("/api/booking/:id", cancelBooking)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Backend listening on port %s\n", port)
	r.Run(":" + port)
}

func getAvailability(c *gin.Context) {
	dateParam := c.Query("date") // Format: YYYY-MM-DD
	if dateParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date parameter is required"})
		return
	}

	targetDate, err := time.Parse("2006-01-02", dateParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format, use YYYY-MM-DD"})
		return
	}

	icsURL := os.Getenv("ICLOUD_ICS_URL")
	if icsURL == "" {
		// Mock response if no ICS URL is configured
		c.JSON(http.StatusOK, gin.H{
			"date":  dateParam,
			"slots": []string{"10:00", "11:30", "14:00", "15:30"},
		})
		return
	}

	// Fetch ICS
	resp, err := http.Get(icsURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch calendar"})
		return
	}
	defer resp.Body.Close()

	// Parse ICS
	cal := gocal.NewParser(resp.Body)
	cal.Start, cal.End = &targetDate, &targetDate
	endOfDay := targetDate.Add(24 * time.Hour)
	cal.End = &endOfDay

	err = cal.Parse()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse calendar"})
		return
	}

	// Calculate free slots (9 AM to 5 PM)
	workingStart := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 9, 0, 0, 0, targetDate.Location())
	workingEnd := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 17, 0, 0, 0, targetDate.Location())

	// Build a list of all 30-min slots
	allSlots := []time.Time{}
	for t := workingStart; t.Before(workingEnd); t = t.Add(30 * time.Minute) {
		allSlots = append(allSlots, t)
	}

	// Filter out overlapping slots
	availableSlots := []string{}
	for _, slotStart := range allSlots {
		slotEnd := slotStart.Add(30 * time.Minute)
		isFree := true

		for _, event := range cal.Events {
			if event.Start == nil || event.End == nil {
				continue
			}
			// If slot overlaps with event
			if slotStart.Before(*event.End) && slotEnd.After(*event.Start) {
				isFree = false
				break
			}
		}

		if isFree && slotStart.After(time.Now()) { // Only future slots
			availableSlots = append(availableSlots, slotStart.Format("15:04"))
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"date":  dateParam,
		"slots": availableSlots,
	})
}

func bookMeeting(c *gin.Context) {
	var req BookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	bookingID := uuid.New().String()
	booking := Booking{
		ID:        bookingID,
		Name:      req.Name,
		Email:     req.Email,
		Time:      req.Time,
		Platform:  req.Platform,
		CreatedAt: time.Now(),
	}

	bookingsMutex.Lock()
	bookings[bookingID] = booking
	bookingsMutex.Unlock()

	// 1. Create event in Apple Calendar via CalDAV
	caldavURL := os.Getenv("CALDAV_URL")
	caldavUser := os.Getenv("CALDAV_USER")
	caldavPass := os.Getenv("CALDAV_PASS")

	if caldavURL != "" && caldavUser != "" && caldavPass != "" {
		go createCalDAVEvent(caldavURL, caldavUser, caldavPass, booking)
	} else {
		fmt.Println("No CalDAV credentials, skipping calendar write.")
	}

	// 2. Send Telegram Notification
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if botToken != "" && chatID != "" {
		go sendTelegramNotification(botToken, chatID, booking, "New")
	} else {
		fmt.Println("No Telegram credentials, skipping notification.")
	}

	// 3. Send Email Notification
	managementURL := fmt.Sprintf("http://localhost:4321/booking/%s", bookingID)
	go sendEmailNotification(booking, managementURL)

	c.JSON(http.StatusOK, gin.H{
		"status": "booked successfully",
		"id":     bookingID,
		"link":   managementURL,
	})
}

func getBooking(c *gin.Context) {
	id := c.Param("id")

	bookingsMutex.RLock()
	booking, exists := bookings[id]
	bookingsMutex.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
		return
	}

	c.JSON(http.StatusOK, booking)
}

func rescheduleBooking(c *gin.Context) {
	id := c.Param("id")
	var req RescheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	bookingsMutex.Lock()
	booking, exists := bookings[id]
	if !exists {
		bookingsMutex.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
		return
	}

	oldTime := booking.Time
	booking.Time = req.Time
	bookings[id] = booking
	bookingsMutex.Unlock()

	fmt.Printf("Booking %s rescheduled from %s to %s\n", id, oldTime, req.Time)

	// Send Telegram Notification
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if botToken != "" && chatID != "" {
		go sendTelegramNotification(botToken, chatID, booking, "Rescheduled")
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "rescheduled successfully",
		"booking": booking,
	})
}

func cancelBooking(c *gin.Context) {
	id := c.Param("id")

	bookingsMutex.Lock()
	booking, exists := bookings[id]
	if !exists {
		bookingsMutex.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
		return
	}

	delete(bookings, id)
	bookingsMutex.Unlock()

	fmt.Printf("Booking %s cancelled\n", id)

	// Send Telegram Notification
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if botToken != "" && chatID != "" {
		go sendTelegramNotification(botToken, chatID, booking, "Cancelled")
	}

	c.JSON(http.StatusOK, gin.H{"status": "cancelled successfully"})
}

func sendTelegramNotification(token, chatID string, booking Booking, action string) {
	msg := fmt.Sprintf("📅 Booking %s!\n\nName: %s\nEmail: %s\nTime: %s\nPlatform: %s", action, booking.Name, booking.Email, booking.Time, booking.Platform)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	payload := map[string]string{
		"chat_id": chatID,
		"text":    msg,
	}
	body, _ := json.Marshal(payload)

	http.Post(url, "application/json", bytes.NewBuffer(body))
}

func createCalDAVEvent(urlStr, user, pass string, booking Booking) {
	req, _ := http.NewRequest("OPTIONS", urlStr, nil)
	req.SetBasicAuth(user, pass)

	// Since go-webdav/caldav requires specific request modifiers for auth in its newer version,
	// let's wrap the client if needed, or simply pass context.Background()

	// Create a transport that sets basic auth for every request
	authTransport := &basicAuthTransport{
		Username: user,
		Password: pass,
		Transport: http.DefaultTransport,
	}

	httpClient := &http.Client{Transport: authTransport}
	client, err := caldav.NewClient(httpClient, urlStr)
	if err != nil {
		fmt.Println("CalDAV Error creating client:", err)
		return
	}

	ctx := context.Background()
	principal, err := client.FindCurrentUserPrincipal(ctx)
	if err != nil {
		fmt.Println("CalDAV Error finding principal:", err)
		return
	}

	homeSet, err := client.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		fmt.Println("CalDAV Error finding home set:", err)
		return
	}

	calendars, err := client.FindCalendars(ctx, homeSet)
	if err != nil || len(calendars) == 0 {
		fmt.Println("CalDAV Error finding calendars or no calendars found:", err)
		return
	}

	// We pick the first calendar for simplicity
	calURL := calendars[0].Path

	startTime, _ := time.Parse(time.RFC3339, booking.Time)
	endTime := startTime.Add(30 * time.Minute)

	icsContent := fmt.Sprintf(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Ihor Kiiaiev//Booking System//EN
BEGIN:VEVENT
UID:%s
DTSTAMP:%s
DTSTART:%s
DTEND:%s
SUMMARY:Meeting with %s
DESCRIPTION:Platform: %s\nEmail: %s
END:VEVENT
END:VCALENDAR`,
		booking.ID,
		time.Now().UTC().Format("20060102T150405Z"),
		startTime.UTC().Format("20060102T150405Z"),
		endTime.UTC().Format("20060102T150405Z"),
		booking.Name,
		booking.Platform,
		booking.Email,
	)

	parsedURL, _ := url.ParseRequestURI(urlStr)

	putURL := parsedURL.Scheme + "://" + parsedURL.Host + calURL + booking.ID + ".ics"
	putReq, err := http.NewRequest("PUT", putURL, bytes.NewBufferString(icsContent))
	if err != nil {
		fmt.Println("Error creating CalDAV PUT request:", err)
		return
	}
	putReq.Header.Set("Content-Type", "text/calendar; charset=utf-8")

	resp, err := httpClient.Do(putReq)
	if err != nil {
		fmt.Println("Error performing CalDAV PUT:", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Println("Failed to create CalDAV event, status:", resp.Status)
	} else {
		fmt.Println("Successfully created CalDAV event.")
	}
}

func sendEmailNotification(booking Booking, managementURL string) {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := 587 // default
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	fromEmail := os.Getenv("SMTP_FROM")

	if smtpHost == "" || smtpUser == "" || smtpPass == "" || fromEmail == "" {
		fmt.Printf("SMTP credentials missing, would send email to %s confirming booking at %s via %s. Management link: %s\n", booking.Email, booking.Time, booking.Platform, managementURL)
		return
	}

	m := gomail.NewMessage()
	m.SetHeader("From", fromEmail)
	m.SetHeader("To", booking.Email)
	m.SetHeader("Subject", "Booking Confirmation: Meeting with Ihor Kiiaiev")

	body := fmt.Sprintf(`
		<h1>Meeting Confirmed</h1>
		<p>Hi %s,</p>
		<p>Your meeting with Ihor is confirmed for %s.</p>
		<p>Platform: %s</p>
		<br/>
		<p>Need to make changes? Manage your booking here: <a href="%s">%s</a></p>
	`, booking.Name, booking.Time, booking.Platform, managementURL, managementURL)

	m.SetBody("text/html", body)

	d := gomail.NewDialer(smtpHost, smtpPort, smtpUser, smtpPass)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	if err := d.DialAndSend(m); err != nil {
		fmt.Println("Error sending email:", err)
	} else {
		fmt.Println("Email sent to", booking.Email)
	}
}

type basicAuthTransport struct {
	Username  string
	Password  string
	Transport http.RoundTripper
}

func (bat *basicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := new(http.Request)
	*req2 = *req
	req2.Header = make(http.Header, len(req.Header))
	for k, s := range req.Header {
		req2.Header[k] = append([]string(nil), s...)
	}
	req2.SetBasicAuth(bat.Username, bat.Password)
	return bat.Transport.RoundTrip(req2)
}
