package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
)

type FormData struct {
	WebhookUrl   string
	Network      string
	RpcUrl       string
	Address      string
	AlertBalance int
}

var networks = map[string]string{
	"Bitcoin":  "Bitcoin",
	"Sepolia":  "https://sepolia.infura.io/v3/YOUR_INFURA_PROJECT_ID",
	"Arbitrum": "https://arbitrum-mainnet.infura.io/v3/YOUR_INFURA_PROJECT_ID",
}

func main() {
	// Add security headers middleware
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
	})

	http.HandleFunc("/", formHandler)
	http.HandleFunc("/submit", submitHandler)
	http.HandleFunc("/start", startHandler)

	log.Println("Listening on :8080...")
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	
	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt)
		<-sigChan
		
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("HTTP server Shutdown: %v", err)
		}
	}()

	err := srv.ListenAndServe()
	if err != nil {
		if err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}
}

func formHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("static/form.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, networks)
}

func submitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Maximum size for form data
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	webhookUrl := r.FormValue("webhook_url")
	network := r.FormValue("network")
	rpcUrl := r.FormValue("rpc_url")
	address := r.FormValue("address")
	alertBalanceStr := r.FormValue("alert_balance")

	// Validate webhook URL
	if _, err := url.ParseRequestURI(webhookUrl); err != nil {
		http.Error(w, "Invalid webhook URL", http.StatusBadRequest)
		return
	}

	// Validate network
	if _, valid := networks[network]; !valid {
		http.Error(w, "Invalid network", http.StatusBadRequest)
		return
	}

	// Validate RPC URL
	if _, err := url.ParseRequestURI(rpcUrl); err != nil {
		http.Error(w, "Invalid RPC URL", http.StatusBadRequest)
		return
	}

	// Validate address format
	if !regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`).MatchString(address) {
		http.Error(w, "Invalid address format", http.StatusBadRequest)
		return
	}

	alertBalance, err := strconv.Atoi(alertBalanceStr)
	if err != nil || alertBalance <= 0 {
		http.Error(w, "Invalid alert balance", http.StatusBadRequest)
		return
	}

	log.Printf("Received form data: webhookUrl=%s, network=%s, rpcUrl=%s, address=%s, alertBalance=%d", webhookUrl, network, rpcUrl, address, alertBalance)

	// Update AddressAndChain.toml
	updateConfig(address, rpcUrl, network, alertBalance)

	// Update .env file
	updateEnv(webhookUrl)

	http.Redirect(w, r, "/start", http.StatusSeeOther)
}

func startHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("static/start.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)

	go startRustApp()
}

var configMutex sync.Mutex

func updateConfig(address, rpcUrl, chain string, alertBalance int) {
	configMutex.Lock()
	defer configMutex.Unlock()

	configPath := "AddressAndChain.toml"
	
	// Create a TOML structure
	type Address struct {
		Address      string `toml:"address"`
		RpcUrl       string `toml:"rpc_url"`
		Chain        string `toml:"chain"`
		AlertBalance int    `toml:"alert_balance"`
	}
	
	addresses := []Address{{
		Address:      address,
		RpcUrl:       rpcUrl,
		Chain:        chain,
		AlertBalance: alertBalance,
	}}
	
	// Marshal to TOML
	config := map[string]interface{}{
		"addresses": addresses,
	}
	
	data, err := toml.Marshal(config)
	if err != nil {
		log.Printf("Error marshaling config: %v", err)
		return
	}

	// Write atomically
	tempFile := configPath + ".tmp"
	err = os.WriteFile(tempFile, data, 0644)
	if err != nil {
		log.Printf("Error writing config: %v", err)
		return
	}
	
	err = os.Rename(tempFile, configPath)
	if err != nil {
		log.Printf("Error replacing config: %v", err)
		return
	}
}

func updateEnv(webhookUrl string) {
	envPath := ".env"
	file, err := os.OpenFile(envPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	_, err = file.WriteString("WEBHOOK=" + webhookUrl + "\n")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Updated .env with webhookUrl=%s", webhookUrl)
}

func startRustApp() {
	log.Println("Starting Rust application...")
	cmd := exec.Command("cargo", "run", "--release")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	err = cmd.Wait()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Rust application started.")
}
