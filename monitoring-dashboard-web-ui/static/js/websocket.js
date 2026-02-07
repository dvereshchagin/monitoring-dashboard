class MetricsWebSocket {
    constructor() {
        this.ws = null;
        this.reconnectDelay = 1000;
        this.maxReconnectDelay = 30000;
        this.charts = {};
        this.screenshotsCaptured = false;
        this.authToken = this.loadAuthToken();
        this.init();
    }

    loadAuthToken() {
        try {
            const urlParams = new URLSearchParams(window.location.search);
            const tokenFromURL = (urlParams.get('token') || '').trim();
            if (tokenFromURL) {
                window.localStorage.setItem('monitoring_auth_token', tokenFromURL);
                return tokenFromURL;
            }
            return window.localStorage.getItem('monitoring_auth_token') || '';
        } catch (_) {
            return '';
        }
    }

    init() {
        this.bootstrapAuth()
            .finally(() => {
                this.connect();
                this.initCharts();
                return this.loadHistoricalData();
            })
            .finally(() => {
                setTimeout(() => {
                    this.captureAndUploadDashboardScreenshots();
                }, 1000);
            });
    }

    async bootstrapAuth() {
        if (!this.authToken) {
            return;
        }

        try {
            const response = await fetch('/api/v1/auth/login', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ token: this.authToken })
            });

            if (!response.ok) {
                console.warn('Auth bootstrap failed:', response.status);
            }
        } catch (err) {
            console.warn('Auth bootstrap error:', err);
        }
    }

    connect() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        let url = `${protocol}//${window.location.host}/ws`;
        if (this.authToken) {
            url += `?token=${encodeURIComponent(this.authToken)}`;
        }

        console.log('Connecting to WebSocket:', url);
        this.ws = new WebSocket(url);

        this.ws.onopen = () => {
            console.log('WebSocket connected');
            this.updateConnectionStatus(true);
            this.reconnectDelay = 1000;
        };

        this.ws.onmessage = (event) => {
            const message = JSON.parse(event.data);

            if (message.type === 'snapshot') {
                this.handleSnapshot(message.data);
            } else if (message.type === 'alert') {
                this.handleAlert(message.data);
            }
        };

        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
            this.updateConnectionStatus(false);
        };

        this.ws.onclose = () => {
            console.log('WebSocket disconnected');
            this.updateConnectionStatus(false);
            this.scheduleReconnect();
        };
    }

    scheduleReconnect() {
        setTimeout(() => {
            console.log('Attempting to reconnect...');
            this.connect();
        }, this.reconnectDelay);

        this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
    }

    handleSnapshot(snapshot) {
        if (snapshot.cpu) this.updateMetric('cpu', snapshot.cpu);
        if (snapshot.memory) this.updateMetric('memory', snapshot.memory);
        if (snapshot.disk) this.updateMetric('disk', snapshot.disk);
        if (snapshot.network) this.updateMetric('network', snapshot.network);

        this.updateCharts(snapshot);
    }

    updateMetric(type, metric) {
        const valueEl = document.getElementById(`${type}-value`);
        const cardEl = document.getElementById(`${type}-card`);

        if (valueEl) {
            valueEl.textContent = metric.value.toFixed(1);
        }

        if (cardEl) {
            cardEl.classList.remove('critical', 'warning');
            if (metric.is_critical) {
                cardEl.classList.add('critical');
            } else if (metric.is_warning) {
                cardEl.classList.add('warning');
            }
        }
    }

    handleAlert(alert) {
        console.warn('Alert received:', alert);
    }

    updateConnectionStatus(connected) {
        const statusEl = document.getElementById('connection-status');
        if (statusEl) {
            if (connected) {
                statusEl.textContent = '● Connected';
                statusEl.className = 'status connected';
            } else {
                statusEl.textContent = '● Disconnected';
                statusEl.className = 'status disconnected';
            }
        }
    }

    initCharts() {
        const commonOptions = {
            responsive: true,
            maintainAspectRatio: false,
            animation: { duration: 0 },
            scales: {
                y: {
                    beginAtZero: true,
                    max: 100
                },
                x: {
                    display: false
                }
            },
            plugins: {
                legend: { display: false }
            }
        };

        this.charts.cpu = new Chart(document.getElementById('cpuChart'), {
            type: 'line',
            data: {
                labels: [],
                datasets: [{
                    label: 'CPU %',
                    data: [],
                    borderColor: 'rgb(52, 152, 219)',
                    backgroundColor: 'rgba(52, 152, 219, 0.1)',
                    tension: 0.4
                }]
            },
            options: commonOptions
        });

        this.charts.memory = new Chart(document.getElementById('memoryChart'), {
            type: 'line',
            data: {
                labels: [],
                datasets: [{
                    label: 'Memory %',
                    data: [],
                    borderColor: 'rgb(155, 89, 182)',
                    backgroundColor: 'rgba(155, 89, 182, 0.1)',
                    tension: 0.4
                }]
            },
            options: commonOptions
        });
    }

    async loadHistoricalData() {
        try {
            const cpuData = await this.fetchWithAuth('/api/v1/metrics/history?type=cpu&duration=1h');
            const cpuHistory = await cpuData.json();
            this.updateChart('cpu', cpuHistory.metrics);

            const memData = await this.fetchWithAuth('/api/v1/metrics/history?type=memory&duration=1h');
            const memHistory = await memData.json();
            this.updateChart('memory', memHistory.metrics);
        } catch (err) {
            console.error('Failed to load historical data:', err);
        }
    }

    fetchWithAuth(url, options = {}) {
        const headers = { ...(options.headers || {}) };
        if (this.authToken) {
            headers.Authorization = `Bearer ${this.authToken}`;
        }
        return fetch(url, {
            ...options,
            headers
        });
    }

    updateChart(type, metrics) {
        if (!this.charts[type] || !metrics || metrics.length === 0) return;

        const labels = metrics.map(m => new Date(m.collected_at).toLocaleTimeString());
        const data = metrics.map(m => m.value);

        this.charts[type].data.labels = labels;
        this.charts[type].data.datasets[0].data = data;
        this.charts[type].update('none');
    }

    updateCharts(snapshot) {
        const maxPoints = 60;
        const timestamp = new Date().toLocaleTimeString();

        if (snapshot.cpu) {
            this.addChartPoint('cpu', timestamp, snapshot.cpu.value, maxPoints);
        }

        if (snapshot.memory) {
            this.addChartPoint('memory', timestamp, snapshot.memory.value, maxPoints);
        }
    }

    addChartPoint(type, label, value, maxPoints) {
        if (!this.charts[type]) return;

        const chart = this.charts[type];
        chart.data.labels.push(label);
        chart.data.datasets[0].data.push(value);

        if (chart.data.labels.length > maxPoints) {
            chart.data.labels.shift();
            chart.data.datasets[0].data.shift();
        }

        chart.update('none');
    }

    async captureAndUploadDashboardScreenshots() {
        if (this.screenshotsCaptured) {
            return;
        }

        if (typeof html2canvas === 'undefined') {
            console.warn('html2canvas is not available, skipping screenshot capture');
            return;
        }

        const artifactSpecs = [
            { type: 'cpu_card', selector: '#cpu-card', source: 'dom' },
            { type: 'memory_card', selector: '#memory-card', source: 'dom' },
            { type: 'disk_card', selector: '#disk-card', source: 'dom' },
            { type: 'network_card', selector: '#network-card', source: 'dom' },
            { type: 'cpu_chart', selector: '#cpuChart', source: 'canvas' },
            { type: 'memory_chart', selector: '#memoryChart', source: 'canvas' }
        ];

        try {
            const artifacts = [];

            for (const spec of artifactSpecs) {
                const element = document.querySelector(spec.selector);
                if (!element) {
                    throw new Error(`Element not found: ${spec.selector}`);
                }

                let dataBase64;
                if (spec.source === 'canvas') {
                    dataBase64 = element.toDataURL('image/png').replace(/^data:image\/png;base64,/, '');
                } else {
                    const canvas = await html2canvas(element, {
                        backgroundColor: '#ffffff',
                        scale: 2,
                        useCORS: true,
                        logging: false
                    });
                    dataBase64 = canvas.toDataURL('image/png').replace(/^data:image\/png;base64,/, '');
                }

                artifacts.push({
                    type: spec.type,
                    content_type: 'image/png',
                    data_base64: dataBase64
                });
            }

            const response = await this.fetchWithAuth('/api/v1/screenshots/dashboard', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    dashboard_id: 'main',
                    captured_at: new Date().toISOString(),
                    artifacts
                })
            });

            if (!response.ok) {
                const errorText = await response.text();
                throw new Error(`Upload failed: ${response.status} ${errorText}`);
            }

            const payload = await response.json();
            this.screenshotsCaptured = true;
            console.log('Dashboard screenshots uploaded', payload.items);
        } catch (error) {
            console.error('Failed to capture/upload dashboard screenshots:', error);
        }
    }
}

document.addEventListener('DOMContentLoaded', () => {
    new MetricsWebSocket();
});
