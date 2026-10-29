package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	container := BuildAppContainer()

	fmt.Println("==================================================================")
	fmt.Println("   SwiftDrop Core API (Modular Monolith) — Identity & Auth Ready")
	fmt.Println("   Storage: 🐘 PostgreSQL (Authoritative Database)")
	fmt.Println("==================================================================")
	fmt.Println(" 5 Mock Accounts Seeded in PostgreSQL:")
	fmt.Println(" 1. ADMIN:      admin@swiftdrop.local      / AdminPassword123!")
	fmt.Println(" 2. DISPATCHER: dispatcher@swiftdrop.local / Dispatcher123!")
	fmt.Println(" 3. COURIER:    courier@swiftdrop.local    / CourierPassword123!")
	fmt.Println(" 4. MERCHANT:   merchant@vendor.com        / Merchant123! (Tenant: mrc_01HMERCHANTTENANT00001)")
	fmt.Println(" 5. CUSTOMER:   customer@swiftdrop.local   / CustomerPassword123!")
	fmt.Println(" -----------------------------------------------------------------")
	fmt.Println(" Auth REST API Endpoints:")
	fmt.Println(" - POST http://localhost:" + port + "/api/v1/auth/login")
	fmt.Println(" - GET  http://localhost:" + port + "/api/v1/auth/me (Bearer Token)")
	fmt.Println(" - GET  http://localhost:" + port + "/api/v1/admin/dashboard (Admin only)")
	fmt.Println(" - GET  http://localhost:" + port + "/api/v1/courier/dashboard (Courier/Admin)")
	fmt.Println(" - GET  http://localhost:" + port + "/api/v1/merchant/portal (Merchant/Admin)")
	fmt.Println("==================================================================")

	serverAddr := ":" + port
	log.Printf("Server listening on http://localhost:%s\n", port)
	if err := http.ListenAndServe(serverAddr, container.Mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
