// ===================================================
// MICRO-FRONTEND WEB COMPONENT: <lego-offers> (534)
// Exponible desde: http://localhost:3004/lego-offers.js
// ===================================================

class LegoOffers extends HTMLElement {
    constructor() {
        super();
        this.attachShadow({ mode: 'open' });
        this.offers = [];
        this.propertyId = null;
    }

    static get observedAttributes() {
        return ['property-id', 'api-url'];
    }

    attributeChangedCallback(name, oldValue, newValue) {
        if (name === 'property-id' && oldValue !== newValue) {
            this.propertyId = newValue;
            this.fetchOffers();
        }
    }

    connectedCallback() {
        this.propertyId = this.getAttribute('property-id');
        this.apiUrl = this.getAttribute('api-url') || 'http://localhost:3004';
        this.render();
        if (this.propertyId) {
            this.fetchOffers();
        }
    }

    formatCurrency(cents) {
        if (!cents) return '$0';
        return new Intl.NumberFormat('es-AR', {
            style: 'currency',
            currency: 'ARS',
            maximumFractionDigits: 0
        }).format(cents / 100);
    }

    async fetchOffers() {
        if (!this.propertyId) return;
        try {
            const res = await fetch(`${this.apiUrl}/api/offers?property_id=${this.propertyId}`);
            if (res.ok) {
                this.offers = await res.json() || [];
                this.updateContent();
            }
        } catch (e) {
            console.error('[lego-offers] Error fetching offers:', e);
        }
    }

    async submitOffer(amountARS, conditions) {
        const amountCents = Math.round(amountARS * 100);
        try {
            const res = await fetch(`${this.apiUrl}/api/offers`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    property_id: parseInt(this.propertyId),
                    buyer_id: 3, // Buyer por defecto
                    amount_cents: amountCents,
                    conditions_text: conditions
                })
            });

            if (res.ok) {
                await this.fetchOffers();
            } else {
                alert('Error enviando oferta');
            }
        } catch (e) {
            alert('Error de conexión');
        }
    }

    render() {
        this.shadowRoot.innerHTML = `
            <style>
                :host {
                    display: block;
                    font-family: var(--lego-font, system-ui, sans-serif);
                    color: var(--lego-text-main, #f8fafc);
                }

                .lego-card {
                    background: var(--lego-card-bg, #1e293b);
                    border: 1px solid var(--lego-card-border, #334155);
                    border-radius: var(--lego-radius, 12px);
                    padding: 16px;
                    display: flex;
                    flex-direction: column;
                    gap: 12px;
                }

                .header {
                    display: flex;
                    justify-content: space-between;
                    align-items: center;
                    border-bottom: 1px solid var(--lego-card-border, #334155);
                    padding-bottom: 8px;
                }

                .title {
                    font-size: 1rem;
                    font-weight: 700;
                    color: var(--lego-text-main, #f8fafc);
                }

                .badge {
                    background: var(--lego-primary, #6366f1);
                    color: #fff;
                    font-size: 0.75rem;
                    padding: 2px 8px;
                    border-radius: 10px;
                }

                .offers-list {
                    display: flex;
                    flex-direction: column;
                    gap: 8px;
                    max-height: 240px;
                    overflow-y: auto;
                }

                .offer-item {
                    background: rgba(255, 255, 255, 0.03);
                    border: 1px solid var(--lego-card-border, #334155);
                    border-radius: 8px;
                    padding: 10px;
                    display: flex;
                    justify-content: space-between;
                    align-items: center;
                    font-size: 0.85rem;
                }

                .amount {
                    font-weight: 700;
                    color: var(--lego-success, #10b981);
                }

                .fee-sub {
                    font-size: 0.75rem;
                    color: var(--lego-text-muted, #94a3b8);
                }

                .status-chip {
                    font-size: 0.7rem;
                    padding: 2px 6px;
                    border-radius: 6px;
                    text-transform: uppercase;
                    font-weight: 700;
                }

                .form-box {
                    display: flex;
                    gap: 8px;
                    margin-top: 8px;
                }

                input {
                    flex: 1;
                    background: rgba(0, 0, 0, 0.2);
                    border: 1px solid var(--lego-card-border, #334155);
                    color: #fff;
                    padding: 8px;
                    border-radius: 6px;
                    font-size: 0.85rem;
                }

                button {
                    background: var(--lego-primary, #6366f1);
                    color: #fff;
                    border: none;
                    padding: 8px 14px;
                    border-radius: 6px;
                    cursor: pointer;
                    font-size: 0.85rem;
                    font-weight: 600;
                }

                button:hover {
                    opacity: 0.9;
                }
            </style>

            <div class="lego-card">
                <div class="header">
                    <span class="title">Ofertas Registradas</span>
                    <span class="badge" id="countBadge">0</span>
                </div>

                <div class="offers-list" id="offersList">
                    <div style="font-size:0.85rem; color:#94a3b8;">Cargando ofertas...</div>
                </div>

                <form class="form-box" id="quickOfferForm">
                    <input type="number" id="quickAmount" placeholder="Ofertar (ARS)" required />
                    <button type="submit">Ofertar</button>
                </form>
            </div>
        `;

        this.shadowRoot.getElementById('quickOfferForm').addEventListener('submit', (e) => {
            e.preventDefault();
            const input = this.shadowRoot.getElementById('quickAmount');
            const amount = parseFloat(input.value);
            if (amount) {
                this.submitOffer(amount, 'Oferta desde Web Component');
                input.value = '';
            }
        });
    }

    updateContent() {
        const countBadge = this.shadowRoot.getElementById('countBadge');
        const offersList = this.shadowRoot.getElementById('offersList');

        if (countBadge) countBadge.textContent = this.offers.length;

        if (!offersList) return;

        if (this.offers.length === 0) {
            offersList.innerHTML = '<div style="font-size:0.85rem; color:#94a3b8;">Sin ofertas por el momento.</div>';
            return;
        }

        offersList.innerHTML = this.offers.map(o => `
            <div class="offer-item">
                <div>
                    <div class="amount">${this.formatCurrency(o.amount_cents)}</div>
                    <div class="fee-sub">Martillero: ${this.formatCurrency(o.martillero_fee_cents)}</div>
                </div>
                <span class="status-chip">${o.status}</span>
            </div>
        `).join('');
    }
}

customElements.define('lego-offers', LegoOffers);
