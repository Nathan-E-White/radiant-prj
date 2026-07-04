package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"
)

// TelemetryPacket defines the structured JSON payload sent to Phaser
type TelemetryPacket struct {
	Timestamp int64   `json:"timestamp"`
	Temp      float64 `json:"temp"`
	Flow      float64 `json:"flow"`
	Angle     float64 `json:"angle"`
	Pressure  float64 `json:"pressure"`
	Transient bool    `json:"transient_active"` // Tells Phaser to change UI colors/modes
}

var (
	// Mutex and flag to safely share the transient state across thread boundaries
	stateMutex      sync.RWMutex
	isTransientOpen bool
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// terminalInputListener runs in its own goroutine to prevent blocking the network loop
func terminalInputListener() {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		// Execution pauses here until the user hits 'Enter' in the console
		scanner.Scan()

		stateMutex.Lock()
		isTransientOpen = !isTransientOpen
		currentStatus := isTransientOpen
		stateMutex.Unlock()

		if currentStatus {
			fmt.Println("\n⚠️  [INPUT INTERCEPT] INJECTING OUT-OF-BOUNDS THERMAL TRANSIENT FAILURE!")
			fmt.Println("👉 Press [ENTER] again to dispatch safety injection and stabilize...")
		} else {
			fmt.Println("\n✅ [INPUT INTERCEPT] DISPATCHING SAFETY INJECTION SYSTEM. RE-STABILIZING CORE...")
			fmt.Println("👉 Press [ENTER] to inject an infrastructure failure transient.")
		}
	}
}

func handleTelemetryStream(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("WebSocket upgrade failed: %v\n", err)
		return
	}
	defer conn.Close()
	fmt.Println("\n🚀 Client connected: Phaser Data Bridge active.")

	ticker := time.NewTicker(50 * time.Millisecond) // 20Hz Tick rate
	defer ticker.Stop()

	// Establish baseline operational values
	temp := 121.0
	flow := 1500.0
	angle := 90.0
	pressure := 25.1

	for {
		select {
		case <-ticker.C:
			// Read the current failure status safely using a Read-Lock
			stateMutex.RLock()
			activeTransient := isTransientOpen
			stateMutex.RUnlock()

			if activeTransient {
				// Out-Of-Bounds (OOB) Scenario: Fast thermal runaway and pressure spikes
				temp += (rand.Float64() * 15.0) + 5.0    // Rapidly escalates heat
				flow -= (rand.Float64() * 35.0) + 10.0   // Drop in cooling capacity
				angle += (rand.Float64() * 2.0) - 0.5    // Actuator mechanical drift
				pressure += (rand.Float64() * 0.8) - 0.1 // Kinetic pressure accumulation

				// Clamp peak critical failure conditions
				if temp > 950.0 {
					temp = 950.0 + (rand.Float64()*2.0 - 1.0)
				}
				if flow < 200.0 {
					flow = 200.0 + (rand.Float64()*5.0 - 2.5)
				}
			} else {
				// Nominal Recovery Scenario: Smoothly decay variables back to baseline targets
				temp += (121.0 - temp) * 0.1
				flow += (1500.0 - flow) * 0.1
				angle += (90.0 - angle) * 0.1
				pressure += (25.1 - pressure) * 0.1

				// Standard operational sensor jitter
				temp += (rand.Float64() * 0.4) - 0.2
				flow += (rand.Float64() * 10.0) - 5.0
				angle += (rand.Float64() * 0.2) - 0.1
				pressure += (rand.Float64() * 0.04) - 0.02
			}

			packet := TelemetryPacket{
				Timestamp: time.Now().UnixMilli(),
				Temp:      temp,
				Flow:      flow,
				Angle:     angle,
				Pressure:  pressure,
				Transient: activeTransient,
			}

			jsonPayload, err := json.Marshal(packet)
			if err != nil {
				fmt.Printf("Serialization error: %v\n", err)
				return
			}

			err = conn.WriteMessage(websocket.TextMessage, jsonPayload)
			if err != nil {
				fmt.Println("❌ Client disconnected from stream loop.")
				return
			}
		}
	}
}

func main() {
	// Spin up our asynchronous console monitoring loop thread
	go terminalInputListener()

	http.HandleFunc("/telemetry", handleTelemetryStream)

	fmt.Println("⚛️  SimEngine Multi-Threaded Mock Broadcaster Active.")
	fmt.Println("👉 Press [ENTER] inside this terminal window to trigger an out-of-bounds transient event.")
	fmt.Println("--------------------------------------------------------------------------------")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("Port binding failure: %v\n", err)
	}
}
