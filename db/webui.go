package db

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
	"golang.org/x/term"
)

// StartWebInterface starts a web-based interface for the database
func StartWebInterface(port string) error {
	url := fmt.Sprintf("http://localhost:%s", port)
	log.Printf("Checking web interface at %s", url)

	// Check if server is responding
	for i := 0; i < 5; i++ {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			break
		}
		if err != nil {
			log.Printf("Server not ready (attempt %d/5): %v", i+1, err)
		} else {
			resp.Body.Close()
			log.Printf("Server returned status %d (attempt %d/5)", resp.StatusCode, i+1)
		}
		if i < 4 {
			time.Sleep(5 * time.Second)
		} else {
			return fmt.Errorf("server failed to respond after 5 attempts")
		}
	}

	log.Printf("Opening web interface at %s", url)
	cmd := exec.Command("xdg-open", url)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open browser: %v", err)
	}
	
	fmt.Println("\nPress 'q' to exit...")
	
	// Get the current state of the terminal
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set terminal to raw mode: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	buffer := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(buffer)
		if err != nil {
			return fmt.Errorf("error reading input: %v", err)
		}
		
		if buffer[0] == 'q' {
			fmt.Println() // Add newline after 'q'
			log.Println("Shutting down...")
			return nil
		}
	}
}
