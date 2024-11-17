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
	http.HandleFunc("/", formHandler)
	http.HandleFunc("/submit", submitHandler)
	http.HandleFunc("/start", startHandler)

	log.Println("Listening on :8080...")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
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

	webhookUrl := r.FormValue("webhook_url")
	network := r.FormValue("network")
	rpcUrl := r.FormValue("rpc_url")
	address := r.FormValue("address")
	alertBalanceStr := r.FormValue("alert_balance")

	alertBalance, err := strconv.Atoi(alertBalanceStr)
	if err != nil {
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

func updateConfig(address, rpcUrl, chain string, alertBalance int) {
	configPath := "AddressAndChain.toml"
	file, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	_, err = file.WriteString(
		"[[addresses]]\n" +
			"address = \"" + address + "\"\n" +
			"rpc_url = \"" + rpcUrl + "\"\n" +
			"chain = \"" + chain + "\"\n" +
			"alert_balance = " + strconv.Itoa(alertBalance) + "\n\n")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Updated AddressAndChain.toml with address=%s, rpcUrl=%s, chain=%s, alertBalance=%d", address, rpcUrl, chain, alertBalance)
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
