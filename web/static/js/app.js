// ================================
// HARDLINK UI - FRONT LOGIC
// ================================

// ----- ELEMENTS GLOBAUX -----
const themeToggle = document.getElementById("theme-toggle");
const logPanel = document.getElementById("log-panel");

// Tabs
const tabButtons = document.querySelectorAll(".tab-button");
const tabContents = document.querySelectorAll(".tab-content");

// Explorer
const searchInput = document.getElementById("search");
const breadcrumbEl = document.getElementById("breadcrumb");
const explorerTableBody = document.querySelector("#fb-table tbody");
const detailsEl = document.getElementById("details");
const explorerDeleteToggle = document.getElementById("explorer-delete-toggle");
const explorerDeleteStatus = document.getElementById("explorer-delete-status");

// Hardlink creator
const hlSrcBreadcrumb = document.getElementById("hl-src-breadcrumb");
const hlDestBreadcrumb = document.getElementById("hl-dest-breadcrumb");
const hlSrcTableBody = document.querySelector("#hl-src-table tbody");
const hlDestTableBody = document.querySelector("#hl-dest-table tbody");
const hlSrcSelectedEl = document.getElementById("hl-src-selected");
const hlDestSelectedEl = document.getElementById("hl-dest-selected");
const hlBtnNewFolder = document.getElementById("hl-btn-new-folder");
const hlBtnCreate = document.getElementById("hl-btn-create");
const hlSelectMode = document.getElementById("hl-select-mode");

// Doublons
const btnDupScan = document.getElementById("btn-dup-scan");
const btnDupConvert = document.getElementById("btn-dup-convert");
const dupTableBody = document.querySelector("#dup-table tbody");
const dupSummaryEl = document.getElementById("dup-summary");

// Dashboard cards
const dupCardGroups = document.getElementById("dup-card-groups")?.querySelector(".dup-card-value");
const dupCardFiles = document.getElementById("dup-card-files")?.querySelector(".dup-card-value");
const dupCardSpace = document.getElementById("dup-card-space")?.querySelector(".dup-card-value");

// Paramètres
const rootLabelInput = document.getElementById("root-label-input");
const rootLabelSave = document.getElementById("root-label-save");
const themeDefaultDark = document.getElementById("theme-default-dark");
const themeDefaultLight = document.getElementById("theme-default-light");

// ----- STATE -----
let logs = [];
let logLevel = localStorage.getItem("logLevel") || "minimal"; // "minimal" | "debug" | "trace"

// Explorer state
let isLoadingFolder = false;
let currentPath = "/";
let currentSelection = null;
let explorerDeleteMode = false;

// Hardlink creator state
let hlSrcPath = "/";
let hlDestPath = "/";
let hlSelectionMode = "single"; // "single" | "multi"
let hlSrcSelectedPaths = [];     // [{path, isDir}]
let hlDestSelectedPath = "/";

// Root label
let ROOT_LABEL = localStorage.getItem("rootLabel") || "DATA";

// ----- MODAL SYSTEM -----

function showModal(type, title, message) {
    // type: 'success', 'error', 'warning', 'info'
    const overlay = document.getElementById("modal-overlay");
    const content = overlay?.querySelector(".modal-content");
    const titleEl = overlay?.querySelector(".modal-title");
    const messageEl = overlay?.querySelector(".modal-message");
    const closeBtn = overlay?.querySelector(".modal-close");
    
    if (!overlay || !content || !titleEl || !messageEl) return;
    
    // Set content
    titleEl.textContent = title;
    messageEl.textContent = message;
    
    // Remove previous type classes
    content.classList.remove("success", "error", "warning", "info");
    content.classList.add(type);
    
    // Show modal
    overlay.classList.remove("hidden");
    
    // Close on button click
    const closeHandler = () => {
        overlay.classList.add("hidden");
        closeBtn.removeEventListener("click", closeHandler);
        overlay.removeEventListener("click", overlayHandler);
    };
    
    // Close on overlay click
    const overlayHandler = (e) => {
        if (e.target === overlay) {
            closeHandler();
        }
    };
    
    closeBtn.addEventListener("click", closeHandler);
    overlay.addEventListener("click", overlayHandler);
}

function showConfirmModal(title, message, onConfirm, onCancel) {
    const overlay = document.getElementById("modal-overlay");
    const content = overlay?.querySelector(".modal-content");
    const titleEl = overlay?.querySelector(".modal-title");
    const messageEl = overlay?.querySelector(".modal-message");
    
    if (!overlay || !content || !titleEl || !messageEl) return;
    
    // Set content
    titleEl.textContent = title;
    messageEl.textContent = message;
    
    // Remove previous type classes
    content.classList.remove("success", "error", "warning", "info");
    content.classList.add("warning");
    
    // Replace buttons with confirm/cancel
    const btnContainer = content.querySelector(".modal-buttons") || document.createElement("div");
    btnContainer.className = "modal-buttons";
    btnContainer.innerHTML = `
        <button class="btn-secondary modal-cancel">Annuler</button>
        <button class="btn modal-confirm">Confirmer</button>
    `;
    
    // Remove old button if exists
    const oldBtn = content.querySelector(".modal-close");
    if (oldBtn) oldBtn.remove();
    
    content.appendChild(btnContainer);
    
    // Show modal
    overlay.classList.remove("hidden");
    
    const confirmBtn = btnContainer.querySelector(".modal-confirm");
    const cancelBtn = btnContainer.querySelector(".modal-cancel");
    
    const cleanup = () => {
        overlay.classList.add("hidden");
        btnContainer.remove();
        // Restore original OK button
        if (!content.querySelector(".modal-close")) {
            const okBtn = document.createElement("button");
            okBtn.className = "btn modal-close";
            okBtn.textContent = "OK";
            content.appendChild(okBtn);
        }
    };
    
    const handleConfirm = () => {
        cleanup();
        if (onConfirm) onConfirm();
    };
    
    const handleCancel = () => {
        cleanup();
        if (onCancel) onCancel();
    };
    
    const handleOverlayClick = (e) => {
        if (e.target === overlay) {
            handleCancel();
        }
    };
    
    confirmBtn.addEventListener("click", handleConfirm, { once: true });
    cancelBtn.addEventListener("click", handleCancel, { once: true });
    overlay.addEventListener("click", handleOverlayClick, { once: true });
}

function showPromptModal(title, message, defaultValue, onSubmit, onCancel) {
    const overlay = document.getElementById("modal-overlay");
    const content = overlay?.querySelector(".modal-content");
    const titleEl = overlay?.querySelector(".modal-title");
    const messageEl = overlay?.querySelector(".modal-message");
    
    if (!overlay || !content || !titleEl || !messageEl) return;
    
    // Set content
    titleEl.textContent = title;
    messageEl.textContent = message;
    
    // Remove previous type classes
    content.classList.remove("success", "error", "warning", "info");
    content.classList.add("info");
    
    // Add input field
    const inputContainer = document.createElement("div");
    inputContainer.className = "modal-input-container";
    inputContainer.innerHTML = `
        <input type="text" class="modal-input search-box" value="${defaultValue || ''}" placeholder="${message}">
    `;
    
    // Replace buttons with submit/cancel
    const btnContainer = document.createElement("div");
    btnContainer.className = "modal-buttons";
    btnContainer.innerHTML = `
        <button class="btn-secondary modal-cancel">Annuler</button>
        <button class="btn modal-submit">OK</button>
    `;
    
    // Remove old button if exists
    const oldBtn = content.querySelector(".modal-close");
    if (oldBtn) oldBtn.remove();
    
    // Insert input before buttons
    const msgEl = content.querySelector(".modal-message");
    msgEl.after(inputContainer);
    content.appendChild(btnContainer);
    
    // Show modal
    overlay.classList.remove("hidden");
    
    const input = inputContainer.querySelector(".modal-input");
    const submitBtn = btnContainer.querySelector(".modal-submit");
    const cancelBtn = btnContainer.querySelector(".modal-cancel");
    
    // Focus input
    setTimeout(() => input.focus(), 100);
    
    const cleanup = () => {
        overlay.classList.add("hidden");
        inputContainer.remove();
        btnContainer.remove();
        // Restore original OK button
        if (!content.querySelector(".modal-close")) {
            const okBtn = document.createElement("button");
            okBtn.className = "btn modal-close";
            okBtn.textContent = "OK";
            content.appendChild(okBtn);
        }
    };
    
    const handleSubmit = () => {
        const value = input.value.trim();
        cleanup();
        if (onSubmit) onSubmit(value);
    };
    
    const handleCancel = () => {
        cleanup();
        if (onCancel) onCancel();
    };
    
    const handleOverlayClick = (e) => {
        if (e.target === overlay) {
            handleCancel();
        }
    };
    
    const handleKeyPress = (e) => {
        if (e.key === "Enter") {
            handleSubmit();
        } else if (e.key === "Escape") {
            handleCancel();
        }
    };
    
    submitBtn.addEventListener("click", handleSubmit, { once: true });
    cancelBtn.addEventListener("click", handleCancel, { once: true });
    overlay.addEventListener("click", handleOverlayClick, { once: true });
    input.addEventListener("keydown", handleKeyPress);
}

// ----- LOADING OVERLAY SYSTEM -----

function showLoadingOverlay(message = "Traitement en cours...") {
    // Remove existing overlay if any
    let overlay = document.getElementById("loading-overlay");
    if (overlay) {
        overlay.remove();
    }
    
    // Create loading overlay
    overlay = document.createElement("div");
    overlay.id = "loading-overlay";
    overlay.className = "modal-overlay";
    overlay.innerHTML = `
        <div class="modal-content" style="min-width: 300px;">
            <div style="display: flex; flex-direction: column; align-items: center; gap: 16px;">
                <span class="spinner" style="width: 40px; height: 40px; border-width: 4px;"></span>
                <p style="margin: 0; font-size: 16px; font-weight: 600; color: var(--fg);">${escapeHtml(message)}</p>
            </div>
        </div>
    `;
    document.body.appendChild(overlay);
    
    // Disable all buttons and inputs to prevent user interaction
    document.querySelectorAll("button, input, select, textarea").forEach(el => {
        el.dataset.wasDisabled = el.disabled ? "true" : "false";
        el.disabled = true;
    });
}

function hideLoadingOverlay() {
    const overlay = document.getElementById("loading-overlay");
    if (overlay) {
        overlay.remove();
    }
    
    // Re-enable previously enabled buttons and inputs
    document.querySelectorAll("button, input, select, textarea").forEach(el => {
        if (el.dataset.wasDisabled === "false") {
            el.disabled = false;
        }
        delete el.dataset.wasDisabled;
    });
}

// ----- MOBILE TOOLTIP SYSTEM -----

let currentTooltip = null;

function showMobileTooltip(text, x, y) {
    // Remove existing tooltip
    if (currentTooltip) {
        currentTooltip.remove();
        currentTooltip = null;
    }
    
    // Create tooltip element
    const tooltip = document.createElement("div");
    tooltip.className = "mobile-tooltip";
    tooltip.textContent = text;
    document.body.appendChild(tooltip);
    
    // Position tooltip
    const tooltipRect = tooltip.getBoundingClientRect();
    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;
    
    // Adjust position to keep tooltip in viewport
    let finalX = x;
    let finalY = y;
    
    if (x + tooltipRect.width > viewportWidth - 10) {
        finalX = viewportWidth - tooltipRect.width - 10;
    }
    if (finalX < 10) {
        finalX = 10;
    }
    
    if (y + tooltipRect.height > viewportHeight - 10) {
        finalY = y - tooltipRect.height - 10;
    }
    if (finalY < 10) {
        finalY = 10;
    }
    
    tooltip.style.left = finalX + "px";
    tooltip.style.top = finalY + "px";
    
    currentTooltip = tooltip;
    
    // Auto-hide after 3 seconds or on any touch
    const hideTooltip = () => {
        if (currentTooltip) {
            currentTooltip.remove();
            currentTooltip = null;
        }
        document.removeEventListener("touchstart", hideTooltip);
    };
    
    setTimeout(hideTooltip, 3000);
    document.addEventListener("touchstart", hideTooltip, { once: true });
}

function setupMobileTooltips() {
    // Add touch handlers for truncated table cells on mobile
    if ('ontouchstart' in window) {
        // Cache to avoid expensive scrollWidth checks
        const truncatedCells = new WeakMap();
        
        document.addEventListener('touchstart', (e) => {
            const cell = e.target.closest('td[title]');
            if (cell && cell.title) {
                // Check if we've already determined this cell is truncated
                let isTruncated = truncatedCells.get(cell);
                if (isTruncated === undefined) {
                    isTruncated = cell.scrollWidth > cell.clientWidth;
                    truncatedCells.set(cell, isTruncated);
                }
                
                const text = cell.textContent.trim();
                const title = cell.title;
                
                // Only show tooltip if text is actually truncated
                if (title && title !== text && isTruncated) {
                    e.preventDefault();
                    const rect = cell.getBoundingClientRect();
                    showMobileTooltip(title, rect.left, rect.bottom + 5);
                }
            }
        });
    }
}

// ----- THEME -----

function applyTheme(theme) {
    document.documentElement.setAttribute("data-theme", theme);
    localStorage.setItem("theme", theme);
    if (themeToggle) {
        themeToggle.textContent = theme === "light" ? "🌙" : "☀️";
    }
}

if (themeToggle) {
    themeToggle.addEventListener("click", () => {
        const current = localStorage.getItem("theme") || "dark";
        applyTheme(current === "dark" ? "light" : "dark");
        refreshThemeButtons();
    });
}

applyTheme(localStorage.getItem("theme") || "dark");

function refreshThemeButtons() {
    const current = localStorage.getItem("theme") || "dark";
    if (themeDefaultDark && themeDefaultLight) {
        themeDefaultDark.classList.remove("btn-theme-active");
        themeDefaultLight.classList.remove("btn-theme-active");
        if (current === "dark") {
            themeDefaultDark.classList.add("btn-theme-active");
        } else {
            themeDefaultLight.classList.add("btn-theme-active");
        }
    }
}

refreshThemeButtons();

// ----- LOG -----

// Helper: convert a level name to an index (higher = more verbose)
function levelIndex(lv) {
    if (!lv) return 0;
    const levels = { minimal: 0, debug: 1, trace: 2 };
    return levels[lv.toString().toLowerCase()] ?? 0;
}

// Decide if an entry should be shown with the current logLevel
function shouldDisplayLogEntry(entry) {
    // Always show errors and warnings
    if (entry.type === "error" || entry.type === "warning") return true;
    return levelIndex(logLevel) >= levelIndex(entry.level || "minimal");
}

function renderLogs() {
    if (!logPanel) return;

    const visible = logs.filter(shouldDisplayLogEntry);

    logPanel.innerHTML = visible
        .map(l => `<div class="log-entry log-${l.type}">[${l.time}] ${l.msg}</div>`)
        .join("");

    logPanel.scrollTop = logPanel.scrollHeight;
}

function addLog(type, msg, level = "minimal") {
    // Ignore empty or whitespace-only messages
    if (!msg || (typeof msg === 'string' && msg.trim() === '')) {
        return;
    }
    
    const time = new Date().toLocaleTimeString();
    logs.push({ type, msg, time, level });

    // Keep size limit (50 entries)
    if (logs.length > 50) logs.shift();

    // Render (filtering will be applied in renderLogs)
    renderLogs();
}

// ----- TABS -----

if (tabButtons.length) {
    tabButtons.forEach(btn => {
        btn.addEventListener("click", () => {
            tabButtons.forEach(b => b.classList.remove("active"));
            tabContents.forEach(c => c.classList.remove("active"));
            btn.classList.add("active");
            const tabId = btn.dataset.tab;
            const tabEl = document.getElementById(tabId);
            if (tabEl) tabEl.classList.add("active");
        });
    });
}

// ----- UTILS -----

function decodeName(name) {
    try {
        const decoded = decodeURIComponent(name);
        return decoded !== name ? decoded : name;
    } catch {
        return name;
    }
}

function escapeHtml(unsafe) {
    if (unsafe === null || unsafe === undefined) return '';
    return String(unsafe)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

// ----- MOBILE TOOLTIP FOR TRUNCATED TEXT -----

let activeTooltip = null;

function showMobileTooltip(text, targetElement) {
    // Remove any existing tooltip
    if (activeTooltip) {
        activeTooltip.remove();
        activeTooltip = null;
    }
    
    if (!text) return;
    
    const tooltip = document.createElement('div');
    tooltip.className = 'mobile-tooltip';
    tooltip.textContent = text;
    document.body.appendChild(tooltip);
    activeTooltip = tooltip;
    
    // Position tooltip near the target element
    const rect = targetElement.getBoundingClientRect();
    const tooltipRect = tooltip.getBoundingClientRect();
    
    // Center horizontally, position below target
    let left = rect.left + (rect.width / 2) - (tooltipRect.width / 2);
    let top = rect.bottom + 8;
    
    // Keep tooltip within viewport
    const padding = 8;
    if (left < padding) left = padding;
    if (left + tooltipRect.width > window.innerWidth - padding) {
        left = window.innerWidth - tooltipRect.width - padding;
    }
    
    // If tooltip would be below viewport, show above target
    if (top + tooltipRect.height > window.innerHeight - padding) {
        top = rect.top - tooltipRect.height - 8;
    }
    
    tooltip.style.left = left + 'px';
    tooltip.style.top = top + 'px';
    
    // Auto-hide after 3 seconds or on next tap
    const hideTimeout = setTimeout(() => {
        if (activeTooltip === tooltip) {
            tooltip.remove();
            activeTooltip = null;
        }
    }, 3000);
    
    // Hide on next tap anywhere (use requestAnimationFrame for better timing)
    const hideOnTap = (e) => {
        if (!tooltip.contains(e.target)) {
            clearTimeout(hideTimeout);
            tooltip.remove();
            activeTooltip = null;
            document.removeEventListener('touchstart', hideOnTap);
        }
    };
    
    requestAnimationFrame(() => {
        document.addEventListener('touchstart', hideOnTap);
    });
}

function setupTooltipForElement(element) {
    if (!element) return;
    
    const title = element.getAttribute('title');
    if (!title) return;
    
    // Check if text is truncated (with small threshold to avoid minor differences)
    const isTruncated = element.scrollWidth > element.clientWidth + 5;
    
    if (isTruncated) {
        // On mobile, handle tap to show tooltip
        element.addEventListener('touchstart', (e) => {
            e.stopPropagation();
            showMobileTooltip(title, element);
        });
    }
}

function setupTooltipsForTable(tableBody) {
    if (!tableBody) return;
    
    // Setup tooltips for all cells with title attribute
    tableBody.querySelectorAll('td[title]').forEach(cell => {
        setupTooltipForElement(cell);
    });
    
    // Also setup for path items in details
    document.querySelectorAll('.path-item[title]').forEach(item => {
        setupTooltipForElement(item);
    });
}

// ---------- EXPLORATEUR ----------

function updateBreadcrumb(path) {
    currentPath = path;
    if (!breadcrumbEl) return;

    const parts = path.split("/").filter(p => p);
    let html = `<a data-path="/">${ROOT_LABEL}</a>`;
    let current = "";

    for (const p of parts) {
        current += "/" + p;
        html += ` › <a data-path="${escapeHtml(current)}">${escapeHtml(decodeName(p))}</a>`;
    }

    breadcrumbEl.innerHTML = html;

    breadcrumbEl.querySelectorAll("a").forEach(a => {
        a.onclick = (e) => {
            e.preventDefault();
            // Reset details when navigating via breadcrumb
            if (detailsEl) detailsEl.innerHTML = "Sélectionne un fichier pour voir les détails de hardlinks.";
            loadFolder(a.dataset.path);
        };
    });
}

async function loadFolder(path) {
    if (!explorerTableBody) return;
    if (isLoadingFolder) return;
    isLoadingFolder = true;

    explorerTableBody.innerHTML = `
        <tr><td colspan="4">
            <span class="spinner"></span> Chargement…
        </td></tr>
    `;
    updateBreadcrumb(path);
    addLog("info", `Ouverture du dossier (explorateur) : ${path}`);

    try {
        const res = await fetch("/api/list?path=" + encodeURIComponent(path));
        if (!res.ok) throw new Error("Erreur HTTP " + res.status);
        const data = await res.json();

        explorerTableBody.innerHTML = "";

        // Handle empty folders (ensure entries is an array)
        const entries = data.entries || [];
        entries.forEach(e => {
            const tr = document.createElement("tr");
            tr.classList.add("fb-row");
            tr.dataset.name = e.name;
            tr.dataset.path = e.path;
            tr.dataset.isDir = e.is_dir ? "1" : "0";
            tr.dataset.nlink = e.nlink;

            const icon = e.is_dir ? "📁" : "📄";
            const hlBadge = e.nlink > 1 ? `<span class="badge">${e.nlink}</span>` : "";
            
            // Add visual indicator for deletable hardlinks in delete mode
            let deleteIndicator = "";
            if (!e.is_dir && e.nlink > 1) {
                tr.classList.add("deletable-hardlink");
            }

            tr.innerHTML = `
                <td>${icon}</td>
                <td title="${escapeHtml(decodeName(e.name))}">${escapeHtml(decodeName(e.name))}</td>
                <td>${hlBadge}</td>
                <td>${escapeHtml(e.size_human || "")}</td>
            `;

            tr.addEventListener("click", () => onRowClickExplorer(e, tr));

            explorerTableBody.appendChild(tr);
        });

        addLog("success", `Dossier chargé (explorateur) : ${path}`);
        
        // Setup tooltips for truncated text on mobile
        setupTooltipsForTable(explorerTableBody);
    } catch (err) {
        explorerTableBody.innerHTML = `<tr><td colspan="4">Erreur de chargement</td></tr>`;
        addLog("error", `Erreur chargement dossier : ${err.message}`);
    } finally {
        isLoadingFolder = false;
    }
}

function onRowClickExplorer(entry, rowEl) {
    // In delete mode, handle deletion for both empty directories and hardlinks
    if (explorerDeleteMode) {
        if (entry.is_dir) {
            // Try to delete directory (backend will check if it's empty)
            showConfirmModal(
                "⚠️ Supprimer ce dossier ?",
                `Dossier : ${entry.name}\n\nSeuls les dossiers vides peuvent être supprimés.\nCette action est irréversible.`,
                async () => {
                    try {
                        const res = await fetch("/api/delete-hardlink", {
                            method: "POST",
                            headers: { "Content-Type": "application/json" },
                            body: JSON.stringify({ path: entry.path })
                        });
                        const data = await res.json();
                        
                        if (!data.ok) {
                            throw new Error(data.error || "Erreur inconnue");
                        }
                        
                        addLog("success", `Dossier vide supprimé : ${entry.path}`);
                        showModal("success", "Suppression réussie", `Le dossier vide a été supprimé.`);
                        
                        // Reload current folder
                        loadFolder(currentPath);
                    } catch (err) {
                        addLog("error", `Erreur suppression : ${err.message}`);
                        showModal("error", "Erreur de suppression", err.message);
                    }
                }
            );
            return;
        }
        
        // File deletion - check if it's a hardlink
        if (entry.nlink > 1) {
            showConfirmModal(
                "⚠️ Supprimer ce hardlink ?",
                `Fichier : ${entry.name}\nCe hardlink sera supprimé, mais le fichier restera accessible via ${entry.nlink - 1} autre(s) emplacement(s).\n\nCette action est irréversible.`,
                async () => {
                    try {
                        const res = await fetch("/api/delete-hardlink", {
                            method: "POST",
                            headers: { "Content-Type": "application/json" },
                            body: JSON.stringify({ path: entry.path })
                        });
                        const data = await res.json();
                        
                        if (!data.ok) {
                            throw new Error(data.error || "Erreur inconnue");
                        }
                        
                        addLog("success", `Hardlink supprimé : ${entry.path} (${data.remaining_links} liens restants)`);
                        showModal("success", "Suppression réussie", `Le hardlink a été supprimé.\n${data.remaining_links} lien(s) restant(s) vers ce fichier.`);
                        
                        // Reload current folder
                        loadFolder(currentPath);
                    } catch (err) {
                        addLog("error", `Erreur suppression : ${err.message}`);
                        showModal("error", "Erreur de suppression", err.message);
                    }
                }
            );
            return;
        }
        
        // If in delete mode but file is original (nlink <= 1)
        showModal("warning", "Suppression impossible", "Ce fichier est l'original (dernier lien).\nLa suppression du fichier original n'est pas autorisée en mode suppression de hardlinks.");
        return;
    }
    
    // Not in delete mode - normal behavior
    if (entry.is_dir) {
        // Reset details when clicking on folder
        if (detailsEl) detailsEl.innerHTML = "Sélectionne un fichier pour voir les détails de hardlinks.";
        loadFolder(entry.path);
        return;
    }

    // Normal selection behavior for files
    document.querySelectorAll(".fb-row").forEach(r => r.classList.remove("selected"));
    rowEl.classList.add("selected");
    currentSelection = entry.path;

    loadDetails(entry);
}

async function loadDetails(entry) {
    if (!detailsEl) return;

    detailsEl.innerHTML = `
        <div style="display:flex;align-items:center;gap:8px;">
            <span class="spinner"></span>
            <span>Chargement des détails…</span>
        </div>
    `;

    try {
        const res = await fetch("/api/details?path=" + encodeURIComponent(entry.path));
        const data = await res.json();

        // FIX BUG #2: Filter out the current path from "Autres emplacements"
        const otherPaths = (data.all_paths || []).filter(p => p !== data.path);
        const pathsHtml = otherPaths.map(p => {
            return `<div class="path-item" title="${escapeHtml(p)}">- ${escapeHtml(p)}</div>`;
        }).join("");

        detailsEl.innerHTML = `
            <div style="font-size:13px;"><strong>Fichier :</strong> ${escapeHtml(decodeName(data.name))}</div>
            <div style="font-size:13px;"><strong>Taille :</strong> ${escapeHtml(data.size_human)}</div>
            <div style="font-size:13px;"><strong>Inode :</strong> ${escapeHtml(data.inode)}</div>
            <div style="font-size:13px;"><strong>Liens :</strong> ${escapeHtml(data.nlink)}</div>
            <div style="font-size:13px;margin-top:6px;"><strong>Emplacement :</strong></div>
            <div style="font-size:13px;padding-left:8px;">${escapeHtml(data.path)}</div>
            ${otherPaths.length > 0 ? `
                <div style="font-size:13px;margin-top:6px;"><strong>Autres emplacements :</strong></div>
                <div class="paths-list" style="font-size:13px;">
                    ${pathsHtml}
                </div>
            ` : ''}
        `;

        addLog("info", `Détails chargés pour : ${data.path}`);
        
        // Setup tooltips for path items
        setupTooltipsForTable(detailsEl);
    } catch (err) {
        detailsEl.innerHTML = "Erreur de chargement.";
        addLog("error", `Erreur détails : ${err.message}`);
    }
}

// Search
if (searchInput) {
    searchInput.addEventListener("input", e => {
        const q = e.target.value.toLowerCase();
        document.querySelectorAll(".fb-row").forEach(row => {
            const name = decodeName(row.dataset.name).toLowerCase();
            row.style.display = name.includes(q) ? "" : "none";
        });
    });
}

// Delete mode toggle
if (explorerDeleteToggle) {
    explorerDeleteToggle.addEventListener("click", () => {
        explorerDeleteMode = !explorerDeleteMode;
        
        if (explorerDeleteMode) {
            explorerDeleteToggle.classList.remove("btn-secondary");
            explorerDeleteToggle.classList.add("btn-danger");
            explorerDeleteToggle.textContent = "✓ Mode suppression actif";
            if (explorerDeleteStatus) explorerDeleteStatus.style.display = "inline";
            document.body.classList.add("delete-mode-active");
            addLog("warning", "Mode suppression activé - Seuls les hardlinks peuvent être supprimés", "minimal");
        } else {
            explorerDeleteToggle.classList.remove("btn-danger");
            explorerDeleteToggle.classList.add("btn-secondary");
            explorerDeleteToggle.textContent = "🗑️ Mode suppression";
            if (explorerDeleteStatus) explorerDeleteStatus.style.display = "none";
            document.body.classList.remove("delete-mode-active");
            addLog("info", "Mode suppression désactivé", "minimal");
        }
    });
}

// ---------- HARDLINK CREATOR ----------

function updateBreadcrumbGeneric(container, path, onClick) {
    if (!container) return;
    const parts = path.split("/").filter(p => p);
    let html = `<a data-path="/">${ROOT_LABEL}</a>`;
    let current = "";

    for (const p of parts) {
        current += "/" + p;
        html += ` › <a data-path="${escapeHtml(current)}">${escapeHtml(decodeName(p))}</a>`;
    }

    container.innerHTML = html;

    // Check if we're in multi mode for source breadcrumb
    const isMultiMode = hlSelectionMode === "multi" && container.id === "hl-src-breadcrumb";
    
    if (isMultiMode) {
        container.classList.add("disabled");
    } else {
        container.classList.remove("disabled");
    }

    container.querySelectorAll("a").forEach(a => {
        a.onclick = (e) => {
            e.preventDefault();
            if (isMultiMode) {
                showModal("warning", "Navigation impossible", "Navigation impossible en mode Multi.\nVeuillez repasser en mode Single pour naviguer dans l'arborescence.");
                return;
            }
            onClick(a.dataset.path);
        };
    });
}

async function loadHlFolder(path, isSource) {
    const tbody = isSource ? hlSrcTableBody : hlDestTableBody;
    const breadcrumb = isSource ? hlSrcBreadcrumb : hlDestBreadcrumb;
    if (!tbody) return;

    tbody.innerHTML = `
        <tr><td colspan="4">
            <span class="spinner"></span> Chargement…
        </td></tr>
    `;

    if (isSource) hlSrcPath = path;
    else hlDestPath = path;

    updateBreadcrumbGeneric(breadcrumb, path, (p) => loadHlFolder(p, isSource));

    try {
        const res = await fetch("/api/list?path=" + encodeURIComponent(path));
        const data = await res.json();

        tbody.innerHTML = "";

        // Handle empty folders (ensure entries is an array)
        const entries = data.entries || [];
        entries.forEach(e => {
            const tr = document.createElement("tr");
            tr.dataset.name = e.name;
            tr.dataset.path = e.path;
            tr.dataset.isDir = e.is_dir ? "1" : "0";

            const icon = e.is_dir ? "📁" : "📄";
            const hlBadge = e.nlink > 1 ? `<span class="badge">${e.nlink}</span>` : "";

            if (isSource) {
                tr.innerHTML = `
                    <td class="col-select">
                        <input type="checkbox" class="hl-src-checkbox" style="display:none;">
                    </td>
                    <td title="${escapeHtml(decodeName(e.name))}">${icon} ${escapeHtml(decodeName(e.name))}</td>
                    <td>${hlBadge}</td>
                    <td>${escapeHtml(e.size_human || "")}</td>
                `;
                tr.addEventListener("click", (ev) => onRowClickHlSource(e, tr, ev));
                tr.addEventListener("dblclick", (ev) => onRowDblClickHlSource(e, tr, ev));
            } else {
                tr.innerHTML = `
                    <td>${icon}</td>
                    <td title="${escapeHtml(decodeName(e.name))}">${escapeHtml(decodeName(e.name))}</td>
                    <td>${hlBadge}</td>
                    <td>${escapeHtml(e.size_human || "")}</td>
                `;
                tr.addEventListener("click", () => onRowClickHlDest(e, tr));
            }

            tbody.appendChild(tr);
        });

        // si on est en mode multi, on ré-affiche les cases à cocher
        if (isSource && hlSelectionMode === "multi" && hlSrcTableBody) {
            hlSrcTableBody.querySelectorAll(".hl-src-checkbox").forEach(cb => cb.style.display = "block");
        }

        refreshHlSelectionDisplay();
        
        // Setup tooltips for truncated text
        setupTooltipsForTable(tbody);
    } catch (err) {
        tbody.innerHTML = `<tr><td colspan="4">Erreur</td></tr>`;
        addLog("error", `Erreur chargement HL : ${err.message}`);
    }
}

function clearSourceSelection() {
    hlSrcSelectedPaths = [];
    if (hlSrcSelectedEl) hlSrcSelectedEl.textContent = "(aucune)";
    if (hlSrcTableBody) {
        hlSrcTableBody.querySelectorAll("tr").forEach(tr => {
            tr.classList.remove("selected");
            const cb = tr.querySelector(".hl-src-checkbox");
            if (cb) cb.checked = false;
        });
    }
}

function refreshHlSelectionDisplay() {
    if (!hlSrcSelectedEl) return;
    
    const count = hlSrcSelectedPaths.length;
    
    // Update selection text
    if (count === 0) {
        hlSrcSelectedEl.textContent = "(aucune)";
    } else if (count === 1) {
        hlSrcSelectedEl.textContent = hlSrcSelectedPaths[0].path;
    } else {
        hlSrcSelectedEl.textContent = `${count} éléments sélectionnés`;
    }
    
    // Update button text with singular/plural
    if (hlBtnCreate) {
        if (count === 0) {
            hlBtnCreate.textContent = "Créer les hardlinks";
        } else if (count === 1) {
            hlBtnCreate.textContent = "Créer le hardlink";
        } else {
            hlBtnCreate.textContent = `Créer ${count} hardlinks`;
        }
    }
}

function onRowClickHlSource(entry, rowEl, ev) {
    const isDir = !!entry.is_dir;

    // clic direct sur la checkbox : on utilise son état, on ne l'inverse pas
    if (ev.target && ev.target.classList.contains("hl-src-checkbox")) {
        const cb = ev.target;
        const existingIndex = hlSrcSelectedPaths.findIndex(x => x.path === entry.path);
        if (cb.checked) {
            if (existingIndex === -1) {
                hlSrcSelectedPaths.push({ path: entry.path, isDir });
            }
            rowEl.classList.add("selected");
        } else {
            if (existingIndex !== -1) {
                hlSrcSelectedPaths.splice(existingIndex, 1);
            }
            rowEl.classList.remove("selected");
        }
        refreshHlSelectionDisplay();
        ev.stopPropagation();
        return;
    }

    if (hlSelectionMode === "single") {
        hlSrcSelectedPaths = [{ path: entry.path, isDir }];
        if (hlSrcTableBody) {
            hlSrcTableBody.querySelectorAll("tr").forEach(tr => tr.classList.remove("selected"));
        }
        rowEl.classList.add("selected");
        refreshHlSelectionDisplay();
    } else {
        const cb = rowEl.querySelector(".hl-src-checkbox");
        if (!cb) return;
        cb.checked = !cb.checked;

        const existingIndex = hlSrcSelectedPaths.findIndex(x => x.path === entry.path);
        if (cb.checked) {
            if (existingIndex === -1) {
                hlSrcSelectedPaths.push({ path: entry.path, isDir });
            }
            rowEl.classList.add("selected");
        } else {
            if (existingIndex !== -1) {
                hlSrcSelectedPaths.splice(existingIndex, 1);
            }
            rowEl.classList.remove("selected");
        }
        refreshHlSelectionDisplay();
    }
}

function onRowDblClickHlSource(entry, rowEl, ev) {
    ev.stopPropagation();
    if (!entry.is_dir) return;

    if (hlSelectionMode === "multi") {
        showConfirmModal(
            "⚠️ Navigation en mode Multi",
            "En naviguant, vous allez perdre votre sélection actuelle.\nVoulez-vous continuer et repasser en mode Single ?",
            () => {
                // Switch back to single mode
                hlSelectionMode = "single";
                const opts = hlSelectMode.querySelectorAll(".switch-option");
                opts.forEach(o => o.classList.remove("switch-option-active"));
                opts[0].classList.add("switch-option-active");
                hlSelectMode.classList.remove("mode-multi");
                
                // Hide checkboxes
                if (hlSrcTableBody) {
                    hlSrcTableBody.querySelectorAll(".hl-src-checkbox").forEach(cb => cb.style.display = "none");
                }
                
                // Clear selection
                clearSourceSelection();
                
                // Navigate
                loadHlFolder(entry.path, true);
            }
        );
        return;
    }

    loadHlFolder(entry.path, true);
}

function onRowClickHlDest(entry, rowEl) {
    if (!entry.is_dir) return;
    hlDestSelectedPath = entry.path;
    if (hlDestSelectedEl) hlDestSelectedEl.textContent = entry.path;
    loadHlFolder(entry.path, false);
}

// Folder name validation for Synology/DSM compatibility
function validateFolderName(name) {
    // Trim whitespace
    const trimmed = name.trim();
    
    // Check if empty after trim
    if (!trimmed) {
        return { valid: false, error: "Le nom du dossier ne peut pas être vide." };
    }
    
    // Check for . and ..
    if (trimmed === "." || trimmed === "..") {
        return { valid: false, error: "Les noms '.' et '..' ne sont pas autorisés." };
    }
    
    // Check length (max 255 characters)
    if (trimmed.length > 255) {
        return { valid: false, error: "Le nom du dossier est trop long (maximum 255 caractères)." };
    }
    
    // Check for forbidden characters: \ / : * ? " < > | and control characters
    const forbiddenChars = /[\\/:*?"<>|\x00-\x1F\x7F]/;
    if (forbiddenChars.test(trimmed)) {
        return { 
            valid: false, 
            error: "Le nom contient des caractères interdits.\nCaractères non autorisés : \\ / : * ? \" < > | et caractères de contrôle." 
        };
    }
    
    return { valid: true, name: trimmed };
}

async function createDestFolder() {
    showPromptModal(
        "Nouveau dossier",
        "Nom du nouveau dossier :",
        "",
        async (name) => {
            if (!name) return;

            // Validate folder name (frontend validation for UX)
            const validation = validateFolderName(name);
            if (!validation.valid) {
                showModal("error", "Nom invalide", validation.error);
                addLog("error", `Nom de dossier invalide : ${validation.error}`, "minimal");
                return;
            }

            try {
                const res = await fetch("/api/create-folder", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ parent: hlDestPath, name: validation.name })
                });
                const data = await res.json();
                if (!data.ok) throw new Error(data.error || "Erreur inconnue");

                addLog("success", `Dossier créé : ${hlDestPath}/${validation.name}`);
                loadHlFolder(hlDestPath, false);
            } catch (err) {
                addLog("error", `Erreur création dossier : ${err.message}`);
                showModal("error", "Erreur de création", err.message);
            }
        }
    );
}

async function createHardlinks() {
    if (hlSrcSelectedPaths.length === 0) {
        showModal("error", "Sélection requise", "Veuillez sélectionner au moins un fichier ou dossier source.");
        return;
    }

    // FIX: Use hlDestSelectedPath instead of hlDestPath
    const destination = hlDestSelectedPath || "/";
    
    console.log("[DEBUG] Destination utilisée:", destination);
    addLog("info", `📍 Destination: ${destination}`, "debug");

    const items = hlSelectionMode === "single"
        ? hlSrcSelectedPaths
        : hlSrcSelectedPaths.slice();

    let totalCreated = 0;
    let errors = [];

    addLog("info", `Début de création de hardlinks pour ${items.length} élément(s)...`, "debug");

    // Process items sequentially
    async function processItem(index) {
        if (index >= items.length) {
            // All items processed
            const resultMsg = `Création de hardlinks terminée : ${totalCreated} créés` +
                (errors.length ? `, erreurs: ${errors.length}` : "");
            
            addLog(
                errors.length ? "error" : "success",
                resultMsg,
                "minimal"
            );

            if (errors.length > 0) {
                const errorDetails = errors.join("\n\n");
                showModal("warning", "Création terminée avec des erreurs", 
                    `Créés : ${totalCreated}\nErreurs : ${errors.length}\n\nDétails:\n${errorDetails.substring(0, 500)}${errorDetails.length > 500 ? '...' : ''}`);
            } else {
                showModal("success", "Création réussie", `Hardlinks créés : ${totalCreated}\n\nTon système est maintenant plus optimisé !`);
            }
            
            // Clear selection and reset
            clearSourceSelection();
            hlDestSelectedPath = hlDestPath;
            
            // Reload destination folder
            console.log("🔄 Rechargement du dossier destination:", destination);
            loadHlFolder(destination, false);
            return;
        }
        
        const item = items[index];
        
        if (item.isDir) {
            const src = item.path;
            const srcName = src.split("/").pop() || "";
            const destRoot = (destination === "/" ? "" : destination) + "/" + srcName;

            console.log("📁 Dossier:", src, "→", destRoot);
            addLog("info", `Traitement du dossier : ${src} → ${destRoot}`, "trace");

            showConfirmModal(
                "Créer des hardlinks",
                `Créer des hardlinks pour tout le dossier :\n${src}\nvers :\n${destRoot} ?`,
                async () => {
                    try {
                        addLog("info", `Envoi de la requête pour le dossier ${src}...`, "trace");
                        const res = await fetch("/api/create-hardlinks-folder", {
                            method: "POST",
                            headers: { "Content-Type": "application/json" },
                            body: JSON.stringify({ source: src, dest_root: destRoot })
                        });
                        const data = await res.json();
                        
                        console.log("✅ Réponse API:", data);
                        addLog("info", `Réponse reçue pour ${src}: ${JSON.stringify(data)}`, "trace");
                        
                        if (!data.ok) throw new Error(data.error || "Erreur inconnue");
                        
                        totalCreated += data.created || 0;
                        
                        if (data.errors && data.errors.length > 0) {
                            addLog("error", `Avertissements pour ${srcName}: ${data.errors.length} erreur(s)`, "debug");
                            data.errors.forEach(err => {
                                addLog("error", `  - ${err}`, "trace");
                            });
                        }
                        
                        addLog("success", `Dossier traité : ${srcName}, ${data.created} hardlinks créés`, "debug");
                    } catch (err) {
                        const errorMsg = `Erreur sur ${srcName}: ${err.message}`;
                        errors.push(errorMsg);
                        addLog("error", errorMsg, "minimal");
                        console.error("❌ Erreur:", err);
                    }
                    
                    // Process next item
                    processItem(index + 1);
                },
                () => {
                    addLog("info", `Dossier ignoré par l'utilisateur : ${src}`, "debug");
                    // Process next item
                    processItem(index + 1);
                }
            );
        } else {
            const src = item.path;
            const srcName = src.split("/").pop();
            const dest = (destination === "/" ? "" : destination) + "/" + srcName;

            console.log("📄 Fichier:", src, "→", dest);
            addLog("info", `Traitement du fichier : ${src} → ${dest}`, "trace");

            try {
                const res = await fetch("/api/create-hardlink", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ source: src, dest })
                });
                const data = await res.json();
                
                console.log("✅ Réponse API:", data);
                addLog("info", `Réponse reçue pour ${srcName}: ${JSON.stringify(data)}`, "trace");
                
                if (!data.ok) throw new Error(data.error || "Erreur inconnue");
                totalCreated += 1;
                
                addLog("success", `Fichier traité : ${srcName}`, "debug");
            } catch (err) {
                const errorMsg = `Erreur sur ${srcName}: ${err.message}`;
                errors.push(errorMsg);
                addLog("error", errorMsg, "minimal");
                console.error("❌ Erreur:", err);
            }
            
            // Process next item
            processItem(index + 1);
        }
    }
    
    // Start processing from first item
    processItem(0);
}

if (hlBtnNewFolder) hlBtnNewFolder.addEventListener("click", createDestFolder);
if (hlBtnCreate) hlBtnCreate.addEventListener("click", createHardlinks);

// Switch Single / Multi
if (hlSelectMode) {
    hlSelectMode.addEventListener("click", (e) => {
        // Make the entire switch act as a toggle
        const newMode = hlSelectionMode === "single" ? "multi" : "single";
        
        hlSelectionMode = newMode;
        const opts = hlSelectMode.querySelectorAll(".switch-option");
        opts.forEach(o => o.classList.remove("switch-option-active"));
        
        if (newMode === "single") {
            opts[0].classList.add("switch-option-active");
            hlSelectMode.classList.remove("mode-multi");
            if (hlSrcTableBody) {
                hlSrcTableBody.querySelectorAll(".hl-src-checkbox").forEach(cb => cb.style.display = "none");
            }
            // Re-enable breadcrumb
            if (hlSrcBreadcrumb) {
                hlSrcBreadcrumb.classList.remove("disabled");
            }
            clearSourceSelection();
        } else {
            opts[1].classList.add("switch-option-active");
            hlSelectMode.classList.add("mode-multi");
            if (hlSrcTableBody) {
                hlSrcTableBody.querySelectorAll(".hl-src-checkbox").forEach(cb => cb.style.display = "block");
            }
            // Disable breadcrumb
            if (hlSrcBreadcrumb) {
                hlSrcBreadcrumb.classList.add("disabled");
            }
            clearSourceSelection();
        }
    });
}

// ---------- DOUBLONS ----------

let dupItems = [];

function resetDupDashboard() {
    if (dupCardGroups) dupCardGroups.textContent = "–";
    if (dupCardFiles) dupCardFiles.textContent = "–";
    if (dupCardSpace) dupCardSpace.textContent = "–";
}

function updateDupDashboard(stats) {
    const { totalGroups, totalDupFiles, totalWastedBytes } = stats;
    if (dupCardGroups) dupCardGroups.textContent = totalGroups.toString();
    if (dupCardFiles) dupCardFiles.textContent = totalDupFiles.toString();

    const gb = totalWastedBytes / (1024 * 1024 * 1024);
    const mb = totalWastedBytes / (1024 * 1024);
    
    let displayText = "";
    let mood = "🙂";
    
    if (gb >= 1) {
        // Display in GB
        if (gb > 10) mood = "😱";
        else if (gb > 5) mood = "😬";
        displayText = `${gb.toFixed(2)} Go ${mood}`;
    } else {
        // Display in MB
        if (mb > 500) mood = "😬";
        displayText = `${mb.toFixed(1)} Mo ${mood}`;
    }

    if (dupCardSpace) dupCardSpace.textContent = displayText;
}

async function scanDuplicates() {
    if (!dupTableBody) return;

    dupTableBody.innerHTML = `
        <tr><td colspan="4">
            <span class="spinner"></span> Scan en cours…
        </td></tr>
    `;
    if (dupSummaryEl) {
        dupSummaryEl.textContent = "Scan en cours…";
    }
    resetDupDashboard();
    
    // Hide actions container while scanning
    const actionsContainer = document.getElementById("dup-actions-container");
    if (actionsContainer) actionsContainer.style.display = "none";

    let eventSource = null;
    let timeoutId = null;

    const cleanup = () => {
        if (eventSource) {
            eventSource.close();
            eventSource = null;
        }
        if (timeoutId) {
            clearTimeout(timeoutId);
            timeoutId = null;
        }
    };

    try {
        // Step 1: Start scan job
        const startRes = await fetch("/api/duplicates/scan");
        if (!startRes.ok) {
            throw new Error("HTTP " + startRes.status);
        }
        const startData = await startRes.json();
        const jobId = startData.job_id;
        
        if (!jobId) {
            throw new Error("No job ID returned");
        }

        addLog("info", `Scan démarré, job ID: ${jobId}`, "debug");

        // Step 2: Subscribe to progress via SSE
        addLog("info", `Connexion SSE à /api/duplicates/progress?job_id=${jobId}`, "trace");
        eventSource = new EventSource(`/api/duplicates/progress?job_id=${jobId}`);
        
        // Log EventSource state changes
        eventSource.addEventListener('open', () => {
            addLog("info", `EventSource connecté (readyState=${eventSource.readyState})`, "trace");
        });
        
        eventSource.onmessage = (event) => {
            try {
                addLog("info", `SSE message reçu, taille=${event.data.length} bytes`, "trace");
                const progress = JSON.parse(event.data);
                
                // Update UI with progress
                if (progress.status === "running") {
                    const percent = progress.total_files > 0 
                        ? Math.round((progress.processed / progress.total_files) * 100) 
                        : 0;
                    dupTableBody.innerHTML = `
                        <tr><td colspan="4">
                            <span class="spinner"></span> Scan en cours… ${percent}% (${progress.processed}/${progress.total_files} fichiers analysés)
                        </td></tr>
                    `;
                    if (dupSummaryEl) {
                        dupSummaryEl.textContent = `Scan en cours… ${progress.groups_found} groupes trouvés`;
                    }
                    addLog("info", `Progrès: ${percent}% (${progress.processed}/${progress.total_files})`, "trace");
                } else if (progress.status === "completed") {
                    cleanup();
                    
                    // Display results
                    dupItems = progress.results || [];
                    displayScanResults();
                    
                    addLog("info", `Scan doublons terminé : ${dupItems.length} groupes trouvés.`);
                } else if (progress.status === "failed") {
                    cleanup();
                    throw new Error(progress.error || "Scan failed");
                }
            } catch (parseErr) {
                console.error("Failed to parse progress:", parseErr);
                addLog("info", "Erreur de parsing des progrès du scan", "debug");
            }
        };

        eventSource.onerror = (err) => {
            console.error("SSE error:", err);
            addLog("info", `EventSource erreur (readyState=${eventSource.readyState}, type=${err.type})`, "trace");
            cleanup();
            
            addLog("info", "SSE non disponible, basculement sur le mode polling...", "trace");
            
            // Fallback to polling with exponential backoff
            let pollAttempts = 0;
            const maxPollAttempts = 60; // ~6-10 minutes total with exponential backoff
            const basePollDelay = 2000; // Start with 2 seconds
            const startTime = Date.now();
            const globalTimeout = 10 * 60 * 1000; // 10 minutes
            
            addLog("info", "Démarrage du mode polling avec backoff exponentiel", "debug");
            
            const pollResults = () => {
                pollAttempts++;
                const elapsed = Date.now() - startTime;
                
                // Check global timeout
                if (elapsed > globalTimeout) {
                    if (timeoutId) {
                        clearTimeout(timeoutId);
                        timeoutId = null;
                    }
                    dupTableBody.innerHTML = `<tr><td colspan="4">Le scan a pris trop de temps. Réessaye.</td></tr>`;
                    if (dupSummaryEl) {
                        dupSummaryEl.textContent = "Timeout lors du scan.";
                    }
                    resetDupDashboard();
                    addLog("error", `Timeout global atteint après ${Math.round(elapsed/1000)}s`, "debug");
                    return;
                }
                
                addLog("info", `Polling tentative ${pollAttempts}/${maxPollAttempts}`, "trace");
                
                fetch(`/api/duplicates/results?job_id=${jobId}`)
                    .then(res => {
                        addLog("info", `Polling réponse HTTP ${res.status}`, "trace");
                        if (res.ok) {
                            return res.json();
                        } else if (res.status === 400) {
                            // Job not completed yet, retry with backoff
                            throw new Error("Job not completed");
                        } else {
                            throw new Error(`HTTP ${res.status}`);
                        }
                    })
                    .then(data => {
                        // Success - job completed
                        dupItems = data.items || [];
                        displayScanResults();
                        addLog("info", `Scan doublons terminé : ${dupItems.length} groupes trouvés.`);
                        addLog("info", `Polling réussi après ${pollAttempts} tentatives`, "debug");
                    })
                    .catch(pollErr => {
                        if (pollErr.message === "Job not completed" && pollAttempts < maxPollAttempts) {
                            // Calculate exponential backoff delay (max 10 seconds)
                            const delay = Math.min(basePollDelay * Math.pow(1.5, pollAttempts - 1), 10000);
                            
                            addLog("info", `Job non terminé, nouvelle tentative dans ${Math.round(delay/1000)}s (backoff)`, "trace");
                            
                            // Update UI to show polling progress
                            dupTableBody.innerHTML = `
                                <tr><td colspan="4">
                                    <span class="spinner"></span> Scan en cours… (tentative ${pollAttempts}/${maxPollAttempts})
                                </td></tr>
                            `;
                            
                            // Schedule next poll
                            timeoutId = setTimeout(pollResults, delay);
                        } else {
                            // Max attempts reached or other error - clear timeout
                            if (timeoutId) {
                                clearTimeout(timeoutId);
                                timeoutId = null;
                            }
                            dupTableBody.innerHTML = `<tr><td colspan="4">Erreur de connexion. Actualise la page et réessaye.</td></tr>`;
                            if (dupSummaryEl) {
                                dupSummaryEl.textContent = "Erreur lors du scan des doublons.";
                            }
                            resetDupDashboard();
                            addLog("error", `Erreur lors du scan (polling) : ${pollErr.message}`, "debug");
                        }
                    });
            };
            
            // Start polling immediately
            pollResults();
        };

        // Timeout after 10 minutes
        timeoutId = setTimeout(() => {
            if (eventSource && eventSource.readyState !== EventSource.CLOSED) {
                cleanup();
                dupTableBody.innerHTML = `<tr><td colspan="4">Le scan a pris trop de temps. Réessaye.</td></tr>`;
                if (dupSummaryEl) {
                    dupSummaryEl.textContent = "Timeout lors du scan.";
                }
                resetDupDashboard();
                addLog("error", "Timeout lors du scan des doublons");
            }
        }, 10 * 60 * 1000);

    } catch (err) {
        cleanup();
        dupTableBody.innerHTML = `<tr><td colspan="4">Erreur lors du scan.</td></tr>`;
        if (dupSummaryEl) {
            dupSummaryEl.textContent = "Erreur lors du scan des doublons.";
        }
        resetDupDashboard();
        addLog("error", `Erreur scan doublons : ${err.message}`);
    }
}

function displayScanResults() {
    const actionsContainer = document.getElementById("dup-actions-container");
    
    if (!dupItems.length) {
        dupTableBody.innerHTML = `<tr><td colspan="4">Aucun doublon détecté.</td></tr>`;
        if (dupSummaryEl) {
            dupSummaryEl.textContent = "Aucun doublon détecté. Rien à optimiser ici 👍";
        }
        addLog("info", "Scan doublons terminé : aucun doublon.");
        return;
    }

    let totalGroups = dupItems.length;
    let totalDupFiles = 0;
    let totalWastedBytes = 0;

    dupItems.forEach(item => {
        totalDupFiles += item.others.length;
        totalWastedBytes += item.size * item.others.length;
    });

    updateDupDashboard({ totalGroups, totalDupFiles, totalWastedBytes });

    if (dupSummaryEl) {
        const gb = totalWastedBytes / (1024 * 1024 * 1024);
        const mb = totalWastedBytes / (1024 * 1024);
        let mood = "🙂";
        let sizeText = "";
        
        if (gb >= 1) {
            if (gb > 10) mood = "😱";
            else if (gb > 5) mood = "😬";
            sizeText = `${gb.toFixed(2)} Go`;
        } else {
            if (mb > 500) mood = "😬";
            sizeText = `${mb.toFixed(1)} Mo`;
        }

        dupSummaryEl.textContent =
            `${totalGroups} groupes de fichiers identiques trouvés, ${totalDupFiles} fichiers doublons, ` +
            `espace potentiellement économisable : ${sizeText} ${mood}`;
    }

    dupTableBody.innerHTML = "";
    dupItems.forEach((item, idx) => {
        const othersHtml = item.others.map(p => `- ${escapeHtml(p)}`).join("<br>");
        const tr = document.createElement("tr");
        tr.innerHTML = `
            <td class="col-select">
                <input type="checkbox" class="dup-checkbox" data-index="${idx}">
            </td>
            <td>${escapeHtml(item.size_human)}</td>
            <td title="${escapeHtml(item.master)}">${escapeHtml(item.master)}</td>
            <td title="${escapeHtml(item.others.join(' | '))}">${othersHtml}</td>
        `;
        dupTableBody.appendChild(tr);
    });

    // Show actions container after successful scan
    if (actionsContainer) actionsContainer.style.display = "block";
    
    // Setup tooltips for truncated text in duplicates table
    setupTooltipsForTable(dupTableBody);
}

async function convertDuplicates() {
    if (!dupItems.length) {
        showModal("warning", "Scan requis", "Pas de doublons chargés. Lance d'abord un scan.");
        return;
    }
    const checked = Array.from(document.querySelectorAll(".dup-checkbox"))
        .filter(cb => cb.checked)
        .map(cb => parseInt(cb.dataset.index, 10))
        .filter(i => !isNaN(i));

    if (!checked.length) {
        showModal("warning", "Sélection requise", "Sélectionne au moins un groupe de doublons.");
        return;
    }

    const groups = checked.map(i => {
        const item = dupItems[i];
        return {
            master: item.master,
            others: item.others
        };
    });

    // Calculate potential space saved
    let potentialBytesSaved = 0;
    checked.forEach(i => {
        const item = dupItems[i];
        potentialBytesSaved += item.size * item.others.length;
    });
    
    const gb = potentialBytesSaved / (1024 * 1024 * 1024);
    const mb = potentialBytesSaved / (1024 * 1024);
    const savedText = gb >= 1 ? `${gb.toFixed(2)} Go` : `${mb.toFixed(1)} Mo`;

    showConfirmModal(
        "⚠️ Attention : Opération irréversible !",
        `Convertir ${groups.length} groupes de doublons en hardlinks ?\nEspace potentiellement économisable : ${savedText}`,
        async () => {
            // Show loading overlay during conversion
            showLoadingOverlay("Conversion en cours...");
            addLog("info", "🔄 Début de la conversion des doublons...", "minimal");
            
            try {
                const res = await fetch("/api/duplicates/convert", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ groups })
                });
                const data = await res.json();
                
                // Hide loading overlay before showing result
                hideLoadingOverlay();
                
                // FIX BUG #4: Check if conversion actually succeeded
                if (!data.ok) {
                    throw new Error(data.error || "Erreur conversion doublons");
                }

                // Check if no hardlinks were created despite "ok: true"
                if (data.created === 0) {
                    let errorMsg = "Aucun hardlink n'a été créé.";
                    if (data.errors && data.errors.length > 0) {
                        errorMsg += "\n\nErreurs détectées:\n" + data.errors.slice(0, 5).join("\n");
                        if (data.errors.length > 5) {
                            errorMsg += `\n... et ${data.errors.length - 5} autres erreurs.`;
                        }
                    } else {
                        errorMsg += "\n\nVérifie les permissions et que les fichiers existent toujours.";
                    }
                    addLog("error", `Conversion échouée : 0 hardlinks créés, ${data.errors?.length || 0} erreurs`, "minimal");
                    showModal("error", "Conversion échouée", errorMsg);
                    return;
                }

                const savedGb = data.bytes_saved / (1024 * 1024 * 1024);
                const savedMb = data.bytes_saved / (1024 * 1024);
                const savedDisplay = savedGb >= 1 ? `${savedGb.toFixed(2)} Go` : `${savedMb.toFixed(1)} Mo`;

                let successMsg = `Hardlinks créés : ${data.created}\nEspace économisé : ${savedDisplay}`;
                
                // Show warnings if some failed
                if (data.errors && data.errors.length > 0) {
                    successMsg += `\n\nAvertissements : ${data.errors.length} erreur(s)`;
                    addLog("warning", `Conversion partielle : ${data.created} créés, ${data.errors.length} erreurs`, "minimal");
                    showModal("warning", "Conversion terminée avec avertissements", successMsg + "\n\nVérifie le journal pour plus de détails.");
                } else {
                    addLog("success", `🎉 Doublons convertis : ${data.created} hardlinks créés, espace économisé ${savedDisplay}`, "minimal");
                    showModal("success", "Conversion réussie", successMsg + "\n\nBravo ! Ton système est maintenant plus optimisé.");
                }

                scanDuplicates();
            } catch (err) {
                // Hide loading overlay on error
                hideLoadingOverlay();
                addLog("error", `Erreur conversion doublons : ${err.message}`, "minimal");
                showModal("error", "Erreur de conversion", `${err.message}\n\nAucune modification n'a été effectuée.`);
            }
        }
    );
}

if (btnDupScan) btnDupScan.addEventListener("click", scanDuplicates);
if (btnDupConvert) btnDupConvert.addEventListener("click", convertDuplicates);

// Select all / Deselect all for duplicates
const btnDupSelectAll = document.getElementById("btn-dup-select-all");
const btnDupDeselectAll = document.getElementById("btn-dup-deselect-all");

if (btnDupSelectAll) {
    btnDupSelectAll.addEventListener("click", () => {
        document.querySelectorAll(".dup-checkbox").forEach(cb => cb.checked = true);
        if (btnDupSelectAll) btnDupSelectAll.style.display = "none";
        if (btnDupDeselectAll) btnDupDeselectAll.style.display = "inline-flex";
    });
}

if (btnDupDeselectAll) {
    btnDupDeselectAll.addEventListener("click", () => {
        document.querySelectorAll(".dup-checkbox").forEach(cb => cb.checked = false);
        if (btnDupSelectAll) btnDupSelectAll.style.display = "inline-flex";
        if (btnDupDeselectAll) btnDupDeselectAll.style.display = "none";
    });
}

// ---------- PARAMÈTRES ----------

if (rootLabelInput) {
    rootLabelInput.value = ROOT_LABEL;
}

if (rootLabelSave) {
    rootLabelSave.addEventListener("click", () => {
        const val = rootLabelInput.value.trim();
        if (!val) return;
        ROOT_LABEL = val;
        localStorage.setItem("rootLabel", val);
        addLog("success", `Nom de racine changé en : ${val}`);
        updateBreadcrumb(currentPath);
        if (hlSrcBreadcrumb) updateBreadcrumbGeneric(hlSrcBreadcrumb, hlSrcPath, (p) => loadHlFolder(p, true));
        if (hlDestBreadcrumb) updateBreadcrumbGeneric(hlDestBreadcrumb, hlDestPath, (p) => loadHlFolder(p, false));
        if (hlDestSelectedEl && (!hlDestSelectedPath || hlDestSelectedPath === "/")) {
            hlDestSelectedEl.textContent = "(racine " + ROOT_LABEL + ")";
        }
    });
}

// Thème par défaut (boutons)
if (themeDefaultDark) {
    themeDefaultDark.addEventListener("click", () => {
        applyTheme("dark");
        refreshThemeButtons();
        addLog("info", "Thème par défaut : sombre");
    });
}
if (themeDefaultLight) {
    themeDefaultLight.addEventListener("click", () => {
        applyTheme("light");
        refreshThemeButtons();
        addLog("info", "Thème par défaut : clair");
    });
}

// Log level settings
const logLevelMinimal = document.getElementById("log-level-minimal");
const logLevelDebug = document.getElementById("log-level-debug");
const logLevelTrace = document.getElementById("log-level-trace");

function refreshLogLevelButtons() {
    if (!logLevelMinimal || !logLevelDebug || !logLevelTrace) return;
    
    logLevelMinimal.classList.remove("btn-theme-active");
    logLevelDebug.classList.remove("btn-theme-active");
    logLevelTrace.classList.remove("btn-theme-active");
    
    if (logLevel === "minimal") {
        logLevelMinimal.classList.add("btn-theme-active");
    } else if (logLevel === "debug") {
        logLevelDebug.classList.add("btn-theme-active");
    } else if (logLevel === "trace") {
        logLevelTrace.classList.add("btn-theme-active");
    }
}

if (logLevelMinimal) {
    logLevelMinimal.addEventListener("click", () => {
        logLevel = "minimal";
        localStorage.setItem("logLevel", "minimal");
        refreshLogLevelButtons();
        addLog("info", "Niveau de log : Minimal");
    });
}

if (logLevelDebug) {
    logLevelDebug.addEventListener("click", () => {
        logLevel = "debug";
        localStorage.setItem("logLevel", "debug");
        refreshLogLevelButtons();
        addLog("info", "Niveau de log : Debug");
    });
}

if (logLevelTrace) {
    logLevelTrace.addEventListener("click", () => {
        logLevel = "trace";
        localStorage.setItem("logLevel", "trace");
        refreshLogLevelButtons();
        addLog("info", "Niveau de log : Trace");
    });
}

refreshLogLevelButtons();

// ----- INIT -----

document.addEventListener("DOMContentLoaded", () => {
    // Setup mobile tooltips
    setupMobileTooltips();
    
    if (explorerTableBody) loadFolder("/");
    if (hlSrcTableBody && hlDestTableBody) {
        loadHlFolder("/", true);
        loadHlFolder("/", false);
        if (hlDestSelectedEl) hlDestSelectedEl.textContent = "(racine " + ROOT_LABEL + ")";
    }
    resetDupDashboard();
});