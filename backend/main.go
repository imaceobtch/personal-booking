package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/apognu/gocal"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

	// 1. Create event in Apple Calendar via CalDAV (mocked or actual if credentials present)
	caldavURL := os.Getenv("CALDAV_URL")
	caldavUser := os.Getenv("CALDAV_USER")
	caldavPass := os.Getenv("CALDAV_PASS")

	if caldavURL != "" && caldavUser != "" && caldavPass != "" {
		fmt.Printf("Would write to CalDAV: %s, %s, %s\n", req.Name, req.Email, req.Time)
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

	// 3. Send Email Notification (via Resend or similar) - Mocked for now
	managementURL := fmt.Sprintf("http://localhost:4321/booking/%s", bookingID)
	fmt.Printf("Would send email to %s confirming booking at %s via %s. Management link: %s\n", req.Email, req.Time, req.Platform, managementURL)

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
