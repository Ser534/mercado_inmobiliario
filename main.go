package main

import (
	"bufio"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ==========================================
// 1. ESTRUCTURAS DE DATOS Y MODELOS (STRUCTS)
// ==========================================

type User struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	IsPremium bool   `json:"is_premium"`
	Balance   int64  `json:"balance_cents"`
}

type Property struct {
	ID          int64   `json:"id"`
	SellerID    int64   `json:"seller_id"`
	Title       string  `json:"title"`
	Address     string  `json:"address"`
	Description string  `json:"description"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	PriceCents  int64   `json:"price_cents"`
	Status      string  `json:"status"`
	OffersCount int     `json:"offers_count,omitempty"`
	MaxOfferCents int64 `json:"max_offer_cents,omitempty"`
}

type Offer struct {
	ID                 int64      `json:"id"`
	PropertyID         int64      `json:"property_id"`
	BuyerID            int64      `json:"buyer_id"`
	BuyerEmail         string     `json:"buyer_email,omitempty"`
	PropertyTitle      string     `json:"property_title,omitempty"`
	PropertyPriceCents int64      `json:"property_price_cents,omitempty"`
	AmountCents        int64      `json:"amount_cents"`
	ConditionsText     string     `json:"conditions_text"`
	Status             string     `json:"status"` // pending, accepted, rejected, countered, paid, withdrawn
	CounterAmountCents int64      `json:"counter_amount_cents"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`

	// Desglose de comisiones de intermediación calculadas
	MartilleroFeeCents int64 `json:"martillero_fee_cents"`
	PlatformFeeCents   int64 `json:"platform_fee_cents"`
	NetBrokerFeeCents  int64 `json:"net_broker_fee_cents"`
}

type OfferHistory struct {
	ID          int64     `json:"id"`
	OfferID     int64     `json:"offer_id"`
	ActorID     int64     `json:"actor_id"`
	ActorEmail  string    `json:"actor_email,omitempty"`
	Action      string    `json:"action"` // created, status_changed, countered, accepted, rejected, withdrawn
	AmountCents int64     `json:"amount_cents"`
	Note        string    `json:"note"`
	CreatedAt   time.Time `json:"created_at"`
}

type MarketStats struct {
	TotalOffersCount     int64 `json:"total_offers_count"`
	ActiveNegotiations   int64 `json:"active_negotiations"`
	TotalOfferedCents    int64 `json:"total_offered_cents"`
	AcceptedVolumeCents  int64 `json:"accepted_volume_cents"`
	AvgDiscountPercentage float64 `json:"avg_discount_percentage"`
}

// Global DB Handle
var db *sql.DB

// ==========================================
// 2. PARSER SIMPLE DE VARIABLES DE ENTORNO (.env)
// ==========================================

func loadEnv(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		return // Si no existe, continúa con variables del SO
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}

// ==========================================
// 3. AUTENTICACIÓN Y SESIONES
// ==========================================

func generateSessionToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func getUserFromSession(r *http.Request) (*User, error) {
	cookie, err := r.Cookie("session_token")
	if err != nil || cookie.Value == "" {
		return nil, fmt.Errorf("sesión no proporcionada")
	}

	var user User
	var expiresAt time.Time

	// Prohibición estricta de SELECT *: enumerar columnas explícitamente
	query := `
		SELECT u.id, u.email, u.role, u.is_premium, u.balance, s.expires_at 
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.token = ?`

	err = db.QueryRow(query, cookie.Value).Scan(
		&user.ID, &user.Email, &user.Role, &user.IsPremium, &user.Balance, &expiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("sesión inválida o expirada")
	}

	if time.Now().After(expiresAt) {
		db.Exec(`DELETE FROM sessions WHERE token = ?`, cookie.Value)
		return nil, fmt.Errorf("sesión expirada")
	}

	return &user, nil
}

// ==========================================
// 4. LÓGICA FINANCIERA Y MATEMÁTICA EN CENTAVOS
// ==========================================

func calculateFees(offerAmountCents int64) (martilleroFee int64, platformFee int64, netBrokerFee int64) {
	// Multiplicar siempre antes de dividir para no perder precisión decimal
	martilleroFee = (offerAmountCents * 3) / 100         // 3%
	platformFee = (martilleroFee * 1) / 100              // 1% del 3% del martillero
	netBrokerFee = martilleroFee - platformFee           // 99% restante para el martillero
	return
}

// ==========================================
// 5. COMPROBACIÓN PASIVA DE BASE DE DATOS
// ==========================================

func checkPassiveDatabase() {
	tables := []string{"users", "properties", "offers", "offer_history", "sessions"}
	for _, t := range tables {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, t).Scan(&name)
		if err != nil || name == "" {
			log.Fatalf("[FATAL ERROR] La tabla '%s' no existe en la base de datos central. Inicialice ejecutando: sqlite3 ../db/534.db < setup_dev.sql", t)
		}
	}
	log.Println("[INFO] Comprobación pasiva de base de datos exitosa. Todas las tablas están presentes.")
}

// ==========================================
// 6. HANDLERS API Y RUTAS (SER VEMUX NATIVO)
// ==========================================

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"app":     "mercado_inmobiliario",
		"version": "1.0.0",
		"db":      "534.db",
	})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		http.Error(w, "Email inválido", http.StatusBadRequest)
		return
	}

	var user User
	err := db.QueryRow(`SELECT id, email, role, is_premium, balance FROM users WHERE email = ?`, req.Email).
		Scan(&user.ID, &user.Email, &user.Role, &user.IsPremium, &user.Balance)
	if err != nil {
		http.Error(w, "Usuario no encontrado", http.StatusNotFound)
		return
	}

	token := generateSessionToken()
	expiresAt := time.Now().Add(24 * time.Hour)

	_, err = db.Exec(`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`, token, user.ID, expiresAt)
	if err != nil {
		http.Error(w, "Error al crear sesión", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Sesión iniciada correctamente",
		"user":    user,
		"token":   token,
	})
}

func propertiesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	// Sin SELECT *: columnas explícitas
	query := `
		SELECT p.id, p.seller_id, p.title, p.address, p.description, p.latitude, p.longitude, p.price_cents, p.status,
		       COUNT(o.id) as offers_count, COALESCE(MAX(o.amount_cents), 0) as max_offer_cents
		FROM properties p
		LEFT JOIN offers o ON p.id = o.property_id
		GROUP BY p.id`

	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, "Error al consultar propiedades: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var props []Property
	for rows.Next() {
		var p Property
		err := rows.Scan(
			&p.ID, &p.SellerID, &p.Title, &p.Address, &p.Description,
			&p.Latitude, &p.Longitude, &p.PriceCents, &p.Status,
			&p.OffersCount, &p.MaxOfferCents,
		)
		if err != nil {
			http.Error(w, "Error al escanear propiedades", http.StatusInternalServerError)
			return
		}
		props = append(props, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(props)
}

func offersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		propertyID := r.URL.Query().Get("property_id")
		buyerID := r.URL.Query().Get("buyer_id")
		statusFilter := r.URL.Query().Get("status")

		query := `
			SELECT o.id, o.property_id, o.buyer_id, u.email, p.title, p.price_cents, o.amount_cents, 
			       o.conditions_text, o.status, o.counter_amount_cents, o.expires_at, o.created_at
			FROM offers o
			JOIN users u ON o.buyer_id = u.id
			JOIN properties p ON o.property_id = p.id
			WHERE 1=1`

		args := []interface{}{}
		if propertyID != "" {
			query += " AND o.property_id = ?"
			args = append(args, propertyID)
		}
		if buyerID != "" {
			query += " AND o.buyer_id = ?"
			args = append(args, buyerID)
		}
		if statusFilter != "" {
			query += " AND o.status = ?"
			args = append(args, statusFilter)
		}
		query += " ORDER BY o.created_at DESC"

		rows, err := db.Query(query, args...)
		if err != nil {
			http.Error(w, "Error al consultar ofertas: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var offers []Offer
		for rows.Next() {
			var o Offer
			var expiresAt sql.NullTime
			err := rows.Scan(
				&o.ID, &o.PropertyID, &o.BuyerID, &o.BuyerEmail, &o.PropertyTitle, &o.PropertyPriceCents,
				&o.AmountCents, &o.ConditionsText, &o.Status, &o.CounterAmountCents, &expiresAt, &o.CreatedAt,
			)
			if err != nil {
				http.Error(w, "Error al procesar ofertas", http.StatusInternalServerError)
				return
			}
			if expiresAt.Valid {
				o.ExpiresAt = &expiresAt.Time
			}
			// Calcular comisiones en tiempo de lectura
			o.MartilleroFeeCents, o.PlatformFeeCents, o.NetBrokerFeeCents = calculateFees(o.AmountCents)
			offers = append(offers, o)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(offers)

	case http.MethodPost:
		user, err := getUserFromSession(r)
		if err != nil {
			// Si no hay cookie de sesión, permitimos especificar buyer_id en desarrollo si está en JSON
			_ = user
		}

		var req struct {
			PropertyID     int64  `json:"property_id"`
			BuyerID        int64  `json:"buyer_id"`
			AmountCents    int64  `json:"amount_cents"`
			ConditionsText string `json:"conditions_text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Datos de oferta inválidos", http.StatusBadRequest)
			return
		}

		if req.PropertyID <= 0 || req.AmountCents <= 0 {
			http.Error(w, "Propiedad y monto en centavos son obligatorios", http.StatusBadRequest)
			return
		}

		buyerID := req.BuyerID
		if user != nil {
			buyerID = user.ID
		}
		if buyerID <= 0 {
			buyerID = 3 // Fallback por defecto a buyer@test.com si se prueba sin login previo
		}

		res, err := db.Exec(`
			INSERT INTO offers (property_id, buyer_id, amount_cents, conditions_text, status)
			VALUES (?, ?, ?, ?, 'pending')`,
			req.PropertyID, buyerID, req.AmountCents, req.ConditionsText,
		)
		if err != nil {
			http.Error(w, "Error al registrar oferta: "+err.Error(), http.StatusInternalServerError)
			return
		}

		offerID, _ := res.LastInsertId()

		// Registro de auditoría en offer_history
		db.Exec(`
			INSERT INTO offer_history (offer_id, actor_id, action, amount_cents, note)
			VALUES (?, ?, 'created', ?, 'Oferta inicial enviada')`,
			offerID, buyerID, req.AmountCents,
		)

		martilleroFee, platformFee, netBrokerFee := calculateFees(req.AmountCents)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":               "Oferta documentada exitosamente",
			"offer_id":              offerID,
			"amount_cents":          req.AmountCents,
			"martillero_fee_cents": martilleroFee,
			"platform_fee_cents":   platformFee,
			"net_broker_fee_cents": netBrokerFee,
		})

	default:
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
	}
}

func offerDetailHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 {
		http.Error(w, "Ruta no encontrada", http.StatusNotFound)
		return
	}

	offerIDStr := parts[2]
	offerID, err := strconv.ParseInt(offerIDStr, 10, 64)
	if err != nil {
		http.Error(w, "ID de oferta inválido", http.StatusBadRequest)
		return
	}

	action := ""
	if len(parts) >= 4 {
		action = parts[3]
	}

	if r.Method == http.MethodGet && action == "history" {
		// Obtener historial de auditoría
		rows, err := db.Query(`
			SELECT h.id, h.offer_id, h.actor_id, u.email, h.action, h.amount_cents, h.note, h.created_at
			FROM offer_history h
			JOIN users u ON h.actor_id = u.id
			WHERE h.offer_id = ?
			ORDER BY h.created_at ASC`, offerID)
		if err != nil {
			http.Error(w, "Error al consultar historial", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var history []OfferHistory
		for rows.Next() {
			var h OfferHistory
			rows.Scan(&h.ID, &h.OfferID, &h.ActorID, &h.ActorEmail, &h.Action, &h.AmountCents, &h.Note, &h.CreatedAt)
			history = append(history, h)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(history)
		return
	}

	if r.Method == http.MethodPost && action == "status" {
		var req struct {
			Status  string `json:"status"` // accepted, rejected, withdrawn
			ActorID int64  `json:"actor_id"`
			Note    string `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Datos inválidos", http.StatusBadRequest)
			return
		}

		actorID := req.ActorID
		if actorID <= 0 {
			actorID = 2 // Vendedor por defecto
		}

		var amountCents int64
		err := db.QueryRow(`SELECT amount_cents FROM offers WHERE id = ?`, offerID).Scan(&amountCents)
		if err != nil {
			http.Error(w, "Oferta no encontrada", http.StatusNotFound)
			return
		}

		_, err = db.Exec(`UPDATE offers SET status = ? WHERE id = ?`, req.Status, offerID)
		if err != nil {
			http.Error(w, "Error al actualizar estado", http.StatusInternalServerError)
			return
		}

		// Registrar auditoría
		db.Exec(`
			INSERT INTO offer_history (offer_id, actor_id, action, amount_cents, note)
			VALUES (?, ?, 'status_changed', ?, ?)`,
			offerID, actorID, amountCents, fmt.Sprintf("Estado cambiado a %s: %s", req.Status, req.Note),
		)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":  "Estado de oferta actualizado",
			"offer_id": offerID,
			"status":   req.Status,
		})
		return
	}

	if r.Method == http.MethodPost && action == "counter" {
		var req struct {
			CounterAmountCents int64  `json:"counter_amount_cents"`
			ActorID            int64  `json:"actor_id"`
			Note               string `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.CounterAmountCents <= 0 {
			http.Error(w, "Monto de contraoferta en centavos inválido", http.StatusBadRequest)
			return
		}

		actorID := req.ActorID
		if actorID <= 0 {
			actorID = 2 // Vendedor por defecto
		}

		_, err = db.Exec(`UPDATE offers SET status = 'countered', counter_amount_cents = ? WHERE id = ?`, req.CounterAmountCents, offerID)
		if err != nil {
			http.Error(w, "Error al registrar contraoferta", http.StatusInternalServerError)
			return
		}

		// Registrar auditoría
		db.Exec(`
			INSERT INTO offer_history (offer_id, actor_id, action, amount_cents, note)
			VALUES (?, ?, 'countered', ?, ?)`,
			offerID, actorID, req.CounterAmountCents, fmt.Sprintf("Contraoferta realizada: %s", req.Note),
		)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":              "Contraoferta enviada",
			"offer_id":             offerID,
			"counter_amount_cents": req.CounterAmountCents,
		})
		return
	}

	http.Error(w, "Acción o método no soportado", http.StatusBadRequest)
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
		return
	}

	var stats MarketStats
	db.QueryRow(`SELECT COUNT(*) FROM offers`).Scan(&stats.TotalOffersCount)
	db.QueryRow(`SELECT COUNT(*) FROM offers WHERE status IN ('pending', 'countered')`).Scan(&stats.ActiveNegotiations)
	db.QueryRow(`SELECT COALESCE(SUM(amount_cents), 0) FROM offers`).Scan(&stats.TotalOfferedCents)
	db.QueryRow(`SELECT COALESCE(SUM(amount_cents), 0) FROM offers WHERE status = 'accepted'`).Scan(&stats.AcceptedVolumeCents)

	// Calcular porcentaje promedio de descuento respecto al precio de lista
	query := `
		SELECT AVG((CAST(p.price_cents AS REAL) - CAST(o.amount_cents AS REAL)) / CAST(p.price_cents AS REAL) * 100)
		FROM offers o
		JOIN properties p ON o.property_id = p.id`
	var avgDisc sql.NullFloat64
	db.QueryRow(query).Scan(&avgDisc)
	if avgDisc.Valid {
		stats.AvgDiscountPercentage = avgDisc.Float64
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// ==========================================
// 7. MAIN SERVIDOR HTTP Y RUTAS
// ==========================================

func main() {
	// Cargar entorno local si existe
	loadEnv(".env")

	port := os.Getenv("PORT")
	if port == "" {
		port = "3004"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "../db/534.db"
	}

	// Conexión nativa con SQLite WAL mode y busy timeout de 5000ms
	connStr := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000", dbPath)
	var err error
	db, err = sql.Open("sqlite3", connStr)
	if err != nil {
		log.Fatalf("[FATAL] Error abriendo base de datos SQLite: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1) // SQLite WAL concurrente recomendado

	// Comprobación pasiva obligatoria
	checkPassiveDatabase()

	mux := http.NewServeMux()

	// Servir archivos estáticos del frontend
	executableDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		executableDir = "."
	}
	_ = executableDir

	fileServer := http.FileServer(http.Dir("./"))
	mux.Handle("/", fileServer)

	// Endpoints API
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("POST /api/login", loginHandler)
	mux.HandleFunc("GET /api/properties", propertiesHandler)
	mux.HandleFunc("GET /api/offers", offersHandler)
	mux.HandleFunc("POST /api/offers", offersHandler)
	mux.HandleFunc("GET /api/offers/stats", statsHandler)
	mux.HandleFunc("/api/offers/", offerDetailHandler)

	// Web Component Distribution
	mux.HandleFunc("GET /lego-offers.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		http.ServeFile(w, r, "./lego-offers.js")
	})

	serverAddr := ":" + port
	log.Printf("[INFO] Servidor Mercado Inmobiliario escuchando en http://localhost%s", serverAddr)
	if err := http.ListenAndServe(serverAddr, mux); err != nil {
		log.Fatalf("[FATAL] Error en servidor HTTP: %v", err)
	}
}
