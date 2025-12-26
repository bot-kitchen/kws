// Dark mode toggle with localStorage persistence and system preference detection

document.addEventListener('alpine:init', () => {
  // Global notification store for sidebar badges
  Alpine.store('notifications', {
    offlineDevices: 0,
    failedOrders: 0,
    initialized: false,

    // Update counts from initial API data
    setInitialCounts(offlineDevices, failedOrders) {
      this.offlineDevices = offlineDevices;
      this.failedOrders = failedOrders;
      this.initialized = true;
    },

    // Increment/decrement methods for WebSocket events
    incrementOfflineDevices() {
      this.offlineDevices++;
    },

    decrementOfflineDevices() {
      if (this.offlineDevices > 0) this.offlineDevices--;
    },

    incrementFailedOrders() {
      this.failedOrders++;
    },

    decrementFailedOrders() {
      if (this.failedOrders > 0) this.failedOrders--;
    },

    // Reset when user visits the respective page
    clearDeviceAlerts() {
      // Keep the count but could add "seen" state if needed
    },

    clearOrderAlerts() {
      // Keep the count but could add "seen" state if needed
    }
  });

  // Status strip component for global stats display
  Alpine.data('statusStrip', () => ({
    cookingOrders: 0,
    queuedOrders: 0,
    onlineDevices: 0,
    totalDevices: 0,
    failedOrders: 0,
    offlineDevices: 0,
    wsConnected: false,
    ws: null,

    init() {
      this.fetchStats();
      this.connectWebSocket();
      // Refresh stats every 30 seconds as backup
      setInterval(() => this.fetchStats(), 30000);
    },

    async fetchStats() {
      try {
        // Fetch order counts
        const ordersRes = await fetch('/api/v1/orders');
        if (ordersRes.ok) {
          const result = await ordersRes.json();
          // API returns {success: true, data: [...]} format
          const orders = Array.isArray(result) ? result : (result.data || []);
          this.cookingOrders = orders.filter(o => o.status === 'in_progress').length;
          this.queuedOrders = orders.filter(o => ['pending', 'scheduled', 'validated'].includes(o.status)).length;
          this.failedOrders = orders.filter(o => o.status === 'failed').length;
        }

        // Fetch device counts
        const devicesRes = await fetch('/api/v1/devices');
        if (devicesRes.ok) {
          const result = await devicesRes.json();
          // API returns {success: true, data: [...]} format
          const devices = Array.isArray(result) ? result : (result.data || []);
          // Passive device types (no health monitoring)
          const passiveTypes = ['cradle', 'pot_staging', 'pot_serving', 'canister_refill_area'];
          // Only count active (non-passive) enabled devices
          const activeEnabledDevices = devices.filter(d => d.enabled && !passiveTypes.includes(d.device_type));
          this.totalDevices = activeEnabledDevices.length;
          this.onlineDevices = activeEnabledDevices.filter(d => ['online', 'busy'].includes(d.status)).length;
          this.offlineDevices = activeEnabledDevices.filter(d => ['offline', 'needs_intervention', 'failed', 'emergency'].includes(d.status)).length;
        }
      } catch (error) {
        console.error('[StatusStrip] Error fetching stats:', error);
      }
    },

    connectWebSocket() {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${protocol}//${window.location.host}/api/v1/ws/events?topics=order.status.*,device.updated,device.health.*`;

      try {
        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {
          this.wsConnected = true;
        };

        this.ws.onmessage = (event) => {
          try {
            const message = JSON.parse(event.data);
            if (message.type === 'event' && message.event) {
              this.handleEvent(message.event);
            }
          } catch (e) {
            // Ignore parse errors
          }
        };

        this.ws.onclose = () => {
          this.wsConnected = false;
          // Reconnect after 5 seconds
          setTimeout(() => this.connectWebSocket(), 5000);
        };

        this.ws.onerror = () => {
          this.wsConnected = false;
        };
      } catch (error) {
        console.error('[StatusStrip] WebSocket error:', error);
      }
    },

    handleEvent(event) {
      const eventType = event.type;
      const data = event.data || {};

      // Order events
      if (eventType && eventType.startsWith('order.status.')) {
        const status = data.current_status || data.status;
        const prevStatus = data.previous_status;

        // Update cooking count
        if (status === 'in_progress' && prevStatus !== 'in_progress') {
          this.cookingOrders++;
          if (['pending', 'scheduled', 'validated'].includes(prevStatus) && this.queuedOrders > 0) {
            this.queuedOrders--;
          }
        }
        if (prevStatus === 'in_progress' && status !== 'in_progress') {
          if (this.cookingOrders > 0) this.cookingOrders--;
        }

        // Update failed count
        if (status === 'failed' && prevStatus !== 'failed') {
          this.failedOrders++;
        }
        if (prevStatus === 'failed' && status !== 'failed') {
          if (this.failedOrders > 0) this.failedOrders--;
        }

        // Update queued count
        if (['completed', 'cancelled', 'failed'].includes(status)) {
          if (['pending', 'scheduled', 'validated'].includes(prevStatus) && this.queuedOrders > 0) {
            this.queuedOrders--;
          }
        }
      }

      // Device events - only count active (non-passive) devices
      if (eventType === 'device.updated' || eventType?.startsWith('device.health.')) {
        const passiveTypes = ['cradle', 'pot_staging', 'pot_serving', 'canister_refill_area'];
        // Skip passive devices for status strip counts
        if (passiveTypes.includes(data.device_type)) {
          return;
        }

        const status = data.current_status || data.status;
        const prevStatus = data.previous_status;
        const problemStatuses = ['offline', 'needs_intervention', 'failed', 'emergency'];
        const healthyStatuses = ['online', 'busy'];

        // Device went into problem state
        if (problemStatuses.includes(status) && !problemStatuses.includes(prevStatus)) {
          this.offlineDevices++;
          if (healthyStatuses.includes(prevStatus) && this.onlineDevices > 0) {
            this.onlineDevices--;
          }
        }

        // Device recovered
        if (healthyStatuses.includes(status) && problemStatuses.includes(prevStatus)) {
          this.onlineDevices++;
          if (this.offlineDevices > 0) this.offlineDevices--;
        }
      }
    }
  }));

  Alpine.data('theme', () => ({
    darkMode: false,
    sidebarOpen: true,

    init() {
      // Check localStorage first, then system preference
      const savedTheme = localStorage.getItem('theme');
      if (savedTheme) {
        this.darkMode = savedTheme === 'dark';
      } else {
        // Use system preference
        this.darkMode = window.matchMedia('(prefers-color-scheme: dark)').matches;
      }

      // Check sidebar state from localStorage
      const savedSidebar = localStorage.getItem('sidebarOpen');
      if (savedSidebar !== null) {
        this.sidebarOpen = savedSidebar === 'true';
      }

      // Apply theme
      this.applyTheme();

      // Listen for system theme changes
      window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
        if (!localStorage.getItem('theme')) {
          this.darkMode = e.matches;
          this.applyTheme();
        }
      });
    },

    toggleDarkMode() {
      this.darkMode = !this.darkMode;
      localStorage.setItem('theme', this.darkMode ? 'dark' : 'light');
      this.applyTheme();
    },

    toggleSidebar() {
      this.sidebarOpen = !this.sidebarOpen;
      localStorage.setItem('sidebarOpen', this.sidebarOpen);
    },

    applyTheme() {
      if (this.darkMode) {
        document.documentElement.classList.add('dark');
      } else {
        document.documentElement.classList.remove('dark');
      }
    }
  }));
});
