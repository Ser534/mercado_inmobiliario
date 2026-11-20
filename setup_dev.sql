-- Esquema de base de datos para Desarrollo - Mercado Inmobiliario (Ofertas y Negociación)

-- Verificación y creación de tablas core compartidas
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    role TEXT CHECK(role IN ('admin', 'seller', 'owner', 'customer', 'worker')) NOT NULL,
    is_premium BOOLEAN DEFAULT 0,
    balance INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS properties (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    seller_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    address TEXT NOT NULL,
    description TEXT,
    latitude REAL NOT NULL,
    longitude REAL NOT NULL,
    price_cents INTEGER NOT NULL,
    video_url TEXT,
    photos_json TEXT,
    status TEXT CHECK(status IN ('active', 'accepted', 'paid', 'completed')) DEFAULT 'active',
    broker_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(seller_id) REFERENCES users(id),
    FOREIGN KEY(broker_id) REFERENCES users(id)
);

-- Tabla Principal de Ofertas
CREATE TABLE IF NOT EXISTS offers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    property_id INTEGER NOT NULL,
    buyer_id INTEGER NOT NULL,
    amount_cents INTEGER NOT NULL,
    conditions_text TEXT,
    status TEXT DEFAULT 'pending',
    counter_amount_cents INTEGER DEFAULT 0,
    expires_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(property_id) REFERENCES properties(id),
    FOREIGN KEY(buyer_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_offers_property ON offers(property_id);
CREATE INDEX IF NOT EXISTS idx_offers_buyer ON offers(buyer_id);

-- Tabla de Auditoría / Historial de Negociaciones
CREATE TABLE IF NOT EXISTS offer_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    offer_id INTEGER NOT NULL,
    actor_id INTEGER NOT NULL,
    action TEXT CHECK(action IN ('created', 'status_changed', 'countered', 'accepted', 'rejected', 'withdrawn')) NOT NULL,
    amount_cents INTEGER NOT NULL,
    note TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(offer_id) REFERENCES offers(id),
    FOREIGN KEY(actor_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_offer_history_offer ON offer_history(offer_id);

-- Tabla de Sesiones (Persistentes)
CREATE TABLE IF NOT EXISTS sessions (
    token TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL,
    expires_at DATETIME NOT NULL,
    FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- INYECCIÓN DE DATOS SEMILLA (DESARROLLO)
INSERT OR IGNORE INTO users (id, email, password_hash, role, balance) VALUES
(1, 'admin@test.com', '240be518fabd2724ddb6f04eeb1da5967448d7e831c08c8fa822809f74c720a9', 'admin', 100000000),
(2, 'seller@test.com', '2a76110d06bcc4fd437337b984131cfa82db9f792e3e2340acef9f3066b264e0', 'seller', 1000000),
(3, 'buyer@test.com', 'e547bd13228250dfb4c7df1d1ebb78cfd9f2ada56ebb0c425d35829dd3ac4ae8', 'customer', 5000000),
(4, 'broker@test.com', 'ce16be2079a6dbb36eb9ff6b0d7d9f6c4c76a3064f31805142e13b05a44bb434', 'worker', 0);

INSERT OR IGNORE INTO properties (id, seller_id, title, address, description, latitude, longitude, price_cents, status) VALUES
(1, 2, 'Departamento Moderno 2 Ambientes Palermo', 'Av. Santa Fe 3200, CABA', 'Hermoso departamento con balcón corrido y vista panorámica.', -34.5889, -58.4115, 12000000, 'active'),
(2, 2, 'Casa Quinta con Piscina San Isidro', 'Av. del Libertador 16000, San Isidro', 'Amplio parque, piscina climatizada y quincho con parrilla.', -34.4722, -58.5264, 35000000, 'active');

INSERT OR IGNORE INTO offers (id, property_id, buyer_id, amount_cents, conditions_text, status, counter_amount_cents) VALUES
(1, 1, 3, 11500000, 'Pago al contado en dólares billete dentro de los 15 días.', 'pending', 0),
(2, 2, 3, 33000000, 'Reserva con $5.000.000 y saldo a la escritura.', 'countered', 34200000);

INSERT OR IGNORE INTO offer_history (id, offer_id, actor_id, action, amount_cents, note) VALUES
(1, 1, 3, 'created', 11500000, 'Oferta inicial realizada'),
(2, 2, 3, 'created', 33000000, 'Oferta inicial realizada'),
(3, 2, 2, 'countered', 34200000, 'Contraoferta del propietario enviada');
