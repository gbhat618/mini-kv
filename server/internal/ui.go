package internal

import (
	"html/template"
	"net/http"
)

var pageTemplate = template.Must(template.New("dashboard").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Mini-KV Dashboard</title>
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    <script src="https://cdn.tailwindcss.com"></script>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap" rel="stylesheet">
    <style>
        * { font-family: 'Inter', sans-serif; }
        .gradient-bg { background: linear-gradient(135deg, #1e1b4b 0%, #312e81 50%, #4c1d95 100%); }
        .card { @apply bg-white rounded-2xl shadow-lg border border-gray-100; }
        .stat-card { @apply bg-white rounded-2xl shadow-lg border border-gray-100 p-6; transition: all 0.3s ease; }
        .stat-card:hover { transform: translateY(-4px); box-shadow: 0 20px 40px rgba(0,0,0,0.1); }
        .pulse { animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite; }
        @keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: .5; } }
        .connection-dot { @apply w-3 h-3 rounded-full; }
        .connection-dot.connected { @apply bg-green-500; }
        .connection-dot.disconnected { @apply bg-red-500; }
        .kv-row { @apply hover:bg-gray-50 transition-colors; }
        .kv-row:hover { background: linear-gradient(90deg, #f0f9ff 0%, #e0f2fe 100%); }
        input, textarea { @apply border border-gray-300 rounded-lg px-4 py-2 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-transparent transition-all; }
        .btn { @apply px-4 py-2 rounded-lg font-medium transition-all; }
        .btn-primary { @apply bg-indigo-600 text-white hover:bg-indigo-700; }
        .btn-danger { @apply bg-red-600 text-white hover:bg-red-700; }
        .btn-success { @apply bg-green-600 text-white hover:bg-green-700; }
        .btn-secondary { @apply bg-gray-200 text-gray-700 hover:bg-gray-300; }
        .status-badge { @apply px-3 py-1 rounded-full text-sm font-medium; }
        .status-connected { @apply bg-green-100 text-green-800; }
        .status-disconnected { @apply bg-red-100 text-red-800; }
        .glow { box-shadow: 0 0 40px rgba(99, 102, 241, 0.3); }
    </style>
</head>
<body class="bg-gray-50 min-h-screen">
    <div class="gradient-bg min-h-screen pb-12">
        <!-- Header -->
        <header class="border-b border-white/10 bg-white/5 backdrop-blur-xl">
            <div class="max-w-7xl mx-auto px-6 py-4 flex items-center justify-between">
                <div class="flex items-center space-x-3">
                    <div class="w-10 h-10 bg-white/10 rounded-xl flex items-center justify-center">
                        <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2 1 3 3 3h10c2 0 3-1 3-3V7c0-2-1-3-3-3H7c-2 0-3 1-3 3z"></path>
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6M12 9v6"></path>
                        </svg>
                    </div>
                    <h1 class="text-2xl font-bold text-white">Mini-KV Console</h1>
                </div>
                <div class="flex items-center space-x-4">
                    <div class="flex items-center space-x-2 text-white/70">
                        <div id="connection-status" class="connection-dot connected"></div>
                        <span id="connection-text">Connected</span>
                    </div>
                    <span class="text-white/50 text-sm">Real-time</span>
                </div>
            </div>
        </header>

        <main class="max-w-7xl mx-auto px-6 py-8">
            <!-- Stats Cards -->
            <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
                <div class="stat-card">
                    <div class="flex items-center justify-between mb-4">
                        <div class="w-12 h-12 bg-indigo-100 rounded-xl flex items-center justify-center">
                            <svg class="w-6 h-6 text-indigo-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path>
                            </svg>
                        </div>
                        <span class="text-sm text-gray-500">Live</span>
                    </div>
                    <p class="text-4xl font-bold text-gray-900" id="connections">0</p>
                    <p class="text-gray-500 mt-1">Active Connections</p>
                </div>
                
                <div class="stat-card">
                    <div class="flex items-center justify-between mb-4">
                        <div class="w-12 h-12 bg-green-100 rounded-xl flex items-center justify-center">
                            <svg class="w-6 h-6 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
                            </svg>
                        </div>
                        <span class="text-sm text-gray-500">Total</span>
                    </div>
                    <p class="text-4xl font-bold text-gray-900" id="kv-count">0</p>
                    <p class="text-gray-500 mt-1">Key-Value Pairs</p>
                </div>

                <div class="stat-card">
                    <div class="flex items-center justify-between mb-4">
                        <div class="w-12 h-12 bg-purple-100 rounded-xl flex items-center justify-center">
                            <svg class="w-6 h-6 text-purple-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                            </svg>
                        </div>
                        <span class="text-sm text-gray-500">Updated</span>
                    </div>
                    <p class="text-4xl font-bold text-gray-900" id="last-update">--</p>
                    <p class="text-gray-500 mt-1">Last Updated</p>
                </div>
            </div>

            <!-- Actions -->
            <div class="card p-6 mb-8">
                <h2 class="text-lg font-semibold text-gray-900 mb-4">Add / Update Key</h2>
                <form id="kv-form" class="flex gap-4">
                    <input type="text" id="key-input" placeholder="Key" class="flex-1" required>
                    <input type="text" id="value-input" placeholder="Value" class="flex-1" required>
                    <button type="submit" class="btn btn-primary">
                        <span class="flex items-center gap-2">
                            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"></path>
                            </svg>
                            Save
                        </span>
                    </button>
                </form>
            </div>

            <!-- KV List -->
            <div class="card">
                <div class="p-6 border-b border-gray-100">
                    <div class="flex items-center justify-between">
                        <h2 class="text-lg font-semibold text-gray-900">Stored Keys</h2>
                        <button onclick="refreshKeys()" class="btn btn-secondary text-sm">
                            <span class="flex items-center gap-1">
                                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path>
                                </svg>
                                Refresh
                            </span>
                        </button>
                    </div>
                </div>
                <div class="divide-y divide-gray-100 max-h-96 overflow-y-auto">
                    <div id="kv-list" class="p-6">
                        <div class="text-center text-gray-500 py-8">
                            <svg class="w-12 h-12 mx-auto mb-3 text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"></path>
                            </svg>
                            <p>No keys stored yet</p>
                        </div>
                    </div>
                </div>
            </div>
        </main>
    </div>

    <script>
        let ws = null;
        let reconnectAttempts = 0;
        const maxReconnectAttempts = 10;

        function connect() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = protocol + '//' + window.location.host + '/ws';
            
            ws = new WebSocket(wsUrl);
            
            ws.onopen = function() {
                console.log('WebSocket connected');
                document.getElementById('connection-status').className = 'connection-dot connected';
                document.getElementById('connection-text').textContent = 'Connected';
                reconnectAttempts = 0;
            };
            
            ws.onmessage = function(event) {
                try {
                    const data = JSON.parse(event.data);
                    
                    if (data.type === 'stats') {
                        document.getElementById('connections').textContent = data.connections;
                        document.getElementById('kv-count').textContent = data.kv_count;
                        
                        const date = new Date(data.timestamp);
                        document.getElementById('last-update').textContent = date.toLocaleTimeString();
                    }
                    
                    if (data.type === 'keys') {
                        renderKeys(data.keys);
                    }
                } catch (e) {
                    console.error('Error parsing message:', e);
                }
            };
            
            ws.onclose = function() {
                console.log('WebSocket disconnected');
                document.getElementById('connection-status').className = 'connection-dot disconnected';
                document.getElementById('connection-text').textContent = 'Disconnected';
                
                if (reconnectAttempts < maxReconnectAttempts) {
                    reconnectAttempts++;
                    setTimeout(connect, Math.min(1000 * Math.pow(2, reconnectAttempts), 30000));
                }
            };
            
            ws.onerror = function(error) {
                console.error('WebSocket error:', error);
            };
        }

        function renderKeys(keys) {
            const container = document.getElementById('kv-list');
            
            if (!keys || keys.length === 0) {
                container.innerHTML = '<div class="text-center text-gray-500 py-8">' +
                    '<svg class="w-12 h-12 mx-auto mb-3 text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">' +
                    '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"></path>' +
                    '</svg><p>No keys stored yet</p></div>';
                return;
            }
            
            let html = '<table class="w-full"><thead><tr class="text-left text-sm text-gray-500">' +
                '<th class="pb-3 font-medium">Key</th><th class="pb-3 font-medium">Value</th>' +
                '<th class="pb-3 font-medium text-right">Actions</th></tr></thead><tbody>';
            
            keys.forEach(kv => {
                const [key, value] = kv.split(/:(.+)/);
                html += '<tr class="kv-row border-b border-gray-50">' +
                    '<td class="py-3 font-medium text-gray-900">' + escapeHtml(key) + '</td>' +
                    '<td class="py-3 text-gray-600 font-mono text-sm max-w-md truncate">' + escapeHtml(value) + '</td>' +
                    '<td class="py-3 text-right">' +
                    '<button onclick="editKey(\'' + escapeHtml(key) + '\', \'' + escapeHtml(value) + '\')" ' +
                    'class="btn btn-secondary text-sm mr-2">Edit</button>' +
                    '<button onclick="deleteKey(\'' + escapeHtml(key) + '\')" ' +
                    'class="btn btn-danger text-sm">Delete</button>' +
                    '</td></tr>';
            });
            
            html += '</tbody></table>';
            container.innerHTML = html;
        }

        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        function refreshKeys() {
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({ type: 'get_keys' }));
            }
        }

        function editKey(key, value) {
            document.getElementById('key-input').value = key;
            document.getElementById('value-input').value = value;
            document.getElementById('key-input').focus();
            window.scrollTo({ top: 0, behavior: 'smooth' });
        }

        function deleteKey(key) {
            if (confirm('Are you sure you want to delete key: ' + key + '?')) {
                if (ws && ws.readyState === WebSocket.OPEN) {
                    ws.send(JSON.stringify({ type: 'delete', key: key }));
                }
            }
        }

        document.getElementById('kv-form').addEventListener('submit', function(e) {
            e.preventDefault();
            
            const key = document.getElementById('key-input').value;
            const value = document.getElementById('value-input').value;
            
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({ type: 'set', key: key, value: value }));
                
                document.getElementById('key-input').value = '';
                document.getElementById('value-input').value = '';
            }
        });

        connect();
    </script>
</body>
</html>`))

type PageData struct {
	Port int
}

func ServeUI(port int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageTemplate.Execute(w, PageData{Port: port})
	})
}
