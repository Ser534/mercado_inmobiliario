package main

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestCalculateFees(t *testing.T) {
	// Oferta de $100,000 Pesos en centavos ($100.000,00 = 10,000,000 centavos)
	var offerCents int64 = 10000000

	martillero, platform, netBroker := calculateFees(offerCents)

	// Martillero: 3% de 10,000,000 = 300,000 centavos ($3,000.00)
	expectedMartillero := int64(300000)
	if martillero != expectedMartillero {
		t.Errorf("Comisión martillero esperada: %d, obtenida: %d", expectedMartillero, martillero)
	}

	// Plataforma: 1% del Martillero = 1% de 300,000 = 3,000 centavos ($30.00)
	expectedPlatform := int64(3000)
	if platform != expectedPlatform {
		t.Errorf("Comisión plataforma esperada: %d, obtenida: %d", expectedPlatform, platform)
	}

	// Broker Neto: 300,000 - 3,000 = 297,000 centavos ($2,970.00)
	expectedNetBroker := int64(297000)
	if netBroker != expectedNetBroker {
		t.Errorf("Comisión neta broker esperada: %d, obtenida: %d", expectedNetBroker, netBroker)
	}
}

func TestNoSelectStar(t *testing.T) {
	// Verificar que en main.go no se esté utilizando SELECT *
	data, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("No se pudo leer main.go: %v", err)
	}

	content := strings.ToUpper(string(data))
	if strings.Contains(content, "SELECT * FROM") || strings.Contains(content, "SELECT * ") {
		t.Errorf("REGLA DE ECOSISTEMA VIOLADA: Se encontró 'SELECT *' en main.go. Debe enumerar explícitamente las columnas.")
	}
}

func TestDatabaseSchemaAndOffersIntegration(t *testing.T) {
	// Usar base de datos SQLite en memoria para prueba de integración rápida
	testDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Error abriendo DB en memoria: %v", err)
	}
	defer testDB.Close()

	// Cargar setup_prod.sql
	setupSQL, err := os.ReadFile("setup_prod.sql")
	if err != nil {
		t.Fatalf("Error leyendo setup_prod.sql: %v", err)
	}

	_, err = testDB.Exec(string(setupSQL))
	if err != nil {
		t.Fatalf("Error ejecutando esquema SQL en memoria: %v", err)
	}

	// Insertar usuario y propiedad de prueba
	_, err = testDB.Exec(`INSERT INTO users (id, email, password_hash, role) VALUES (1, 'buyer@test.com', 'hash', 'customer')`)
	if err != nil {
		t.Fatalf("Error insertando usuario: %v", err)
	}

	_, err = testDB.Exec(`INSERT INTO properties (id, seller_id, title, address, price_cents, latitude, longitude) VALUES (1, 1, 'Prop Test', 'Calle 123', 10000000, 0.0, 0.0)`)
	if err != nil {
		t.Fatalf("Error insertando propiedad: %v", err)
	}

	// Insertar oferta
	res, err := testDB.Exec(`INSERT INTO offers (property_id, buyer_id, amount_cents, conditions_text, status) VALUES (?, ?, ?, ?, 'pending')`, 1, 1, 9500000, "Contado")
	if err != nil {
		t.Fatalf("Error insertando oferta: %v", err)
	}

	offerID, _ := res.LastInsertId()
	if offerID <= 0 {
		t.Errorf("ID de oferta inválido: %d", offerID)
	}

	// Verificar lectura con Parameter Binding
	var amountCents int64
	var status string
	err = testDB.QueryRow(`SELECT amount_cents, status FROM offers WHERE id = ?`, offerID).Scan(&amountCents, &status)
	if err != nil {
		t.Fatalf("Error consultando oferta: %v", err)
	}

	if amountCents != 9500000 || status != "pending" {
		t.Errorf("Datos de oferta no coinciden: %d, %s", amountCents, status)
	}
}
