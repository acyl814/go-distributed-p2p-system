// SearchRequest represents a search query to the super peer
type SearchRequest struct {
	Query    string `json:"query"`
	Limit    int    `json:"limit"`
	FromPeer string `json:"fromPeer"`
}

// SearchResponse represents the response from the super peer
type SearchResponse struct {
	Files []File           `json:"files"`
	Peers map[string]*Peer `json:"peers"`
}

// PeerClient is the client that communicates with the super peer and other peers
type PeerClient struct {
	ID              string
	SuperPeerURL    string
	LocalPort       int
	WebPort         int
	SharedDir       string
	DownloadDir     string
	Files           []File
	ActiveDownloads map[string]struct {
		Progress int
		Total    int64
	}
	mutex        sync.RWMutex
	httpClient   *http.Client
	searchResults []File
	resultPeers   map[string]*Peer
	statusMessage string
}

// NewPeerClient creates a new peer client
func NewPeerClient(superPeerURL string, localPort, webPort int, sharedDir, downloadDir string) *PeerClient {
	// Generate a random ID
	rand.Seed(time.Now().UnixNano())
	id := fmt.Sprintf("peer-%d", rand.Intn(10000))

	return &PeerClient{
		ID:              id,
		SuperPeerURL:    superPeerURL,
		LocalPort:       localPort,
		WebPort:         webPort,
		SharedDir:       sharedDir,
		DownloadDir:     downloadDir,
		Files:           []File{},
		ActiveDownloads: make(map[string]struct {
			Progress int
			Total    int64
		}),
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		searchResults: []File{},
		resultPeers:   make(map[string]*Peer),
		statusMessage: "Ready",
	}
}

// Start starts the peer client
func (pc *PeerClient) Start() {
	// Create directories if they don't exist
	os.MkdirAll(pc.SharedDir, 0755)
	os.MkdirAll(pc.DownloadDir, 0755)

	// Scan shared directory for files
	pc.ScanSharedDirectory()

	// Register with super peer
	err := pc.Register()
	if err != nil {
		log.Fatalf("Failed to register with super peer: %v", err)
	}

	// Start heartbeat service
	go pc.heartbeatService()

	// Start file server
	go pc.startFileServer()

	// Start web UI
	pc.startWebUI()
}

// ScanSharedDirectory scans the shared directory for files
func (pc *PeerClient) ScanSharedDirectory() {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	pc.Files = []File{}

	err := filepath.Walk(pc.SharedDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			relPath, err := filepath.Rel(pc.SharedDir, path)
			if err != nil {
				return err
			}

			// Calculate file hash
			hash, err := pc.calculateFileHash(path)
			if err != nil {
				log.Printf("Failed to calculate hash for %s: %v", path, err)
				return nil
			}

			file := File{
				Name:    relPath,
				Hash:    hash,
				Size:    info.Size(),
				PeerIDs: []string{pc.ID},
			}

			pc.Files = append(pc.Files, file)
		}

		return nil
	})

	if err != nil {
		log.Printf("Error scanning shared directory: %v", err)
	}

	log.Printf("Found %d files in shared directory", len(pc.Files))
}

// calculateFileHash calculates the SHA-256 hash of a file
func (pc *PeerClient) calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Register registers the peer with the super peer
func (pc *PeerClient) Register() error {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()

	peer := Peer{
		ID:      pc.ID,
		Address: "localhost", // This will be overridden by the super peer
		Port:    pc.LocalPort,
		Files:   pc.Files,
	}

	jsonData, err := json.Marshal(peer)
	if err != nil {
		return err
	}

	resp, err := pc.httpClient.Post(pc.SuperPeerURL+"/register", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to register: %s", body)
	}

	log.Printf("Registered with super peer as %s", pc.ID)
	return nil
}

// Unregister unregisters the peer from the super peer
func (pc *PeerClient) Unregister() error {
	data := map[string]string{"peerId": pc.ID}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	resp, err := pc.httpClient.Post(pc.SuperPeerURL+"/unregister", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to unregister: %s", body)
	}

	log.Printf("Unregistered from super peer")
	return nil
}

// Search searches for files via the super peer
func (pc *PeerClient) Search(query string, limit int) (*SearchResponse, error) {
	req := SearchRequest{
		Query:    query,
		Limit:    limit,
		FromPeer: pc.ID,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := pc.httpClient.Post(pc.SuperPeerURL+"/search", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed: %s", body)
	}

	var searchResp SearchResponse
	err = json.NewDecoder(resp.Body).Decode(&searchResp)
	if err != nil {
		return nil, err
	}

	return &searchResp, nil
}

// SendHeartbeat sends a heartbeat to the super peer
func (pc *PeerClient) SendHeartbeat() error {
	data := map[string]string{"peerId": pc.ID}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	resp, err := pc.httpClient.Post(pc.SuperPeerURL+"/heartbeat", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("heartbeat failed: %s", body)
	}

	return nil
}

// heartbeatService periodically sends heartbeats to the super peer
func (pc *PeerClient) heartbeatService() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		<-ticker.C
		err := pc.SendHeartbeat()
		if err != nil {
			log.Printf("Failed to send heartbeat: %v", err)
		}
	}
}

// startFileServer starts the HTTP server for serving files to other peers
func (pc *PeerClient) startFileServer() {
	// File request handler
	http.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		fileName := r.URL.Query().Get("name")
		if fileName == "" {
			http.Error(w, "Missing file name", http.StatusBadRequest)
			return
		}

		// Sanitize the file name to prevent directory traversal
		fileName = filepath.Clean(fileName)
		if strings.Contains(fileName, "..") {
			http.Error(w, "Invalid file name", http.StatusBadRequest)
			return
		}

		filePath := filepath.Join(pc.SharedDir, fileName)
		file, err := os.Open(filePath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to open file: %v", err), http.StatusNotFound)
			return
		}
		defer file.Close()

		// Get file info
		fileInfo, err := file.Stat()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get file info: %v", err), http.StatusInternalServerError)
			return
		}

		// Set content type and length
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(fileName)))

		// Copy the file to the response
		_, err = io.Copy(w, file)
		if err != nil {
			log.Printf("Error sending file: %v", err)
		}
	})

	// Start the server
	addr := fmt.Sprintf(":%d", pc.LocalPort)
	log.Printf("Starting file server on %s", addr)
	go func() {
		err := http.ListenAndServe(addr, nil)
		if err != nil {
			log.Fatalf("Failed to start file server: %v", err)
		}
	}()
}

// DownloadFile downloads a file from another peer
func (pc *PeerClient) DownloadFile(fileName, fileHash string, peer *Peer) error {
	pc.mutex.Lock()
	if _, exists := pc.ActiveDownloads[fileHash]; exists {
		pc.mutex.Unlock()
		return fmt.Errorf("already downloading this file")
	}
	
	pc.ActiveDownloads[fileHash] = struct {
		Progress int
		Total    int64
	}{
		Progress: 0,
		Total:    100, // Will be updated with actual size
	}
	pc.mutex.Unlock()

	defer func() {
		pc.mutex.Lock()
		delete(pc.ActiveDownloads, fileHash)
		pc.mutex.Unlock()
	}()

	// Create the URL for the file request
	url := fmt.Sprintf("http://%s:%d/file?name=%s", peer.Address, peer.Port, fileName)

	// Send the request
	resp, err := pc.httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download failed: %s", body)
	}

	// Get content length
	contentLength := resp.ContentLength
	if contentLength > 0 {
		pc.mutex.Lock()
		download := pc.ActiveDownloads[fileHash]
		download.Total = contentLength
		pc.ActiveDownloads[fileHash] = download
		pc.mutex.Unlock()
	}

	// Create the destination file in downloads directory
	destPath := filepath.Join(pc.DownloadDir, filepath.Base(fileName))
	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Create a buffer for reading
	buf := make([]byte, 32*1024)
	var totalRead int64

	// Read and write in chunks to update progress
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := destFile.Write(buf[:n])
			if writeErr != nil {
				return writeErr
			}
			
			totalRead += int64(n)
			
			// Update progress
			if contentLength > 0 {
				progress := int(float64(totalRead) / float64(contentLength) * 100)
				pc.mutex.Lock()
				download := pc.ActiveDownloads[fileHash]
				download.Progress = progress
				pc.ActiveDownloads[fileHash] = download
				pc.mutex.Unlock()
			}
		}
		
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	log.Printf("Downloaded %s to %s", fileName, destPath)
	
	// Also copy the file to the shared directory to make it available to other peers
	sharedPath := filepath.Join(pc.SharedDir, filepath.Base(fileName))
	err = copyFile(destPath, sharedPath)
	if err != nil {
		log.Printf("Warning: Failed to copy file to shared directory: %v", err)
	} else {
		log.Printf("Copied %s to shared directory for sharing", fileName)
		
		// Rescan shared directory and update registration
		pc.ScanSharedDirectory()
		err = pc.Register()
		if err != nil {
			log.Printf("Warning: Failed to update registration after download: %v", err)
		} else {
			log.Printf("Updated registration to share downloaded file")
		}
	}
	
	return nil
}

// Helper function to copy a file
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return nil
}

// GetDownloadProgress returns the progress of a download
func (pc *PeerClient) GetDownloadProgress(fileHash string) (int, bool) {
	pc.mutex.RLock()
	defer pc.mutex.RUnlock()
	
	if download, exists := pc.ActiveDownloads[fileHash]; exists {
		return download.Progress, true
	}
	return 0, false
}

// startWebUI starts the web-based user interface
func (pc *PeerClient) startWebUI() {
	// Serve static files
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		// Extract the file path from the URL
		filePath := r.URL.Path[len("/static/"):]
		
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
				
				.peer-info {
					display: flex;
					align-items: center;
				}
				
				.peer-id {
					background-color: var(--primary-color);
					color: white;
					padding: 5px 10px;
					border-radius: 20px;
					font-size: 0.9rem;
					margin-right: 10px;
				}
				
				.status {
					background-color: var(--accent-color);
					color: white;
					padding: 15px;
					border-radius: 10px;
					margin-bottom: 20px;
					animation: fadeIn 0.5s ease;
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
					transition: border-color 0.3s ease;
				}
				
				.search-form input:focus {
					outline: none;
					border-color: var(--primary-color);
				}
				
				.search-form button {
					padding: 12px 20px;
					background-color: var(--primary-color);
					color: white;
					border: none;
					border-radius: 0 8px 8px 0;
					cursor: pointer;
					font-size: 1rem;
					transition: background-color 0.3s ease;
				}
				
				.search-form button:hover {
					background-color: var(--secondary-color);
				}
				
				.button {
					padding: 10px 15px;
					background-color: var(--primary-color);
					color: white;
					border: none;
					border-radius: 8px;
					cursor: pointer;
					text-decoration: none;
					display: inline-block;
					margin-right: 10px;
					font-size: 0.9rem;
					transition: background-color 0.3s ease, transform 0.2s ease;
				}
				
				.button:hover {
					background-color: var(--secondary-color);
					transform: scale(1.05);
				}
				
				.button.secondary {
					background-color: var(--accent-color);
				}
				
				.button.secondary:hover {
					background-color: var(--success-color);
				}
				
				.button.danger {
					background-color: var(--warning-color);
				}
				
				.button.danger:hover {
					background-color: #d90429;
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
				
				.file-icon {
					margin-right: 10px;
					color: var(--primary-color);
				}
				
				.badge {
					display: inline-block;
					padding: 3px 8px;
					border-radius: 20px;
					font-size: 0.8rem;
					font-weight: 600;
					background-color: var(--accent-color);
					color: white;
				}
				
				.progress-container {
					width: 100%;
					height: 8px;
					background-color: #e9ecef;
					border-radius: 4px;
					overflow: hidden;
					margin-top: 5px;
				}
				
				.progress-bar {
					height: 100%;
					background-color: var(--success-color);
					border-radius: 4px;
					transition: width 0.3s ease;
				}
				
				.empty-state {
					text-align: center;
					padding: 30px;
					color: #6c757d;
				}
				
				.empty-state i {
					font-size: 3rem;
					margin-bottom: 15px;
					color: #dee2e6;
				}
				
				@keyframes fadeIn {
					from { opacity: 0; transform: translateY(-10px); }
					to { opacity: 1; transform: translateY(0); }
				}
				
				@keyframes pulse {
					0% { transform: scale(1); }
					50% { transform: scale(1.05); }
					100% { transform: scale(1); }
				}
				
				.animate-pulse {
					animation: pulse 2s infinite;
				}
				
				.file-row {
					animation: fadeIn 0.5s ease;
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
				
				@media (max-width: 768px) {
					.header {
						flex-direction: column;
						align-items: flex-start;
					}
					
					.peer-info {
						margin-top: 10px;
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
					
					// Update progress bars for active downloads
					function updateDownloadProgress() {
						const progressBars = document.querySelectorAll('[data-file-hash]');
						if (progressBars.length > 0) {
							fetch('/api/download-progress')
								.then(response => response.json())
								.then(data => {
									progressBars.forEach(progressBar => {
										const fileHash = progressBar.getAttribute('data-file-hash');
										if (data[fileHash]) {
											progressBar.style.width = data[fileHash] + '%';
											progressBar.parentElement.parentElement.querySelector('.progress-text').textContent = 
												data[fileHash] + '% Complete';
										}
									});
								})
								.catch(error => console.error('Error fetching download progress:', error));
						}
					}
					
					// Update progress every second
					setInterval(updateDownloadProgress, 1000);
					
					// Add animation to the status message
					const statusElement = document.querySelector('.status');
					if (statusElement) {
						statusElement.classList.add('animate-pulse');
						setTimeout(() => {
							statusElement.classList.remove('animate-pulse');
						}, 2000);
					}
				});
			`))
		default:
			http.NotFound(w, r)
		}
	})

	// API endpoint for download progress
	http.HandleFunc("/api/download-progress", func(w http.ResponseWriter, r *http.Request) {
		pc.mutex.RLock()
		progress := make(map[string]int)
		for hash, download := range pc.ActiveDownloads {
			progress[hash] = download.Progress
		}
		pc.mutex.RUnlock()
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(progress)
	})

	// HTML template for the web UI
	const htmlTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>P2P File Sharing Client</title>
    <link rel="stylesheet" href="/static/styles.css">
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0-beta3/css/all.min.css">
</head>
<body>
    <div class="container">
        <div class="card">
            <div class="header">
                <h1><i class="fas fa-network-wired"></i> P2P File Sharing</h1>
                <div class="peer-info">
                    <span class="peer-id">{{.ID}}</span>
                    <a href="/scan" class="button secondary"><i class="fas fa-sync-alt"></i> Scan</a>
                    <a href="/exit" class="button danger"><i class="fas fa-sign-out-alt"></i> Exit</a>
                    <button id="theme-toggle" class="theme-toggle">🌙</button>
                </div>
            </div>
            
            <div class="status">
                <i class="fas fa-info-circle"></i> {{.StatusMessage}}
            </div>
            
            <div class="section">
                <div class="section-header">
                    <h2><i class="fas fa-share-alt"></i> Shared Files</h2>
                </div>
                <table>
                    <thead>
                        <tr>
                            <th>Name</th>
                            <th>Size</th>
                            <th>Hash</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .Files}}
                        <tr class="file-row">
                            <td><i class="fas fa-file file-icon"></i> {{.Name}}</td>
                            <td>{{formatSize .Size}}</td>
                            <td>{{truncateHash .Hash}}</td>
                        </tr>
                        {{else}}
                        <tr>
                            <td colspan="3" class="empty-state">
                                <i class="fas fa-folder-open"></i>
                                <p>No shared files</p>
                            </td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
            
            <div class="section">
                <div class="section-header">
                    <h2><i class="fas fa-search"></i> Search Files</h2>
                </div>
                <form class="search-form" action="/search" method="get">
                    <input type="text" name="query" placeholder="Enter search term" required>
                    <button type="submit"><i class="fas fa-search"></i> Search</button>
                </form>
                
                {{if .SearchResults}}
                <table>
                    <thead>
                        <tr>
                            <th>Name</th>
                            <th>Size</th>
                            <th>Available From</th>
                            <th>Action</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range $index, $file := .SearchResults}}
                        <tr class="file-row">
                            <td><i class="fas fa-file file-icon"></i> {{$file.Name}}</td>
                            <td>{{formatSize $file.Size}}</td>
                            <td><span class="badge">{{len $file.PeerIDs}} peers</span></td>
                            <td>
                                {{if isDownloading $file.Hash}}
                                <div>
                                    <span class="progress-text">Downloading...</span>
                                    <div class="progress-container">
                                        <div class="progress-bar" data-file-hash="{{$file.Hash}}" style="width: 0%"></div>
                                    </div>
                                </div>
                                {{else}}
                                <a href="/download?index={{$index}}" class="button"><i class="fas fa-download"></i> Download</a>
                                {{end}}
                            </td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
                {{else}}
                {{if .SearchPerformed}}
                <div class="empty-state">
                    <i class="fas fa-search"></i>
                    <p>No files found matching your search.</p>
                </div>
                {{end}}
                {{end}}
            </div>
            
            <div class="section">
                <div class="section-header">
                    <h2><i class="fas fa-download"></i> Downloaded Files</h2>
                </div>
                <table>
                    <thead>
                        <tr>
                            <th>Name</th>
                            <th>Size</th>
                            <th>Action</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .DownloadedFiles}}
                        <tr class="file-row">
                            <td><i class="fas fa-file-download file-icon"></i> {{.Name}}</td>
                            <td>{{formatSize .Size}}</td>
                            <td><a href="/downloaded/{{.Name}}" class="button secondary"><i class="fas fa-eye"></i> Open</a></td>
                        </tr>
                        {{else}}
                        <tr>
                            <td colspan="3" class="empty-state">
                                <i class="fas fa-download"></i>
                                <p>No downloaded files</p>
                            </td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
        </div>
    </div>
    
    <script src="/static/script.js"></script>
</body>
</html>
`

	// Create template functions
	funcMap := template.FuncMap{
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
		"isDownloading": func(hash string) bool {
			pc.mutex.RLock()
			defer pc.mutex.RUnlock()
			_, exists := pc.ActiveDownloads[hash]
			return exists
		},
	}

	// Parse the HTML template
	tmpl, err := template.New("index").Funcs(funcMap).Parse(htmlTemplate)
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}

	// Handler for the main page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		// Get downloaded files
		downloadedFiles := []struct {
			Name string
			Size int64
		}{}

		err := filepath.Walk(pc.DownloadDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				downloadedFiles = append(downloadedFiles, struct {
					Name string
					Size int64
				}{
					Name: filepath.Base(path),
					Size: info.Size(),
				})
			}

			return nil
		})

		if err != nil {
			log.Printf("Error scanning download directory: %v", err)
		}

		// Prepare template data
		data := struct {
			ID              string
			StatusMessage   string
			Files           []File
			SearchResults   []File
			SearchPerformed bool
			DownloadedFiles []struct {
				Name string
				Size int64
			}
		}{
			ID:              pc.ID,
			StatusMessage:   pc.statusMessage,
			Files:           pc.Files,
			SearchResults:   pc.searchResults,
			SearchPerformed: len(pc.searchResults) > 0,
			DownloadedFiles: downloadedFiles,
		}

		// Execute the template
		w.Header().Set("Content-Type", "text/html")
		err = tmpl.Execute(w, data)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
		}
	})

	// Handler for scanning the shared directory
	http.HandleFunc("/scan", func(w http.ResponseWriter, r *http.Request) {
		pc.statusMessage = "Scanning shared directory..."
		pc.ScanSharedDirectory()
		err := pc.Register()
		if err != nil {
			pc.statusMessage = fmt.Sprintf("Failed to update registration: %v", err)
		} else {
			pc.statusMessage = fmt.Sprintf("Found %d files in shared directory", len(pc.Files))
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	// Handler for searching files
	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if query == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		pc.statusMessage = fmt.Sprintf("Searching for '%s'...", query)
		results, err := pc.Search(query, 50)
		if err != nil {
			pc.statusMessage = fmt.Sprintf("Search failed: %v", err)
		} else {
			pc.searchResults = results.Files
			pc.resultPeers = results.Peers

			if len(results.Files) == 0 {
				pc.statusMessage = "No files found"
			} else {
				pc.statusMessage = fmt.Sprintf("Found %d files", len(results.Files))
			}
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	// Handler for downloading files
	http.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		indexStr := r.URL.Query().Get("index")
		if indexStr == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		index, err := strconv.Atoi(indexStr)
		if err != nil || index < 0 || index >= len(pc.searchResults) {
			pc.statusMessage = "Invalid file index"
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		file := pc.searchResults[index]
		if len(file.PeerIDs) == 0 {
			pc.statusMessage = "No peers available for this file"
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		// Pick the first available peer
		peerID := file.PeerIDs[0]
		peer, exists := pc.resultPeers[peerID]
		if !exists {
			pc.statusMessage = "Peer information not available"
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		pc.statusMessage = fmt.Sprintf("Downloading %s from peer %s...", file.Name, peer.ID)
		go func() {
			err := pc.DownloadFile(file.Name, file.Hash, peer)
			if err != nil {
				pc.statusMessage = fmt.Sprintf("Download failed: %v", err)
			} else {
				pc.statusMessage = fmt.Sprintf("Download complete: %s", file.Name)
			}
		}()

		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	// Handler for serving downloaded files
	http.HandleFunc("/downloaded/", func(w http.ResponseWriter, r *http.Request) {
		fileName := strings.TrimPrefix(r.URL.Path, "/downloaded/")
		if fileName == "" {
			http.NotFound(w, r)
			return
		}

		// Sanitize the file name to prevent directory traversal
		fileName = filepath.Clean(fileName)
		if strings.Contains(fileName, "..") {
			http.Error(w, "Invalid file name", http.StatusBadRequest)
			return
		}

		filePath := filepath.Join(pc.DownloadDir, fileName)
		http.ServeFile(w, r, filePath)
	})

	// Handler for exiting the program
	http.HandleFunc("/exit", func(w http.ResponseWriter, r *http.Request) {
		pc.statusMessage = "Unregistering from super peer..."
		pc.Unregister()
		
		// Return a page that says the program is shutting down
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<!DOCTYPE html>
			<html>
			<head>
				<title>P2P Client Shutting Down</title>
				<style>
					body {
						font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
						margin: 0;
						padding: 20px;
						text-align: center;
						background-color: #f8f9fa;
						color: #333;
					}
					.container {
						max-width: 600px;
						margin: 100px auto;
						padding: 40px;
						background-color: white;
						border-radius: 10px;
						box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
						animation: fadeIn 0.5s ease;
					}
					h1 {
						color: #4361ee;
						margin-bottom: 20px;
					}
					p {
						font-size: 1.2rem;
						margin-bottom: 30px;
					}
					.icon {
						font-size: 4rem;
						color: #4361ee;
						margin-bottom: 20px;
						animation: pulse 2s infinite;
					}
					@keyframes fadeIn {
						from { opacity: 0; transform: translateY(-20px); }
						to { opacity: 1; transform: translateY(0); }
					}
					@keyframes pulse {
						0% { transform: scale(1); }
						50% { transform: scale(1.1); }
						100% { transform: scale(1); }
					}
				</style>
				<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0-beta3/css/all.min.css">
			</head>
			<body>
				<div class="container">
					<div class="icon"><i class="fas fa-power-off"></i></div>
					<h1>P2P Client is shutting down</h1>
					<p>Thank you for using our P2P File Sharing system.</p>
					<p>You can close this window now.</p>
				</div>
			</body>
			</html>
		`))
		
		// Shutdown the program after a short delay
		go func() {
			time.Sleep(2 * time.Second)
			os.Exit(0)
		}()
	})

	// Start the web server
	addr := fmt.Sprintf(":%d", pc.WebPort)
	log.Printf("Starting web UI on http://localhost:%d", pc.WebPort)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func main() {
	// Parse command line flags
	superPeerURL := flag.String("super", "http://localhost:8080", "URL of the super peer")
	localPort := flag.Int("port", 8081, "Local port for the file server")
	webPort := flag.Int("webport", 8090, "Port for the web UI")
	sharedDir := flag.String("shared", "./shared", "Directory to share files from")
	downloadDir := flag.String("download", "./downloads", "Directory to download files to")
	flag.Parse()

	// Create and start the peer client
	client := NewPeerClient(*superPeerURL, *localPort, *webPort, *sharedDir, *downloadDir)
	client.Start()
}
