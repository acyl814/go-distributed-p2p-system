package main

// import (
// 	"bufio"
// 	"bytes"
// 	"crypto/sha256"
// 	"encoding/hex"
// 	"encoding/json"
// 	"flag"
// 	"fmt"
// 	"io"
// 	"log"
// 	"math/rand"
// 	"net/http"
// 	"os"
// 	"path/filepath"
// 	"strconv"
// 	"strings"
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

// // SearchRequest represents a search query to the super peer
// type SearchRequest struct {
// 	Query    string `json:"query"`
// 	Limit    int    `json:"limit"`
// 	FromPeer string `json:"fromPeer"`
// }

// // SearchResponse represents the response from the super peer
// type SearchResponse struct {
// 	Files []File           `json:"files"`
// 	Peers map[string]*Peer `json:"peers"`
// }

// // FileRequest represents a request for a file from another peer
// type FileRequest struct {
// 	FileName string `json:"fileName"`
// 	FileHash string `json:"fileHash"`
// }

// // PeerClient is the client that communicates with the super peer and other peers
// type PeerClient struct {
// 	ID             string
// 	SuperPeerURL   string
// 	LocalPort      int
// 	SharedDir      string
// 	DownloadDir    string
// 	Files          []File
// 	ActiveDownloads map[string]bool
// 	mutex          sync.RWMutex
// 	httpClient     *http.Client
// }

// // NewPeerClient creates a new peer client
// func NewPeerClient(superPeerURL string, localPort int, sharedDir, downloadDir string) *PeerClient {
// 	// Generate a random ID
// 	rand.Seed(time.Now().UnixNano())
// 	id := fmt.Sprintf("peer-%d", rand.Intn(10000))

// 	return &PeerClient{
// 		ID:             id,
// 		SuperPeerURL:   superPeerURL,
// 		LocalPort:      localPort,
// 		SharedDir:      sharedDir,
// 		DownloadDir:    downloadDir,
// 		Files:          []File{},
// 		ActiveDownloads: make(map[string]bool),
// 		httpClient:     &http.Client{Timeout: 30 * time.Second},
// 	}
// }

// // Start starts the peer client
// func (pc *PeerClient) Start() {
// 	// Create directories if they don't exist
// 	os.MkdirAll(pc.SharedDir, 0755)
// 	os.MkdirAll(pc.DownloadDir, 0755)

// 	// Scan shared directory for files
// 	pc.ScanSharedDirectory()

// 	// Register with super peer
// 	err := pc.Register()
// 	if err != nil {
// 		log.Fatalf("Failed to register with super peer: %v", err)
// 	}

// 	// Start heartbeat service
// 	go pc.heartbeatService()

// 	// Start file server
// 	go pc.startFileServer()

// 	// Start command line interface
// 	pc.startCLI()
// }

// // ScanSharedDirectory scans the shared directory for files
// func (pc *PeerClient) ScanSharedDirectory() {
// 	pc.mutex.Lock()
// 	defer pc.mutex.Unlock()

// 	pc.Files = []File{}

// 	err := filepath.Walk(pc.SharedDir, func(path string, info os.FileInfo, err error) error {
// 		if err != nil {
// 			return err
// 		}

// 		if !info.IsDir() {
// 			relPath, err := filepath.Rel(pc.SharedDir, path)
// 			if err != nil {
// 				return err
// 			}

// 			// Calculate file hash
// 			hash, err := pc.calculateFileHash(path)
// 			if err != nil {
// 				log.Printf("Failed to calculate hash for %s: %v", path, err)
// 				return nil
// 			}

// 			file := File{
// 				Name:    relPath,
// 				Hash:    hash,
// 				Size:    info.Size(),
// 				PeerIDs: []string{pc.ID},
// 			}

// 			pc.Files = append(pc.Files, file)
// 		}

// 		return nil
// 	})

// 	if err != nil {
// 		log.Printf("Error scanning shared directory: %v", err)
// 	}

// 	log.Printf("Found %d files in shared directory", len(pc.Files))
// }

// // calculateFileHash calculates the SHA-256 hash of a file
// func (pc *PeerClient) calculateFileHash(filePath string) (string, error) {
// 	file, err := os.Open(filePath)
// 	if err != nil {
// 		return "", err
// 	}
// 	defer file.Close()

// 	hash := sha256.New()
// 	if _, err := io.Copy(hash, file); err != nil {
// 		return "", err
// 	}

// 	return hex.EncodeToString(hash.Sum(nil)), nil
// }

// // Register registers the peer with the super peer
// func (pc *PeerClient) Register() error {
// 	pc.mutex.RLock()
// 	defer pc.mutex.RUnlock()

// 	peer := Peer{
// 		ID:      pc.ID,
// 		Address: "localhost", // This will be overridden by the super peer
// 		Port:    pc.LocalPort,
// 		Files:   pc.Files,
// 	}

// 	jsonData, err := json.Marshal(peer)
// 	if err != nil {
// 		return err
// 	}

// 	resp, err := pc.httpClient.Post(pc.SuperPeerURL+"/register", "application/json", bytes.NewBuffer(jsonData))
// 	if err != nil {
// 		return err
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		body, _ := io.ReadAll(resp.Body)
// 		return fmt.Errorf("failed to register: %s", body)
// 	}

// 	log.Printf("Registered with super peer as %s", pc.ID)
// 	return nil
// }

// // Unregister unregisters the peer from the super peer
// func (pc *PeerClient) Unregister() error {
// 	data := map[string]string{"peerId": pc.ID}
// 	jsonData, err := json.Marshal(data)
// 	if err != nil {
// 		return err
// 	}

// 	resp, err := pc.httpClient.Post(pc.SuperPeerURL+"/unregister", "application/json", bytes.NewBuffer(jsonData))
// 	if err != nil {
// 		return err
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		body, _ := io.ReadAll(resp.Body)
// 		return fmt.Errorf("failed to unregister: %s", body)
// 	}

// 	log.Printf("Unregistered from super peer")
// 	return nil
// }

// // Search searches for files via the super peer
// func (pc *PeerClient) Search(query string, limit int) (*SearchResponse, error) {
// 	req := SearchRequest{
// 		Query:    query,
// 		Limit:    limit,
// 		FromPeer: pc.ID,
// 	}

// 	jsonData, err := json.Marshal(req)
// 	if err != nil {
// 		return nil, err
// 	}

// 	resp, err := pc.httpClient.Post(pc.SuperPeerURL+"/search", "application/json", bytes.NewBuffer(jsonData))
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		body, _ := io.ReadAll(resp.Body)
// 		return nil, fmt.Errorf("search failed: %s", body)
// 	}

// 	var searchResp SearchResponse
// 	err = json.NewDecoder(resp.Body).Decode(&searchResp)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &searchResp, nil
// }

// // SendHeartbeat sends a heartbeat to the super peer
// func (pc *PeerClient) SendHeartbeat() error {
// 	data := map[string]string{"peerId": pc.ID}
// 	jsonData, err := json.Marshal(data)
// 	if err != nil {
// 		return err
// 	}

// 	resp, err := pc.httpClient.Post(pc.SuperPeerURL+"/heartbeat", "application/json", bytes.NewBuffer(jsonData))
// 	if err != nil {
// 		return err
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		body, _ := io.ReadAll(resp.Body)
// 		return fmt.Errorf("heartbeat failed: %s", body)
// 	}

// 	return nil
// }

// // heartbeatService periodically sends heartbeats to the super peer
// func (pc *PeerClient) heartbeatService() {
// 	ticker := time.NewTicker(1 * time.Minute)
// 	defer ticker.Stop()

// 	for {
// 		<-ticker.C
// 		err := pc.SendHeartbeat()
// 		if err != nil {
// 			log.Printf("Failed to send heartbeat: %v", err)
// 		}
// 	}
// }

// // startFileServer starts the HTTP server for serving files to other peers
// func (pc *PeerClient) startFileServer() {
// 	// File request handler
// 	http.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
// 		if r.Method != http.MethodGet {
// 			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 			return
// 		}

// 		fileName := r.URL.Query().Get("name")
// 		if fileName == "" {
// 			http.Error(w, "Missing file name", http.StatusBadRequest)
// 			return
// 		}

// 		// Sanitize the file name to prevent directory traversal
// 		fileName = filepath.Clean(fileName)
// 		if strings.Contains(fileName, "..") {
// 			http.Error(w, "Invalid file name", http.StatusBadRequest)
// 			return
// 		}

// 		filePath := filepath.Join(pc.SharedDir, fileName)
// 		file, err := os.Open(filePath)
// 		if err != nil {
// 			http.Error(w, fmt.Sprintf("Failed to open file: %v", err), http.StatusNotFound)
// 			return
// 		}
// 		defer file.Close()

// 		// Get file info
// 		fileInfo, err := file.Stat()
// 		if err != nil {
// 			http.Error(w, fmt.Sprintf("Failed to get file info: %v", err), http.StatusInternalServerError)
// 			return
// 		}

// 		// Set content type and length
// 		w.Header().Set("Content-Type", "application/octet-stream")
// 		w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
// 		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(fileName)))

// 		// Copy the file to the response
// 		_, err = io.Copy(w, file)
// 		if err != nil {
// 			log.Printf("Error sending file: %v", err)
// 		}
// 	})

// 	// Start the server
// 	addr := fmt.Sprintf(":%d", pc.LocalPort)
// 	log.Printf("Starting file server on %s", addr)
// 	err := http.ListenAndServe(addr, nil)
// 	if err != nil {
// 		log.Fatalf("Failed to start file server: %v", err)
// 	}
// }

// // DownloadFile downloads a file from another peer
// func (pc *PeerClient) DownloadFile(fileName, fileHash string, peer *Peer) error {
// 	pc.mutex.Lock()
// 	if pc.ActiveDownloads[fileHash] {
// 		pc.mutex.Unlock()
// 		return fmt.Errorf("already downloading this file")
// 	}
// 	pc.ActiveDownloads[fileHash] = true
// 	pc.mutex.Unlock()

// 	defer func() {
// 		pc.mutex.Lock()
// 		delete(pc.ActiveDownloads, fileHash)
// 		pc.mutex.Unlock()
// 	}()

// 	// Create the URL for the file request
// 	url := fmt.Sprintf("http://%s:%d/file?name=%s", peer.Address, peer.Port, fileName)

// 	// Send the request
// 	resp, err := pc.httpClient.Get(url)
// 	if err != nil {
// 		return err
// 	}
// 	defer resp.Body.Close()

// 	if resp.StatusCode != http.StatusOK {
// 		body, _ := io.ReadAll(resp.Body)
// 		return fmt.Errorf("download failed: %s", body)
// 	}

// 	// Create the destination file
// 	destPath := filepath.Join(pc.DownloadDir, filepath.Base(fileName))
// 	destFile, err := os.Create(destPath)
// 	if err != nil {
// 		return err
// 	}
// 	defer destFile.Close()

// 	// Copy the file data
// 	_, err = io.Copy(destFile, resp.Body)
// 	if err != nil {
// 		return err
// 	}

// 	log.Printf("Downloaded %s to %s", fileName, destPath)
// 	return nil
// }

// // startCLI starts the command line interface
// func (pc *PeerClient) startCLI() {
// 	fmt.Printf("P2P Client started. ID: %s\n", pc.ID)
// 	fmt.Println("Available commands:")
// 	fmt.Println("  search <query> [limit] - Search for files")
// 	fmt.Println("  download <index> - Download a file from the last search results")
// 	fmt.Println("  list - List local shared files")
// 	fmt.Println("  scan - Rescan shared directory")
// 	fmt.Println("  exit - Exit the program")

// 	var lastSearchResults *SearchResponse

// 	scanner := NewLineScanner()
// 	for {
// 		fmt.Print("> ")
// 		line := scanner.ReadLine()
// 		args := strings.Fields(line)
// 		if len(args) == 0 {
// 			continue
// 		}

// 		switch args[0] {
// 		case "search":
// 			if len(args) < 2 {
// 				fmt.Println("Usage: search <query> [limit]")
// 				continue
// 			}

// 			query := args[1]
// 			limit := 10
// 			if len(args) > 2 {
// 				var err error
// 				limit, err = strconv.Atoi(args[2])
// 				if err != nil {
// 					fmt.Printf("Invalid limit: %v\n", err)
// 					continue
// 				}
// 			}

// 			fmt.Printf("Searching for '%s' (limit: %d)...\n", query, limit)
// 			results, err := pc.Search(query, limit)
// 			if err != nil {
// 				fmt.Printf("Search failed: %v\n", err)
// 				continue
// 			}

// 			lastSearchResults = results

// 			if len(results.Files) == 0 {
// 				fmt.Println("No files found")
// 			} else {
// 				fmt.Println("Search results:")
// 				for i, file := range results.Files {
// 					fmt.Printf("%d. %s (%d bytes) - Available from %d peers\n", i+1, file.Name, file.Size, len(file.PeerIDs))
// 				}
// 			}

// 		case "download":
// 			if lastSearchResults == nil {
// 				fmt.Println("No search results available. Please search first.")
// 				continue
// 			}

// 			if len(args) < 2 {
// 				fmt.Println("Usage: download <index>")
// 				continue
// 			}

// 			index, err := strconv.Atoi(args[1])
// 			if err != nil {
// 				fmt.Printf("Invalid index: %v\n", err)
// 				continue
// 			}

// 			if index < 1 || index > len(lastSearchResults.Files) {
// 				fmt.Printf("Index out of range. Must be between 1 and %d\n", len(lastSearchResults.Files))
// 				continue
// 			}

// 			file := lastSearchResults.Files[index-1]
// 			if len(file.PeerIDs) == 0 {
// 				fmt.Println("No peers available for this file")
// 				continue
// 			}

// 			// Pick the first available peer
// 			peerID := file.PeerIDs[0]
// 			peer, exists := lastSearchResults.Peers[peerID]
// 			if !exists {
// 				fmt.Println("Peer information not available")
// 				continue
// 			}

// 			fmt.Printf("Downloading %s from peer %s...\n", file.Name, peer.ID)
// 			err = pc.DownloadFile(file.Name, file.Hash, peer)
// 			if err != nil {
// 				fmt.Printf("Download failed: %v\n", err)
// 			} else {
// 				fmt.Printf("Download complete: %s\n", file.Name)
// 			}

// 		case "list":
// 			pc.mutex.RLock()
// 			if len(pc.Files) == 0 {
// 				fmt.Println("No shared files")
// 			} else {
// 				fmt.Println("Shared files:")
// 				for i, file := range pc.Files {
// 					fmt.Printf("%d. %s (%d bytes)\n", i+1, file.Name, file.Size)
// 				}
// 			}
// 			pc.mutex.RUnlock()

// 		case "scan":
// 			fmt.Println("Scanning shared directory...")
// 			pc.ScanSharedDirectory()
// 			err := pc.Register()
// 			if err != nil {
// 				fmt.Printf("Failed to update registration: %v\n", err)
// 			} else {
// 				fmt.Println("Registration updated")
// 			}

// 		case "exit":
// 			fmt.Println("Unregistering from super peer...")
// 			pc.Unregister()
// 			fmt.Println("Exiting...")
// 			return

// 		default:
// 			fmt.Println("Unknown command. Available commands: search, download, list, scan, exit")
// 		}
// 	}
// }

// // LineScanner is a helper for reading lines from stdin
// type MyLineScanner struct {
// 	scanner *bufio.Scanner
// }

// // NewLineScanner creates a new line scanner
// func MyNewLineScanner() *MyLineScanner {
// 	return &MyLineScanner{
// 		scanner: bufio.NewScanner(os.Stdin),
// 	}
// }

// // ReadLine reads a line from stdin
// func (ls *MyLineScanner) ReadLine() string {
// 	ls.scanner.Scan()
// 	return ls.scanner.Text()
// }

// func main() {
// 	// Parse command line flags
// 	superPeerURL := flag.String("super", "http://localhost:8080", "URL of the super peer")
// 	localPort := flag.Int("port", 8081, "Local port for the file server")
// 	sharedDir := flag.String("shared", "./shared", "Directory to share files from")
// 	downloadDir := flag.String("download", "./downloads", "Directory to download files to")
// 	flag.Parse()

// 	// Create and start the peer client
// 	client := NewPeerClient(*superPeerURL, *localPort, *sharedDir, *downloadDir)
// 	client.Start()
// }
