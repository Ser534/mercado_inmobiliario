// ===================================================
// MERCADO INMOBILIARIO (534) - VANILLA JAVASCRIPT APP
// ===================================================

document.addEventListener('DOMContentLoaded', () => {
    // ESTADO LOCAL DE LA APLICACIÓN
    const state = {
        properties: [],
        offers: [],
        stats: null,
        selectedPropertyId: null,
        activeStatusFilter: '',
        activeOfferId: null,
        currentUser: {
            id: 3,
            email: 'buyer@test.com',
            role: 'customer'
        }
    };

    // ELEMENTOS DOM
    const propertiesList = document.getElementById('propertiesList');
    const offersTableBody = document.getElementById('offersTableBody');
    const propCountBadge = document.getElementById('propCountBadge');
    const offersPanelTitle = document.getElementById('offersPanelTitle');

    // STATS DOM
    const statTotalOffers = document.getElementById('statTotalOffers');
    const statActiveNegs = document.getElementById('statActiveNegs');
    const statTotalVolume = document.getElementById('statTotalVolume');
    const statAvgDiscount = document.getElementById('statAvgDiscount');

    // MODAL REGISTRO OFERTA
    const btnNewOffer = document.getElementById('btnNewOffer');
    const offerModal = document.getElementById('offerModal');
    const btnCloseOfferModal = document.getElementById('btnCloseOfferModal');
    const btnCancelOffer = document.getElementById('btnCancelOffer');
    const offerForm = document.getElementById('offerForm');
    const formPropertySelect = document.getElementById('formPropertySelect');
    const formAmountARS = document.getElementById('formAmountARS');
    const formConditions = document.getElementById('formConditions');

    // PREVIEW DE COMISIONES
    const prevMartilleroFee = document.getElementById('prevMartilleroFee');
    const prevPlatformFee = document.getElementById('prevPlatformFee');
    const prevNetBrokerFee = document.getElementById('prevNetBrokerFee');

    // DRAWER DE AUDITORÍA
    const auditDrawer = document.getElementById('auditDrawer');
    const btnCloseDrawer = document.getElementById('btnCloseDrawer');
    const auditOfferId = document.getElementById('auditOfferId');
    const auditSummaryBox = document.getElementById('auditSummaryBox');
    const auditTimeline = document.getElementById('auditTimeline');

    // ACCIONES DE AUDITORÍA
    const counterAmountARS = document.getElementById('counterAmountARS');
    const counterNote = document.getElementById('counterNote');
    const btnAcceptOffer = document.getElementById('btnAcceptOffer');
    const btnSendCounter = document.getElementById('btnSendCounter');
    const btnRejectOffer = document.getElementById('btnRejectOffer');
    const btnSwitchUser = document.getElementById('btnSwitchUser');
    const sessionChip = document.getElementById('sessionChip');

    // FORMATEADORES DE MONEDA
    const formatARS = (cents) => {
        if (cents === undefined || cents === null) return '$0';
        const pesos = cents / 100;
        return new Intl.NumberFormat('es-AR', {
            style: 'currency',
            currency: 'ARS',
            maximumFractionDigits: 0
        }).format(pesos);
    };

    // INICIALIZACIÓN
    const init = async () => {
        setupEventListeners();
        await loadStats();
        await loadProperties();
        await loadOffers();
    };

    // EVENT LISTENERS
    const setupEventListeners = () => {
        // Filtros por Estado
        document.querySelectorAll('.filter-chip').forEach(chip => {
            chip.addEventListener('click', (e) => {
                document.querySelectorAll('.filter-chip').forEach(c => c.classList.remove('active'));
                e.target.classList.add('active');
                state.activeStatusFilter = e.target.dataset.status;
                loadOffers();
            });
        });

        // Abrir/Cerrar Modal de Oferta
        btnNewOffer.addEventListener('click', () => {
            populatePropertySelect();
            if (state.selectedPropertyId) {
                formPropertySelect.value = state.selectedPropertyId;
            }
            offerModal.classList.add('open');
        });

        const closeModal = () => offerModal.classList.remove('open');
        btnCloseOfferModal.addEventListener('click', closeModal);
        btnCancelOffer.addEventListener('click', closeModal);

        // Previsualización de Comisiones en Tiempo Real
        formAmountARS.addEventListener('input', () => {
            const amountARS = parseFloat(formAmountARS.value) || 0;
            const amountCents = Math.round(amountARS * 100);

            const martilleroCents = Math.floor((amountCents * 3) / 100);
            const platformCents = Math.floor((martilleroCents * 1) / 100);
            const netBrokerCents = martilleroCents - platformCents;

            prevMartilleroFee.textContent = formatARS(martilleroCents);
            prevPlatformFee.textContent = formatARS(platformCents);
            prevNetBrokerFee.textContent = formatARS(netBrokerCents);
        });

        // Enviar Oferta Form
        offerForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const propertyId = parseInt(formPropertySelect.value);
            const amountARS = parseFloat(formAmountARS.value);
            const conditions = formConditions.value;

            if (!propertyId || !amountARS) return;

            const amountCents = Math.round(amountARS * 100);

            try {
                const res = await fetch('/api/offers', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        property_id: propertyId,
                        buyer_id: state.currentUser.id,
                        amount_cents: amountCents,
                        conditions_text: conditions
                    })
                });

                if (res.ok) {
                    closeModal();
                    offerForm.reset();
                    prevMartilleroFee.textContent = '$0';
                    prevPlatformFee.textContent = '$0';
                    prevNetBrokerFee.textContent = '$0';
                    await loadOffers();
                    await loadProperties();
                    await loadStats();
                } else {
                    const err = await res.text();
                    alert('Error al registrar oferta: ' + err);
                }
            } catch (error) {
                alert('Error de conexión: ' + error.message);
            }
        });

        // Cerrar Drawer
        btnCloseDrawer.addEventListener('click', () => auditDrawer.classList.remove('open'));

        // Acciones desde el Drawer de Auditoría
        btnAcceptOffer.addEventListener('click', () => changeOfferStatus('accepted'));
        btnRejectOffer.addEventListener('click', () => changeOfferStatus('rejected'));
        btnSendCounter.addEventListener('click', submitCounterOffer);

        // Cambiar Rol de Pruebas
        btnSwitchUser.addEventListener('click', () => {
            if (state.currentUser.role === 'customer') {
                state.currentUser = { id: 2, email: 'seller@test.com', role: 'seller' };
            } else {
                state.currentUser = { id: 3, email: 'buyer@test.com', role: 'customer' };
            }
            sessionChip.querySelector('.user-role-badge').textContent = state.currentUser.role;
            sessionChip.querySelector('.user-email').textContent = state.currentUser.email;
        });
    };

    // CARGAR ESTADÍSTICAS DEL MERCADO
    const loadStats = async () => {
        try {
            const res = await fetch('/api/offers/stats');
            if (res.ok) {
                state.stats = await res.json();
                statTotalOffers.textContent = state.stats.total_offers_count;
                statActiveNegs.textContent = state.stats.active_negotiations;
                statTotalVolume.textContent = formatARS(state.stats.total_offered_cents);
                statAvgDiscount.textContent = `${state.stats.avg_discount_percentage.toFixed(1)}%`;
            }
        } catch (e) {
            console.error('Error cargando métricas:', e);
        }
    };

    // CARGAR PROPIEDADES
    const loadProperties = async () => {
        try {
            const res = await fetch('/api/properties');
            if (res.ok) {
                state.properties = await res.json();
                renderProperties();
            }
        } catch (e) {
            propertiesList.innerHTML = '<div class="text-center">Error cargando propiedades</div>';
        }
    };

    const renderProperties = () => {
        propCountBadge.textContent = state.properties.length;
        if (state.properties.length === 0) {
            propertiesList.innerHTML = '<div class="text-center">No hay propiedades publicadas.</div>';
            return;
        }

        propertiesList.innerHTML = state.properties.map(p => `
            <div class="property-card ${state.selectedPropertyId === p.id ? 'active' : ''}" data-id="${p.id}">
                <h3>${p.title}</h3>
                <p>${p.address}</p>
                <div class="price-tag">Precio Lista: ${formatARS(p.price_cents)}</div>
                <div class="meta-row">
                    <span>Ofertas: <strong>${p.offers_count || 0}</strong></span>
                    <span>Máx. Ofertada: <strong>${formatARS(p.max_offer_cents)}</strong></span>
                </div>
            </div>
        `).join('');

        document.querySelectorAll('.property-card').forEach(card => {
            card.addEventListener('click', () => {
                const id = parseInt(card.dataset.id);
                if (state.selectedPropertyId === id) {
                    state.selectedPropertyId = null;
                    offersPanelTitle.textContent = 'Todas las Ofertas Registradas';
                } else {
                    state.selectedPropertyId = id;
                    const prop = state.properties.find(p => p.id === id);
                    offersPanelTitle.textContent = `Ofertas para: ${prop ? prop.title : id}`;
                }
                renderProperties();
                loadOffers();
            });
        });
    };

    const populatePropertySelect = () => {
        formPropertySelect.innerHTML = '<option value="">Seleccione una propiedad...</option>' +
            state.properties.map(p => `
                <option value="${p.id}">${p.title} - ${formatARS(p.price_cents)}</option>
            `).join('');
    };

    // CARGAR OFERTAS
    const loadOffers = async () => {
        try {
            let url = '/api/offers?';
            if (state.selectedPropertyId) url += `property_id=${state.selectedPropertyId}&`;
            if (state.activeStatusFilter) url += `status=${state.activeStatusFilter}&`;

            const res = await fetch(url);
            if (res.ok) {
                state.offers = await res.json() || [];
                renderOffers();
            }
        } catch (e) {
            offersTableBody.innerHTML = '<tr><td colspan="7" class="text-center">Error al cargar ofertas</td></tr>';
        }
    };

    const renderOffers = () => {
        if (state.offers.length === 0) {
            offersTableBody.innerHTML = '<tr><td colspan="7" class="text-center">No hay ofertas registradas con estos filtros.</td></tr>';
            return;
        }

        offersTableBody.innerHTML = state.offers.map(o => `
            <tr>
                <td>#${o.id}</td>
                <td><strong>${o.property_title || ('Propiedad #' + o.property_id)}</strong></td>
                <td>${o.buyer_email || ('Usuario #' + o.buyer_id)}</td>
                <td>
                    <div style="font-weight:700;">${formatARS(o.amount_cents)}</div>
                    ${o.counter_amount_cents ? `<small style="color:var(--lego-warning);">Contra: ${formatARS(o.counter_amount_cents)}</small>` : ''}
                </td>
                <td>
                    <small>Martillero: ${formatARS(o.martillero_fee_cents)}</small><br>
                    <small style="color:var(--lego-success);">Neto: ${formatARS(o.net_broker_fee_cents)}</small>
                </td>
                <td><span class="status-badge status-${o.status}">${o.status}</span></td>
                <td>
                    <button class="btn btn-secondary btn-sm btn-audit" data-id="${o.id}">Auditar & Historial</button>
                </td>
            </tr>
        `).join('');

        document.querySelectorAll('.btn-audit').forEach(btn => {
            btn.addEventListener('click', () => {
                const offerId = parseInt(btn.dataset.id);
                openAuditDrawer(offerId);
            });
        });
    };

    // AUDITORÍA Y HISTORIAL DE NEGOCIACIÓN
    const openAuditDrawer = async (offerId) => {
        state.activeOfferId = offerId;
        auditOfferId.textContent = offerId;
        const offer = state.offers.find(o => o.id === offerId);

        if (offer) {
            auditSummaryBox.innerHTML = `
                <p><strong>Propiedad:</strong> ${offer.property_title}</p>
                <p><strong>Precio Lista:</strong> ${formatARS(offer.property_price_cents)}</p>
                <p><strong>Oferta Actual:</strong> ${formatARS(offer.amount_cents)}</p>
                <p><strong>Condiciones:</strong> ${offer.conditions_text || 'Sin condiciones'}</p>
                <p style="margin-top:6px;"><strong>Desglose Comisión Martillero (3%):</strong> ${formatARS(offer.martillero_fee_cents)} (Plataforma 1%: ${formatARS(offer.platform_fee_cents)})</p>
            `;
        }

        auditDrawer.classList.add('open');
        await loadAuditHistory(offerId);
    };

    const loadAuditHistory = async (offerId) => {
        try {
            const res = await fetch(`/api/offers/${offerId}/history`);
            if (res.ok) {
                const history = await res.json() || [];
                renderTimeline(history);
            }
        } catch (e) {
            auditTimeline.innerHTML = '<div class="timeline-empty">Error al cargar el historial</div>';
        }
    };

    const renderTimeline = (history) => {
        if (history.length === 0) {
            auditTimeline.innerHTML = '<div class="timeline-empty">Sin historial registrado</div>';
            return;
        }

        auditTimeline.innerHTML = history.map(h => `
            <div class="timeline-item">
                <div style="font-size:0.75rem; color:var(--lego-text-muted); display:flex; justify-content:space-between;">
                    <span>${h.actor_email}</span>
                    <span>${new Date(h.created_at).toLocaleString('es-AR')}</span>
                </div>
                <div style="font-weight:600; margin: 4px 0;">Acción: ${h.action.toUpperCase()} (${formatARS(h.amount_cents)})</div>
                <p style="color:var(--lego-text-muted);">${h.note || ''}</p>
            </div>
        `).join('');
    };

    const changeOfferStatus = async (status) => {
        if (!state.activeOfferId) return;

        const note = counterNote.value || `Cambio de estado a ${status}`;
        try {
            const res = await fetch(`/api/offers/${state.activeOfferId}/status`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    status: status,
                    actor_id: state.currentUser.id,
                    note: note
                })
            });

            if (res.ok) {
                counterNote.value = '';
                await loadOffers();
                await loadAuditHistory(state.activeOfferId);
                await loadStats();
            } else {
                alert('Error al actualizar estado');
            }
        } catch (e) {
            alert('Error de conexión: ' + e.message);
        }
    };

    const submitCounterOffer = async () => {
        if (!state.activeOfferId) return;
        const amountARS = parseFloat(counterAmountARS.value);
        if (!amountARS) {
            alert('Ingrese el monto de la contraoferta en Pesos ARS');
            return;
        }

        const counterCents = Math.round(amountARS * 100);
        const note = counterNote.value || 'Contraoferta enviada';

        try {
            const res = await fetch(`/api/offers/${state.activeOfferId}/counter`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    counter_amount_cents: counterCents,
                    actor_id: state.currentUser.id,
                    note: note
                })
            });

            if (res.ok) {
                counterAmountARS.value = '';
                counterNote.value = '';
                await loadOffers();
                await loadAuditHistory(state.activeOfferId);
                await loadStats();
            } else {
                alert('Error al enviar contraoferta');
            }
        } catch (e) {
            alert('Error de conexión: ' + e.message);
        }
    };

    init();
});
