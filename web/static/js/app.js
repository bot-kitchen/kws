// KWS Application JavaScript

// Toast notifications
const Toast = {
    container: null,

    init() {
        this.container = document.getElementById('toast-container');
        if (!this.container) {
            this.container = document.createElement('div');
            this.container.id = 'toast-container';
            this.container.className = 'fixed bottom-4 right-4 z-50 space-y-2';
            document.body.appendChild(this.container);
        }
    },

    show(type, message, duration = 5000) {
        if (!this.container) this.init();

        const toast = document.createElement('div');
        toast.className = `flex items-center p-4 rounded-lg shadow-lg toast-enter max-w-sm ${this.getTypeClasses(type)}`;

        const icon = this.getIcon(type);
        toast.innerHTML = `
            <span class="material-icons-outlined mr-3">${icon}</span>
            <span class="flex-1 text-sm font-medium">${message}</span>
            <button onclick="this.parentElement.remove()" class="ml-3 hover:opacity-75">
                <span class="material-icons-outlined text-lg">close</span>
            </button>
        `;

        this.container.appendChild(toast);

        if (duration > 0) {
            setTimeout(() => {
                toast.classList.remove('toast-enter');
                toast.classList.add('toast-exit');
                setTimeout(() => toast.remove(), 300);
            }, duration);
        }
    },

    getTypeClasses(type) {
        const classes = {
            success: 'bg-green-600 text-white',
            error: 'bg-red-600 text-white',
            warning: 'bg-yellow-500 text-white',
            info: 'bg-blue-600 text-white'
        };
        return classes[type] || classes.info;
    },

    getIcon(type) {
        const icons = {
            success: 'check_circle',
            error: 'error',
            warning: 'warning',
            info: 'info'
        };
        return icons[type] || icons.info;
    }
};

// Global toast function
function showToast(type, message, duration) {
    Toast.show(type, message, duration);
}

// HTMX event handlers
document.addEventListener('htmx:afterRequest', function(event) {
    const xhr = event.detail.xhr;

    if (xhr.status >= 200 && xhr.status < 300) {
        // Check for success message in response header
        const message = xhr.getResponseHeader('X-Toast-Message');
        if (message) {
            Toast.show('success', message);
        }
    } else if (xhr.status >= 400) {
        // Parse error response
        try {
            const response = JSON.parse(xhr.responseText);
            Toast.show('error', response.message || 'An error occurred');
        } catch {
            Toast.show('error', 'An error occurred');
        }
    }
});

document.addEventListener('htmx:sendError', function(event) {
    Toast.show('error', 'Network error. Please check your connection.');
});

// Confirmation dialogs
function confirmAction(message, callback) {
    if (confirm(message)) {
        callback();
    }
}

// Format relative time
function formatRelativeTime(date) {
    const now = new Date();
    const diff = now - new Date(date);

    const seconds = Math.floor(diff / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);

    if (seconds < 60) return 'just now';
    if (minutes < 60) return `${minutes}m ago`;
    if (hours < 24) return `${hours}h ago`;
    if (days < 7) return `${days}d ago`;

    return new Date(date).toLocaleDateString();
}

// Format numbers
function formatNumber(num) {
    if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M';
    if (num >= 1000) return (num / 1000).toFixed(1) + 'K';
    return num.toString();
}

// Debounce function
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

// Keyboard shortcuts
document.addEventListener('keydown', function(event) {
    // Ctrl/Cmd + K for search
    if ((event.ctrlKey || event.metaKey) && event.key === 'k') {
        event.preventDefault();
        const searchInput = document.querySelector('input[placeholder*="Search"]');
        if (searchInput) searchInput.focus();
    }

    // Escape to close modals
    if (event.key === 'Escape') {
        const modals = document.querySelectorAll('[id$="-modal"]:not(.hidden)');
        modals.forEach(modal => modal.classList.add('hidden'));
    }
});

// WebSocket for real-time updates
class WSClient {
    constructor(url) {
        this.url = url;
        this.ws = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectDelay = 1000;
        this.handlers = {};
    }

    connect() {
        try {
            this.ws = new WebSocket(this.url);

            this.ws.onopen = () => {
                console.log('WebSocket connected');
                this.reconnectAttempts = 0;
            };

            this.ws.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data);
                    this.handleMessage(data);
                } catch (e) {
                    console.error('Failed to parse WebSocket message:', e);
                }
            };

            this.ws.onclose = () => {
                console.log('WebSocket disconnected');
                this.reconnect();
            };

            this.ws.onerror = (error) => {
                console.error('WebSocket error:', error);
            };
        } catch (e) {
            console.error('Failed to connect WebSocket:', e);
            this.reconnect();
        }
    }

    reconnect() {
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
            this.reconnectAttempts++;
            const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);
            console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);
            setTimeout(() => this.connect(), delay);
        }
    }

    handleMessage(data) {
        const handler = this.handlers[data.type];
        if (handler) {
            handler(data.payload);
        }
    }

    on(type, handler) {
        this.handlers[type] = handler;
    }

    send(type, payload) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify({ type, payload }));
        }
    }
}

// Initialize WebSocket if on dashboard
if (window.location.pathname === '/' || window.location.pathname === '/dashboard') {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;

    const ws = new WSClient(wsUrl);

    ws.on('kos_status', (data) => {
        // Update KOS status in UI
        const element = document.querySelector(`[data-kos-id="${data.id}"] .status-badge`);
        if (element) {
            element.textContent = data.status;
            element.className = `status-badge ${data.status}`;
        }
    });

    ws.on('order_update', (data) => {
        // Show notification for order updates
        Toast.show('info', `Order ${data.reference} is now ${data.status}`);
    });

    ws.on('alert', (data) => {
        Toast.show(data.severity === 'critical' ? 'error' : 'warning', data.message);
    });

    // Uncomment to enable WebSocket
    // ws.connect();
}

// Initialize on DOM ready
document.addEventListener('DOMContentLoaded', function() {
    Toast.init();

    // Set up dark mode from localStorage
    if (localStorage.getItem('darkMode') === 'true') {
        document.documentElement.classList.add('dark');
    }
});
