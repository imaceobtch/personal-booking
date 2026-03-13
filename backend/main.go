package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/apognu/gocal"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// BookingRequest struct
type BookingRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Time     string `json:"time" binding:"required"`     // ISO8601 string, e.g. 2026-03-14T10:00:00Z
	Platform string `json:"platform" binding:"required"` // Google Meet, Telegram, etc.
}

func main() {
	r := gin.Default()

	// CORS configuration
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	r.Use(cors.New(config))

	r.GET("/api/availability", getAvailability)
	r.POST("/api/book", bookMeeting)

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

	// 1. Create event in Apple Calendar via CalDAV (mocked or actual if credentials present)
	caldavURL := os.Getenv("CALDAV_URL")
	caldavUser := os.Getenv("CALDAV_USER")
	caldavPass := os.Getenv("CALDAV_PASS")

	if caldavURL != "" && caldavUser != "" && caldavPass != "" {
		// Real CalDAV logic would go here using go-caldav
		// For now, print to console
		fmt.Printf("Would write to CalDAV: %s, %s, %s\n", req.Name, req.Email, req.Time)
	} else {
		fmt.Println("No CalDAV credentials, skipping calendar write.")
	}

	// 2. Send Telegram Notification
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if botToken != "" && chatID != "" {
		go sendTelegramNotification(botToken, chatID, req)
	} else {
		fmt.Println("No Telegram credentials, skipping notification.")
	}

	// 3. Send Email Notification (via Resend or similar) - Mocked for now
	fmt.Printf("Would send email to %s confirming booking at %s via %s\n", req.Email, req.Time, req.Platform)

	c.JSON(http.StatusOK, gin.H{"status": "booked successfully"})
}

func sendTelegramNotification(token, chatID string, req BookingRequest) {
	msg := fmt.Sprintf("📅 New Booking!\n\nName: %s\nEmail: %s\nTime: %s\nPlatform: %s", req.Name, req.Email, req.Time, req.Platform)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	payload := map[string]string{
		"chat_id": chatID,
		"text":    msg,
	}
	body, _ := json.Marshal(payload)

	http.Post(url, "application/json", bytes.NewBuffer(body))
}
