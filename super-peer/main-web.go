package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Peer represents a node in the P2P network
type Peer struct {
	ID       string    `json:"id"`
	Address  string    `json:"address"`
	Port     int       `json:"port"`
	LastSeen time.Time `json:"lastSeen"`
	Files    []File    `json:"files"`
}

// File represents a file in the P2P network
type File struct {
	Name    string   `json:"name"`
	Hash    string   `json:"hash"`
	Size    int64    `json:"size"`
	PeerIDs []string `json:"peerIds"`
}

// SearchRequest represents a search query from a peer
type SearchRequest struct {
	Query    string `json:"query"`
	Limit    int    `json:"limit"`
	FromPeer string `json:"fromPeer"`
}

// SearchResponse represents the response to a search query
type SearchResponse struct {
	Files []File           `json:"files"`
	Peers map[string]*Peer `json:"peers"`
}

// Index is the central repository of peer and file information
type Index struct {
	Peers       map[string]*Peer    // Map of peer ID to peer info
	FilesByName map[string][]string // Map of filename to peer IDs
	FilesByHash map[string][]string // Map of file hash to peer IDs
	mutex       sync.RWMutex        // For thread safety
}

// NewIndex creates a new empty index
func NewIndex() *Index {
	return &Index{
		Peers:       make(map[string]*Peer),
		FilesByName: make(map[string][]string),
		FilesByHash: make(map[string][]string),
	}
}

// RegisterPeer adds or updates a peer in the index
func (idx *Index) RegisterPeer(peer *Peer) {
	idx.mutex.Lock()
	defer idx.mutex.Unlock()

	// Update or add the peer
	peer.LastSeen = time.Now()
	idx.Peers[peer.ID] = peer

	// Update file indices
	for _, file := range peer.Files {
		// Update FilesByName
		if _, exists := idx.FilesByName[file.Name]; !exists {
			idx.FilesByName[file.Name] = []string{}
		}
		// Check if peer ID is already in the list
		found := false
		for _, id := range idx.FilesByName[file.Name] {
			if id == peer.ID {
				found = true
				break
			}
		}
		if !found {
			idx.FilesByName[file.Name] = append(idx.FilesByName[file.Name], peer.ID)
		}

		// Update FilesByHash
		if _, exists := idx.FilesByHash[file.Hash]; !exists {
			idx.FilesByHash[file.Hash] = []string{}
		}
		// Check if peer ID is already in the list
		found = false
		for _, id := range idx.FilesByHash[file.Hash] {
			if id == peer.ID {
				found = true
				break
			}
		}
		if !found {
			idx.FilesByHash[file.Hash] = append(idx.FilesByHash[file.Hash], peer.ID)
		}
	}
}

// UnregisterPeer removes a peer from the index
func (idx *Index) UnregisterPeer(peerID string) {
	idx.mutex.Lock()
	defer idx.mutex.Unlock()

	// Get the peer
	peer, exists := idx.Peers[peerID]
	if !exists {
		return
	}

	// Remove peer from file indices
	for _, file := range peer.Files {
		// Remove from FilesByName
		if peerIDs, exists := idx.FilesByName[file.Name]; exists {
			newPeerIDs := []string{}
			for _, id := range peerIDs {
				if id != peerID {
					newPeerIDs = append(newPeerIDs, id)
				}
			}
			if len(newPeerIDs) > 0 {
				idx.FilesByName[file.Name] = newPeerIDs
			} else {
				delete(idx.FilesByName, file.Name)
			}
		}

		// Remove from FilesByHash
		if peerIDs, exists := idx.FilesByHash[file.Hash]; exists {
			newPeerIDs := []string{}
			for _, id := range peerIDs {
				if id != peerID {
					newPeerIDs = append(newPeerIDs, id)
				}
			}
			if len(newPeerIDs) > 0 {
				idx.FilesByHash[file.Hash] = newPeerIDs
			} else {
				delete(idx.FilesByHash, file.Hash)
			}
		}
	}

	// Remove the peer
	delete(idx.Peers, peerID)
}

// SearchByName searches for files by name
func (idx *Index) SearchByName(query string, limit int) ([]File, map[string]*Peer) {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()

	files := []File{}
	peers := make(map[string]*Peer)

	// Simple substring search for now
	for name, peerIDs := range idx.FilesByName {
		if len(files) >= limit && limit > 0 {
			break
		}

		if containsSubstring(name, query) {
			file := File{
				Name:    name,
				PeerIDs: peerIDs,
			}

			// Get hash and size from first peer that has this file
			if len(peerIDs) > 0 {
				firstPeerID := peerIDs[0]
				if peer, exists := idx.Peers[firstPeerID]; exists {
					for _, peerFile := range peer.Files {
						if peerFile.Name == name {
							file.Hash = peerFile.Hash
							file.Size = peerFile.Size
							break
						}
					}
				}
			}

			files = append(files, file)

			// Add peers to the result
			for _, peerID := range peerIDs {
				if peer, exists := idx.Peers[peerID]; exists {
					peers[peerID] = peer
				}
			}
		}
	}

	return files, peers
}

// Helper function to check if a string contains a substring (case insensitive)
func containsSubstring(s, substr string) bool {
	s, substr = toLowerCase(s), toLowerCase(substr)
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

// Helper function to convert a string to lowercase
func toLowerCase(s string) string {
	result := ""
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			result += string(c + 32)
		} else {
			result += string(c)
		}
	}
	return result
}

// CleanupDeadPeers removes peers that haven't been seen for a while
func (idx *Index) CleanupDeadPeers(timeout time.Duration) {
	idx.mutex.Lock()
	defer idx.mutex.Unlock()

	now := time.Now()
	for id, peer := range idx.Peers {
		if now.Sub(peer.LastSeen) > timeout {
			// Remove peer from file indices
			for _, file := range peer.Files {
				// Remove from FilesByName
				if peerIDs, exists := idx.FilesByName[file.Name]; exists {
					newPeerIDs := []string{}
					for _, pid := range peerIDs {
						if pid != id {
							newPeerIDs = append(newPeerIDs, pid)
						}
					}
					if len(newPeerIDs) > 0 {
						idx.FilesByName[file.Name] = newPeerIDs
					} else {
						delete(idx.FilesByName, file.Name)
					}
				}

				// Remove from FilesByHash
				if peerIDs, exists := idx.FilesByHash[file.Hash]; exists {
					newPeerIDs := []string{}
					for _, pid := range peerIDs {
						if pid != id {
							newPeerIDs = append(newPeerIDs, pid)
						}
					}
					if len(newPeerIDs) > 0 {
						idx.FilesByHash[file.Hash] = newPeerIDs
					} else {
						delete(idx.FilesByHash, file.Hash)
					}
				}
			}

			// Remove the peer
			delete(idx.Peers, id)
		}
	}
}

// GetStats returns statistics about the index
func (idx *Index) GetStats() map[string]interface{} {
	idx.mutex.RLock()
	defer idx.mutex.RUnlock()

	// Count unique files
	uniqueFiles := make(map[string]bool)
	for hash := range idx.FilesByHash {
		uniqueFiles[hash] = true
	}

	return map[string]interface{}{
		"peerCount":     len(idx.Peers),
		"uniqueFiles":   len(uniqueFiles),
		"totalFileRefs": len(idx.FilesByName),
	}
}

// SuperPeer is the main server that coordinates the P2P network
type SuperPeer struct {
	index            *Index
	registrationChan chan *Peer
	searchChan       chan SearchRequest
	unregisterChan   chan string
	statsChan        chan chan map[string]interface{}
	webPort          int
}

// NewSuperPeer creates a new super peer
func NewSuperPeer(webPort int) *SuperPeer {
	return &SuperPeer{
		index:            NewIndex(),
		registrationChan: make(chan *Peer, 100),
		searchChan:       make(chan SearchRequest, 100),
		unregisterChan:   make(chan string, 100),
		statsChan:        make(chan chan map[string]interface{}, 10),
		webPort:          webPort,
	}
}

// Start starts the super peer services
func (sp *SuperPeer) Start() {
	// Start the registration service
	go sp.registrationService()

	// Start the heartbeat service
	go sp.heartbeatService()

	// Start the HTTP server for peer communication
	go sp.startHTTPServer()

	// Start the web UI
	go sp.startWebUI()

	// Log that we're starting
	log.Println("Super peer started")

	// Block forever
	select {}
}

// registrationService handles peer registrations
func (sp *SuperPeer) registrationService() {
	for {
		select {
		case peer := <-sp.registrationChan:
			sp.index.RegisterPeer(peer)
			log.Printf("Registered peer %s with %d files\n", peer.ID, len(peer.Files))
		case peerID := <-sp.unregisterChan:
			sp.index.UnregisterPeer(peerID)
			log.Printf("Unregistered peer %s\n", peerID)
		}
	}
}

// heartbeatService periodically cleans up dead peers
func (sp *SuperPeer) heartbeatService() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		<-ticker.C
		sp.index.CleanupDeadPeers(5 * time.Minute)
		log.Println("Cleaned up dead peers")
	}
}

// startHTTPServer starts the HTTP server for peer communication
func (sp *SuperPeer) startHTTPServer() {
	// Register handler
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var peer Peer
		err := json.NewDecoder(r.Body).Decode(&peer)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Get the peer's IP address
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil {
			peer.Address = host
		}

		sp.registrationChan <- &peer
		w.WriteHeader(http.StatusOK)
	})

	// Unregister handler
	http.HandleFunc("/unregister", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var data struct {
			PeerID string `json:"peerId"`
		}
		err := json.NewDecoder(r.Body).Decode(&data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		sp.unregisterChan <- data.PeerID
		w.WriteHeader(http.StatusOK)
	})

	// Search handler
	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req SearchRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		files, peers := sp.index.SearchByName(req.Query, req.Limit)
		resp := SearchResponse{
			Files: files,
			Peers: peers,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// Heartbeat handler
	http.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var data struct {
			PeerID string `json:"peerId"`
		}
		err := json.NewDecoder(r.Body).Decode(&data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Update the peer's last seen time
		sp.index.mutex.Lock()
		if peer, exists := sp.index.Peers[data.PeerID]; exists {
			peer.LastSeen = time.Now()
		}
		sp.index.mutex.Unlock()

		w.WriteHeader(http.StatusOK)
	})

	// Stats handler
	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		stats := sp.index.GetStats()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})

	// Start the server
	log.Println("Starting HTTP server on :8080")
	go func() {
		err := http.ListenAndServe(":8080", nil)
		if err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()
}

// Serve static files for the admin UI
func (sp *SuperPeer) serveStaticFiles() {
	http.HandleFunc("/admin/static/", func(w http.ResponseWriter, r *http.Request) {
		// Extract the file path from the URL
		filePath := r.URL.Path[len("/admin/static/"):]

		// Set appropriate content type based on file extension
		switch {
		case strings.HasSuffix(filePath, ".css"):
			w.Header().Set("Content-Type", "text/css")
		case strings.HasSuffix(filePath, ".js"):
			w.Header().Set("Content-Type", "application/javascript")
		}

		// Serve the static content
		switch filePath {
		case "styles.css":
			w.Write([]byte(`
				:root {
					--primary-color: #4361ee;
					--secondary-color: #3f37c9;
					--accent-color: #4895ef;
					--success-color: #4cc9f0;
					--warning-color: #f72585;
					--text-color: #333;
					--bg-color: #f8f9fa;
					--card-bg: #ffffff;
					--border-color: #e9ecef;
				}
				
				* {
					box-sizing: border-box;
					margin: 0;
					padding: 0;
				}
				
				body {
					font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
					background-color: var(--bg-color);
					color: var(--text-color);
					line-height: 1.6;
					transition: background-color 0.3s ease;
				}
				
				.container {
					max-width: 1200px;
					margin: 0 auto;
					padding: 20px;
				}
				
				.card {
					background-color: var(--card-bg);
					border-radius: 10px;
					box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
					padding: 20px;
					margin-bottom: 20px;
					transition: transform 0.3s ease, box-shadow 0.3s ease;
				}
				
				.card:hover {
					transform: translateY(-5px);
					box-shadow: 0 8px 15px rgba(0, 0, 0, 0.1);
				}
				
				.header {
					display: flex;
					justify-content: space-between;
					align-items: center;
					margin-bottom: 20px;
					padding-bottom: 10px;
					border-bottom: 1px solid var(--border-color);
				}
				
				.header h1 {
					color: var(--primary-color);
					font-size: 2rem;
				}
				
				.stats {
					display: flex;
					justify-content: space-between;
					margin-bottom: 20px;
				}
				
				.stat-card {
					background-color: var(--card-bg);
					border-radius: 10px;
					box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
					padding: 20px;
					flex: 1;
					margin-right: 15px;
					text-align: center;
					transition: transform 0.3s ease;
				}
				
				.stat-card:hover {
					transform: translateY(-5px);
				}
				
				.stat-card:last-child {
					margin-right: 0;
				}
				
				.stat-card h3 {
					color: var(--secondary-color);
					margin-bottom: 10px;
				}
				
				.stat-card .value {
					font-size: 2.5rem;
					font-weight: bold;
					color: var(--primary-color);
				}
				
				.section {
					margin-bottom: 30px;
				}
				
				.section-header {
					display: flex;
					justify-content: space-between;
					align-items: center;
					margin-bottom: 15px;
				}
				
				.section-header h2 {
					color: var(--secondary-color);
					font-size: 1.5rem;
				}
				
				table {
					width: 100%;
					border-collapse: collapse;
					margin-bottom: 20px;
				}
				
				th, td {
					text-align: left;
					padding: 12px;
					border-bottom: 1px solid var(--border-color);
				}
				
				th {
					background-color: rgba(67, 97, 238, 0.1);
					color: var(--primary-color);
					font-weight: 600;
				}
				
				tr:hover {
					background-color: rgba(67, 97, 238, 0.05);
				}
				
				.badge {
					display: inline-block;
					padding: 3px 8px;
					border-radius: 20px;
					font-size: 0.8rem;
					font-weight: 600;
				}
				
				.badge.online {
					background-color: var(--success-color);
					color: white;
				}
				
				.badge.offline {
					background-color: var(--warning-color);
					color: white;
				}
				
				.search-form {
					display: flex;
					margin-bottom: 20px;
				}
				
				.search-form input {
					flex-grow: 1;
					padding: 12px;
					border: 1px solid var(--border-color);
					border-radius: 8px 0 0 8px;
					font-size: 1rem;
				}
				
				.search-form button {
					padding: 12px 20px;
					background-color: var(--primary-color);
					color: white;
					border: none;
					border-radius: 0 8px 8px 0;
					cursor: pointer;
					font-size: 1rem;
				}
				
				.theme-toggle {
					background: none;
					border: none;
					cursor: pointer;
					font-size: 1.2rem;
					color: var(--text-color);
					margin-left: 10px;
				}
				
				.dark-theme {
					--bg-color: #121212;
					--card-bg: #1e1e1e;
					--text-color: #e0e0e0;
					--border-color: #333;
				}
				
				@keyframes fadeIn {
					from { opacity: 0; transform: translateY(-10px); }
					to { opacity: 1; transform: translateY(0); }
				}
				
				.animate-fade-in {
					animation: fadeIn 0.5s ease;
				}
				
				@keyframes pulse {
					0% { transform: scale(1); }
					50% { transform: scale(1.05); }
					100% { transform: scale(1); }
				}
				
				.animate-pulse {
					animation: pulse 2s infinite;
				}
				
				@media (max-width: 768px) {
					.stats {
						flex-direction: column;
					}
					
					.stat-card {
						margin-right: 0;
						margin-bottom: 15px;
					}
					
					.search-form {
						flex-direction: column;
					}
					
					.search-form input, .search-form button {
						width: 100%;
						border-radius: 8px;
					}
					
					.search-form button {
						margin-top: 10px;
					}
				}
			`))
		case "script.js":
			w.Write([]byte(`
				document.addEventListener('DOMContentLoaded', function() {
					// Theme toggle functionality
					const themeToggle = document.getElementById('theme-toggle');
					const body = document.body;
					
					themeToggle.addEventListener('click', function() {
						body.classList.toggle('dark-theme');
						const isDark = body.classList.contains('dark-theme');
						themeToggle.innerHTML = isDark ? '☀️' : '🌙';
						localStorage.setItem('dark-theme', isDark);
					});
					
					// Check for saved theme preference
					if (localStorage.getItem('dark-theme') === 'true') {
						body.classList.add('dark-theme');
						themeToggle.innerHTML = '☀️';
					}
					
					// Animate elements when they come into view
					const animateOnScroll = function() {
						const cards = document.querySelectorAll('.card');
						cards.forEach(card => {
							const cardPosition = card.getBoundingClientRect();
							// Check if card is in viewport
							if (cardPosition.top < window.innerHeight && cardPosition.bottom >= 0) {
								card.style.opacity = '1';
								card.style.transform = 'translateY(0)';
							}
						});
					};
					
					// Set initial state for cards
					const cards = document.querySelectorAll('.card');
					cards.forEach(card => {
						card.style.opacity = '0';
						card.style.transform = 'translateY(20px)';
						card.style.transition = 'opacity 0.5s ease, transform 0.5s ease';
					});
					
					// Run animation on load and scroll
					animateOnScroll();
					window.addEventListener('scroll', animateOnScroll);
					
					// Real-time updates
					function updateStats() {
						fetch('/admin/api/stats')
							.then(response => response.json())
							.then(data => {
								document.getElementById('peer-count').textContent = data.peerCount;
								document.getElementById('file-count').textContent = data.uniqueFiles;
								document.getElementById('ref-count').textContent = data.totalFileRefs;
							})
							.catch(error => console.error('Error fetching stats:', error));
					}
					
					// Update stats every 5 seconds
					setInterval(updateStats, 5000);
					
					// Network visualization
					const networkCanvas = document.getElementById('network-canvas');
					if (networkCanvas) {
						const ctx = networkCanvas.getContext('2d');
						const peers = [];
						
						// Resize canvas to fit container
						function resizeCanvas() {
							const container = networkCanvas.parentElement;
							networkCanvas.width = container.clientWidth;
							networkCanvas.height = 300;
						}
						
						// Initialize peers
						function initPeers() {
							fetch('/admin/api/peers')
								.then(response => response.json())
								.then(data => {
									peers.length = 0;
									data.forEach((peer, index) => {
										peers.push({
											id: peer.id,
											x: Math.random() * networkCanvas.width,
											y: Math.random() * networkCanvas.height,
											radius: 10,
											color: peer.isOnline ? '#4cc9f0' : '#f72585',
											connections: peer.connections || []
										});
									});
									drawNetwork();
								})
								.catch(error => console.error('Error fetching peers:', error));
						}
						
						// Draw the network
						function drawNetwork() {
							ctx.clearRect(0, 0, networkCanvas.width, networkCanvas.height);
							
							// Draw connections
							ctx.strokeStyle = '#e9ecef';
							ctx.lineWidth = 1;
							peers.forEach(peer => {
								peer.connections.forEach(connId => {
									const connPeer = peers.find(p => p.id === connId);
									if (connPeer) {
										ctx.beginPath();
										ctx.moveTo(peer.x, peer.y);
										ctx.lineTo(connPeer.x, connPeer.y);
										ctx.stroke();
									}
								});
							});
							
							// Draw peers
							peers.forEach(peer => {
								ctx.beginPath();
								ctx.arc(peer.x, peer.y, peer.radius, 0, Math.PI * 2);
								ctx.fillStyle = peer.color;
								ctx.fill();
								
								// Draw peer ID
								ctx.fillStyle = '#333';
								ctx.font = '10px Arial';
								ctx.textAlign = 'center';
								ctx.fillText(peer.id.substring(0, 8), peer.x, peer.y + 20);
							});
						}
						
						// Initialize and set up auto-refresh
						resizeCanvas();
						initPeers();
						window.addEventListener('resize', resizeCanvas);
						setInterval(initPeers, 5000);
					}
				});
			`))
		default:
			http.NotFound(w, r)
		}
	})
}

// startWebUI starts the web-based user interface for the super peer
func (sp *SuperPeer) startWebUI() {
	// Serve static files
	sp.serveStaticFiles()

	// API endpoint for stats
	http.HandleFunc("/admin/api/stats", func(w http.ResponseWriter, r *http.Request) {
		stats := sp.index.GetStats()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})

	// API endpoint for peers
	http.HandleFunc("/admin/api/peers", func(w http.ResponseWriter, r *http.Request) {
		sp.index.mutex.RLock()
		peers := make([]*PeerWithStatus, 0, len(sp.index.Peers))
		for _, peer := range sp.index.Peers {
			// Create connections based on shared files
			connections := make([]string, 0)
			for _, file := range peer.Files {
				for _, peerID := range sp.index.FilesByHash[file.Hash] {
					if peerID != peer.ID && !contains(connections, peerID) {
						connections = append(connections, peerID)
					}
				}
			}

			peers = append(peers, &PeerWithStatus{
				Peer:        *peer,
				IsOnline:    time.Since(peer.LastSeen) < 5*time.Minute,
				Connections: connections,
			})
		}
		sp.index.mutex.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(peers)
	})

	// HTML template for the web UI
	const htmlTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>P2P Super Peer Dashboard</title>
    <link rel="stylesheet" href="/admin/static/styles.css">
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0-beta3/css/all.min.css">
</head>
<body>
    <div class="container">
        <div class="card">
            <div class="header">
                <h1><i class="fas fa-server"></i> P2P Super Peer Dashboard</h1>
                <button id="theme-toggle" class="theme-toggle">🌙</button>
            </div>
            
            <div class="stats">
                <div class="stat-card animate-fade-in">
                    <h3><i class="fas fa-users"></i> Connected Peers</h3>
                    <div class="value" id="peer-count">{{.PeerCount}}</div>
                </div>
                <div class="stat-card animate-fade-in" style="animation-delay: 0.1s;">
                    <h3><i class="fas fa-file"></i> Unique Files</h3>
                    <div class="value" id="file-count">{{.UniqueFiles}}</div>
                </div>
                <div class="stat-card animate-fade-in" style="animation-delay: 0.2s;">
                    <h3><i class="fas fa-share-alt"></i> File References</h3>
                    <div class="value" id="ref-count">{{.TotalFileRefs}}</div>
                </div>
            </div>
            
            <div class="card">
                <div class="section-header">
                    <h2><i class="fas fa-project-diagram"></i> Network Visualization</h2>
                </div>
                <canvas id="network-canvas" style="width: 100%; height: 300px;"></canvas>
            </div>
            
            <div class="section">
                <div class="section-header">
                    <h2><i class="fas fa-users"></i> Connected Peers</h2>
                </div>
                <table>
                    <thead>
                        <tr>
                            <th>ID</th>
                            <th>Address</th>
                            <th>Port</th>
                            <th>Files</th>
                            <th>Last Seen</th>
                            <th>Status</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .Peers}}
                        <tr class="animate-fade-in">
                            <td>{{.ID}}</td>
                            <td>{{.Address}}</td>
                            <td>{{.Port}}</td>
                            <td><span class="badge">{{len .Files}}</span></td>
                            <td>{{formatTime .LastSeen}}</td>
                            <td>
                                {{if .IsOnline}}
                                <span class="badge online"><i class="fas fa-circle"></i> Online</span>
                                {{else}}
                                <span class="badge offline"><i class="fas fa-circle"></i> Offline</span>
                                {{end}}
                            </td>
                        </tr>
                        {{else}}
                        <tr>
                            <td colspan="6" class="empty-state">No peers connected</td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
            
            <div class="section">
                <div class="section-header">
                    <h2><i class="fas fa-file"></i> Indexed Files</h2>
                </div>
                <form class="search-form" action="/admin/search" method="get">
                    <input type="text" name="query" placeholder="Search files..." value="{{.SearchQuery}}">
                    <button type="submit"><i class="fas fa-search"></i> Search</button>
                </form>
                
                <table>
                    <thead>
                        <tr>
                            <th>Name</th>
                            <th>Size</th>
                            <th>Hash</th>
                            <th>Available From</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .Files}}
                        <tr class="animate-fade-in">
                            <td><i class="fas fa-file file-icon"></i> {{.Name}}</td>
                            <td>{{formatSize .Size}}</td>
                            <td>{{truncateHash .Hash}}</td>
                            <td>
                                <span class="badge">{{len .PeerIDs}} peers</span>
                            </td>
                        </tr>
                        {{else}}
                        <tr>
                            <td colspan="4" class="empty-state">
                                {{if .SearchQuery}}
                                No files found matching "{{.SearchQuery}}"
                                {{else}}
                                No files indexed
                                {{end}}
                            </td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
        </div>
    </div>
    
    <script src="/admin/static/script.js"></script>
</body>
</html>
`

	// Create template functions
	funcMap := template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
		"formatSize": func(size int64) string {
			if size < 1024 {
				return fmt.Sprintf("%d bytes", size)
			} else if size < 1024*1024 {
				return fmt.Sprintf("%.1f KB", float64(size)/1024)
			} else if size < 1024*1024*1024 {
				return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
			}
			return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
		},
		"truncateHash": func(hash string) string {
			if len(hash) > 12 {
				return hash[:6] + "..." + hash[len(hash)-6:]
			}
			return hash
		},
	}

	// Parse the HTML template
	tmpl, err := template.New("index").Funcs(funcMap).Parse(htmlTemplate)
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}

	// Handler for the main page
	http.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin" {
			http.NotFound(w, r)
			return
		}

		// Get stats
		stats := sp.index.GetStats()

		// Get all peers
		sp.index.mutex.RLock()
		peers := make([]*PeerWithStatus, 0, len(sp.index.Peers))
		for _, peer := range sp.index.Peers {
			peers = append(peers, &PeerWithStatus{
				Peer:     *peer,
				IsOnline: time.Since(peer.LastSeen) < 5*time.Minute,
			})
		}
		sp.index.mutex.RUnlock()

		// Get search query
		searchQuery := r.URL.Query().Get("query")

		// Get files
		var files []File
		if searchQuery != "" {
			fileList, _ := sp.index.SearchByName(searchQuery, 100)
			files = fileList
		} else {
			// Get all files
			sp.index.mutex.RLock()
			uniqueFiles := make(map[string]File)
			for name, peerIDs := range sp.index.FilesByName {
				file := File{
					Name:    name,
					PeerIDs: peerIDs,
				}

				// Get hash and size from first peer that has this file
				if len(peerIDs) > 0 {
					firstPeerID := peerIDs[0]
					if peer, exists := sp.index.Peers[firstPeerID]; exists {
						for _, peerFile := range peer.Files {
							if peerFile.Name == name {
								file.Hash = peerFile.Hash
								file.Size = peerFile.Size
								break
							}
						}
					}
				}

				uniqueFiles[name] = file
			}
			sp.index.mutex.RUnlock()

			files = make([]File, 0, len(uniqueFiles))
			for _, file := range uniqueFiles {
				files = append(files, file)
			}
		}

		// Prepare template data
		data := struct {
			PeerCount     int
			UniqueFiles   int
			TotalFileRefs int
			Peers         []*PeerWithStatus
			Files         []File
			SearchQuery   string
		}{
			PeerCount:     stats["peerCount"].(int),
			UniqueFiles:   stats["uniqueFiles"].(int),
			TotalFileRefs: stats["totalFileRefs"].(int),
			Peers:         peers,
			Files:         files,
			SearchQuery:   searchQuery,
		}

		// Execute the template
		w.Header().Set("Content-Type", "text/html")
		err = tmpl.Execute(w, data)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
		}
	})

	// Handler for searching files
	http.HandleFunc("/admin/search", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		http.Redirect(w, r, fmt.Sprintf("/admin?query=%s", query), http.StatusSeeOther)
	})

	// Start the web server
	addr := fmt.Sprintf(":%d", sp.webPort)
	log.Printf("Starting admin web UI on http://localhost:%d/admin", sp.webPort)
	go func() {
		err := http.ListenAndServe(addr, nil)
		if err != nil {
			log.Fatalf("Failed to start web UI: %v", err)
		}
	}()
}

// PeerWithStatus adds online status to peer information
type PeerWithStatus struct {
	Peer
	IsOnline    bool
	Connections []string
}

// Helper function to check if a slice contains a string
func contains(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

func main() {
	fmt.Println("Starting P2P Super Peer...")
	sp := NewSuperPeer(8085)
	sp.Start()
}
