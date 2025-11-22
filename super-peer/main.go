package main

// import (
// 	"encoding/json"
// 	"fmt"
// 	"log"
// 	"net"
// 	"net/http"
// 	"sync"
// 	"time"
// )

// // Peer represents a node in the P2P network
// type Peer struct {
// 	ID       string    `json:"id"`
// 	Address  string    `json:"address"`
// 	Port     int       `json:"port"`
// 	LastSeen time.Time `json:"lastSeen"`
// 	Files    []File    `json:"files"`
// }

// // File represents a file in the P2P network
// type File struct {
// 	Name    string   `json:"name"`
// 	Hash    string   `json:"hash"`
// 	Size    int64    `json:"size"`
// 	PeerIDs []string `json:"peerIds"`
// }

// // SearchRequest represents a search query from a peer
// type SearchRequest struct {
// 	Query    string `json:"query"`
// 	Limit    int    `json:"limit"`
// 	FromPeer string `json:"fromPeer"`
// }

// // SearchResponse represents the response to a search query
// type SearchResponse struct {
// 	Files []File           `json:"files"`
// 	Peers map[string]*Peer `json:"peers"`
// }

// // Index is the central repository of peer and file information
// type Index struct {
// 	Peers       map[string]*Peer  // Map of peer ID to peer info
// 	FilesByName map[string][]string // Map of filename to peer IDs
// 	FilesByHash map[string][]string // Map of file hash to peer IDs
// 	mutex       sync.RWMutex      // For thread safety
// }

// // NewIndex creates a new empty index
// func NewIndex() *Index {
// 	return &Index{
// 		Peers:       make(map[string]*Peer),
// 		FilesByName: make(map[string][]string),
// 		FilesByHash: make(map[string][]string),
// 	}
// }

// // RegisterPeer adds or updates a peer in the index
// func (idx *Index) RegisterPeer(peer *Peer) {
// 	idx.mutex.Lock()
// 	defer idx.mutex.Unlock()

// 	// Update or add the peer
// 	peer.LastSeen = time.Now()
// 	idx.Peers[peer.ID] = peer

// 	// Update file indices
// 	for _, file := range peer.Files {
// 		// Update FilesByName
// 		if _, exists := idx.FilesByName[file.Name]; !exists {
// 			idx.FilesByName[file.Name] = []string{}
// 		}
// 		// Check if peer ID is already in the list
// 		found := false
// 		for _, id := range idx.FilesByName[file.Name] {
// 			if id == peer.ID {
// 				found = true
// 				break
// 			}
// 		}
// 		if !found {
// 			idx.FilesByName[file.Name] = append(idx.FilesByName[file.Name], peer.ID)
// 		}

// 		// Update FilesByHash
// 		if _, exists := idx.FilesByHash[file.Hash]; !exists {
// 			idx.FilesByHash[file.Hash] = []string{}
// 		}
// 		// Check if peer ID is already in the list
// 		found = false
// 		for _, id := range idx.FilesByHash[file.Hash] {
// 			if id == peer.ID {
// 				found = true
// 				break
// 			}
// 		}
// 		if !found {
// 			idx.FilesByHash[file.Hash] = append(idx.FilesByHash[file.Hash], peer.ID)
// 		}
// 	}
// }

// // UnregisterPeer removes a peer from the index
// func (idx *Index) UnregisterPeer(peerID string) {
// 	idx.mutex.Lock()
// 	defer idx.mutex.Unlock()

// 	// Get the peer
// 	peer, exists := idx.Peers[peerID]
// 	if !exists {
// 		return
// 	}

// 	// Remove peer from file indices
// 	for _, file := range peer.Files {
// 		// Remove from FilesByName
// 		if peerIDs, exists := idx.FilesByName[file.Name]; exists {
// 			newPeerIDs := []string{}
// 			for _, id := range peerIDs {
// 				if id != peerID {
// 					newPeerIDs = append(newPeerIDs, id)
// 				}
// 			}
// 			if len(newPeerIDs) > 0 {
// 				idx.FilesByName[file.Name] = newPeerIDs
// 			} else {
// 				delete(idx.FilesByName, file.Name)
// 			}
// 		}

// 		// Remove from FilesByHash
// 		if peerIDs, exists := idx.FilesByHash[file.Hash]; exists {
// 			newPeerIDs := []string{}
// 			for _, id := range peerIDs {
// 				if id != peerID {
// 					newPeerIDs = append(newPeerIDs, id)
// 				}
// 			}
// 			if len(newPeerIDs) > 0 {
// 				idx.FilesByHash[file.Hash] = newPeerIDs
// 			} else {
// 				delete(idx.FilesByHash, file.Hash)
// 			}
// 		}
// 	}

// 	// Remove the peer
// 	delete(idx.Peers, peerID)
// }

// // SearchByName searches for files by name
// func (idx *Index) SearchByName(query string, limit int) ([]File, map[string]*Peer) {
// 	idx.mutex.RLock()
// 	defer idx.mutex.RUnlock()

// 	files := []File{}
// 	peers := make(map[string]*Peer)

// 	// Simple substring search for now
// 	for name, peerIDs := range idx.FilesByName {
// 		if len(files) >= limit && limit > 0 {
// 			break
// 		}

// 		if containsSubstring(name, query) {
// 			file := File{
// 				Name:    name,
// 				PeerIDs: peerIDs,
// 			}

// 			// Get hash and size from first peer that has this file
// 			if len(peerIDs) > 0 {
// 				firstPeerID := peerIDs[0]
// 				if peer, exists := idx.Peers[firstPeerID]; exists {
// 					for _, peerFile := range peer.Files {
// 						if peerFile.Name == name {
// 							file.Hash = peerFile.Hash
// 							file.Size = peerFile.Size
// 							break
// 						}
// 					}
// 				}
// 			}

// 			files = append(files, file)

// 			// Add peers to the result
// 			for _, peerID := range peerIDs {
// 				if peer, exists := idx.Peers[peerID]; exists {
// 					peers[peerID] = peer
// 				}
// 			}
// 		}
// 	}

// 	return files, peers
// }

// // Helper function to check if a string contains a substring (case insensitive)
// func containsSubstring(s, substr string) bool {
// 	s, substr = toLowerCase(s), toLowerCase(substr)
// 	return len(s) >= len(substr) && s[:len(substr)] == substr
// }

// // Helper function to convert a string to lowercase
// func toLowerCase(s string) string {
// 	result := ""
// 	for _, c := range s {
// 		if c >= 'A' && c <= 'Z' {
// 			result += string(c + 32)
// 		} else {
// 			result += string(c)
// 		}
// 	}
// 	return result
// }

// // CleanupDeadPeers removes peers that haven't been seen for a while
// func (idx *Index) CleanupDeadPeers(timeout time.Duration) {
// 	idx.mutex.Lock()
// 	defer idx.mutex.Unlock()

// 	now := time.Now()
// 	for id, peer := range idx.Peers {
// 		if now.Sub(peer.LastSeen) > timeout {
// 			// Remove peer from file indices
// 			for _, file := range peer.Files {
// 				// Remove from FilesByName
// 				if peerIDs, exists := idx.FilesByName[file.Name]; exists {
// 					newPeerIDs := []string{}
// 					for _, pid := range peerIDs {
// 						if pid != id {
// 							newPeerIDs = append(newPeerIDs, pid)
// 						}
// 					}
// 					if len(newPeerIDs) > 0 {
// 						idx.FilesByName[file.Name] = newPeerIDs
// 					} else {
// 						delete(idx.FilesByName, file.Name)
// 					}
// 				}

// 				// Remove from FilesByHash
// 				if peerIDs, exists := idx.FilesByHash[file.Hash]; exists {
// 					newPeerIDs := []string{}
// 					for _, pid := range peerIDs {
// 						if pid != id {
// 							newPeerIDs = append(newPeerIDs, pid)
// 						}
// 					}
// 					if len(newPeerIDs) > 0 {
// 						idx.FilesByHash[file.Hash] = newPeerIDs
// 					} else {
// 						delete(idx.FilesByHash, file.Hash)
// 					}
// 				}
// 			}

// 			// Remove the peer
// 			delete(idx.Peers, id)
// 		}
// 	}
// }

// // GetStats returns statistics about the index
// func (idx *Index) GetStats() map[string]interface{} {
// 	idx.mutex.RLock()
// 	defer idx.mutex.RUnlock()

// 	// Count unique files
// 	uniqueFiles := make(map[string]bool)
// 	for hash := range idx.FilesByHash {
// 		uniqueFiles[hash] = true
// 	}

// 	return map[string]interface{}{
// 		"peerCount":     len(idx.Peers),
// 		"uniqueFiles":   len(uniqueFiles),
// 		"totalFileRefs": len(idx.FilesByName),
// 	}
// }

// // SuperPeer is the main server that coordinates the P2P network
// type SuperPeer struct {
// 	index            *Index
// 	registrationChan chan *Peer
// 	searchChan       chan SearchRequest
// 	unregisterChan   chan string
// 	statsChan        chan chan map[string]interface{}
// }

// // NewSuperPeer creates a new super peer
// func NewSuperPeer() *SuperPeer {
// 	return &SuperPeer{
// 		index:            NewIndex(),
// 		registrationChan: make(chan *Peer, 100),
// 		searchChan:       make(chan SearchRequest, 100),
// 		unregisterChan:   make(chan string, 100),
// 		statsChan:        make(chan chan map[string]interface{}, 10),
// 	}
// }

// // Start starts the super peer services
// func (sp *SuperPeer) Start() {
// 	// Start the registration service
// 	go sp.registrationService()

// 	// Start the heartbeat service
// 	go sp.heartbeatService()

// 	// Start the HTTP server
// 	go sp.startHTTPServer()

// 	// Log that we're starting
// 	log.Println("Super peer started")

// 	// Block forever
// 	select {}
// }

// // registrationService handles peer registrations
// func (sp *SuperPeer) registrationService() {
// 	for {
// 		select {
// 		case peer := <-sp.registrationChan:
// 			sp.index.RegisterPeer(peer)
// 			log.Printf("Registered peer %s with %d files\n", peer.ID, len(peer.Files))
// 		case peerID := <-sp.unregisterChan:
// 			sp.index.UnregisterPeer(peerID)
// 			log.Printf("Unregistered peer %s\n", peerID)
// 		}
// 	}
// }

// // heartbeatService periodically cleans up dead peers
// func (sp *SuperPeer) heartbeatService() {
// 	ticker := time.NewTicker(1 * time.Minute)
// 	defer ticker.Stop()

// 	for {
// 		<-ticker.C
// 		sp.index.CleanupDeadPeers(5 * time.Minute)
// 		log.Println("Cleaned up dead peers")
// 	}
// }

// // startHTTPServer starts the HTTP server for peer communication
// func (sp *SuperPeer) startHTTPServer() {
// 	// Register handler
// 	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
// 		if r.Method != http.MethodPost {
// 			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 			return
// 		}

// 		var peer Peer
// 		err := json.NewDecoder(r.Body).Decode(&peer)
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusBadRequest)
// 			return
// 		}

// 		// Get the peer's IP address
// 		host, _, err := net.SplitHostPort(r.RemoteAddr)
// 		if err == nil {
// 			peer.Address = host
// 		}

// 		sp.registrationChan <- &peer
// 		w.WriteHeader(http.StatusOK)
// 	})

// 	// Unregister handler
// 	http.HandleFunc("/unregister", func(w http.ResponseWriter, r *http.Request) {
// 		if r.Method != http.MethodPost {
// 			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 			return
// 		}

// 		var data struct {
// 			PeerID string `json:"peerId"`
// 		}
// 		err := json.NewDecoder(r.Body).Decode(&data)
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusBadRequest)
// 			return
// 		}

// 		sp.unregisterChan <- data.PeerID
// 		w.WriteHeader(http.StatusOK)
// 	})

// 	// Search handler
// 	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
// 		if r.Method != http.MethodPost {
// 			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 			return
// 		}

// 		var req SearchRequest
// 		err := json.NewDecoder(r.Body).Decode(&req)
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusBadRequest)
// 			return
// 		}

// 		files, peers := sp.index.SearchByName(req.Query, req.Limit)
// 		resp := SearchResponse{
// 			Files: files,
// 			Peers: peers,
// 		}

// 		w.Header().Set("Content-Type", "application/json")
// 		json.NewEncoder(w).Encode(resp)
// 	})

// 	// Heartbeat handler
// 	http.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
// 		if r.Method != http.MethodPost {
// 			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 			return
// 		}

// 		var data struct {
// 			PeerID string `json:"peerId"`
// 		}
// 		err := json.NewDecoder(r.Body).Decode(&data)
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusBadRequest)
// 			return
// 		}

// 		// Update the peer's last seen time
// 		sp.index.mutex.Lock()
// 		if peer, exists := sp.index.Peers[data.PeerID]; exists {
// 			peer.LastSeen = time.Now()
// 		}
// 		sp.index.mutex.Unlock()

// 		w.WriteHeader(http.StatusOK)
// 	})

// 	// Stats handler
// 	http.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
// 		if r.Method != http.MethodGet {
// 			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 			return
// 		}

// 		stats := sp.index.GetStats()
// 		w.Header().Set("Content-Type", "application/json")
// 		json.NewEncoder(w).Encode(stats)
// 	})

// 	// Start the server
// 	log.Println("Starting HTTP server on :8080")
// 	err := http.ListenAndServe(":8080", nil)
// 	if err != nil {
// 		log.Fatalf("Failed to start HTTP server: %v", err)
// 	}
// }

// func main() {
// 	fmt.Println("Starting P2P Super Peer...")
// 	sp := NewSuperPeer()
// 	sp.Start()
// }
