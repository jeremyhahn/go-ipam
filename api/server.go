package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/jeremyhahn/go-ipam/pkg/ipam"
	"github.com/jeremyhahn/go-ipam/pkg/store"
)

type Server struct {
	ipam      *ipam.IPAM
	store     ipam.Store
	router    *mux.Router
	raftStore *store.RaftStore // Optional, only set in cluster mode
}

func NewServer(ipamClient *ipam.IPAM, st ipam.Store) *Server {
	s := &Server{
		ipam:   ipamClient,
		store:  st,
		router: mux.NewRouter(),
	}

	// Check if this is a Raft store
	if raftStore, ok := st.(*store.RaftStore); ok {
		s.raftStore = raftStore
	}

	s.setupRoutes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) setupRoutes() {
	// API routes
	api := s.router.PathPrefix("/api/v1").Subrouter()
	api.Use(jsonMiddleware)

	// Network endpoints
	api.HandleFunc("/networks", s.listNetworks).Methods("GET")
	api.HandleFunc("/networks", s.createNetwork).Methods("POST")
	api.HandleFunc("/networks/{id}", s.getNetwork).Methods("GET")
	api.HandleFunc("/networks/{id}", s.deleteNetwork).Methods("DELETE")
	api.HandleFunc("/networks/{id}/stats", s.getNetworkStats).Methods("GET")

	// Allocation endpoints
	api.HandleFunc("/allocations", s.listAllocations).Methods("GET")
	api.HandleFunc("/allocations", s.allocateIP).Methods("POST")
	api.HandleFunc("/allocations/{id}", s.getAllocation).Methods("GET")
	api.HandleFunc("/allocations/{id}/release", s.releaseIP).Methods("POST")

	// Audit endpoints
	api.HandleFunc("/audit", s.listAuditEntries).Methods("GET")

	// Health check
	api.HandleFunc("/health", s.healthCheck).Methods("GET")

	// Cluster endpoints (only available in cluster mode)
	if s.raftStore != nil {
		api.HandleFunc("/cluster/status", s.clusterStatus).Methods("GET")
		api.HandleFunc("/cluster/nodes", s.addNode).Methods("POST")
		api.HandleFunc("/cluster/nodes/{nodeID}", s.removeNode).Methods("DELETE")
	}

}

// Middleware
func jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// Network handlers
func (s *Server) listNetworks(w http.ResponseWriter, r *http.Request) {
	networks, err := s.store.ListNetworks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(networks)
}

func (s *Server) createNetwork(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CIDR        string   `json:"cidr"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	network, err := s.ipam.AddNetwork(req.CIDR, req.Description, req.Tags)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(network)
}

func (s *Server) getNetwork(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	network, err := s.store.GetNetwork(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(network)
}

func (s *Server) deleteNetwork(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// Check for active allocations
	allocations, err := s.store.ListAllocations(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	activeCount := 0
	for _, alloc := range allocations {
		if alloc.ReleasedAt == nil {
			activeCount++
		}
	}

	if activeCount > 0 {
		http.Error(w, "Network has active allocations", http.StatusConflict)
		return
	}

	if err := s.store.DeleteNetwork(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getNetworkStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	stats, err := s.ipam.GetNetworkStats(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(stats)
}

// Allocation handlers
func (s *Server) listAllocations(w http.ResponseWriter, r *http.Request) {
	networkID := r.URL.Query().Get("network_id")
	showAll := r.URL.Query().Get("all") == "true"

	var allAllocations []*ipam.IPAllocation

	if networkID != "" {
		allocations, err := s.store.ListAllocations(networkID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for _, alloc := range allocations {
			if !showAll && alloc.ReleasedAt != nil {
				continue
			}
			allAllocations = append(allAllocations, alloc)
		}
	} else {
		networks, err := s.store.ListNetworks()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for _, network := range networks {
			allocations, err := s.store.ListAllocations(network.ID)
			if err != nil {
				continue
			}

			for _, alloc := range allocations {
				if !showAll && alloc.ReleasedAt != nil {
					continue
				}
				allAllocations = append(allAllocations, alloc)
			}
		}
	}

	json.NewEncoder(w).Encode(allAllocations)
}

func (s *Server) allocateIP(w http.ResponseWriter, r *http.Request) {
	var req ipam.AllocationRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	allocation, err := s.ipam.AllocateIP(&req)
	if err != nil {
		if err == ipam.ErrIPNotAvailable || err == ipam.ErrNetworkFull {
			http.Error(w, err.Error(), http.StatusConflict)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(allocation)
}

func (s *Server) getAllocation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	allocation, err := s.store.GetAllocation(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(allocation)
}

func (s *Server) releaseIP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	allocation, err := s.store.GetAllocation(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err := s.ipam.ReleaseIP(allocation.NetworkID, allocation.IP); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Audit handlers
func (s *Server) listAuditEntries(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			http.Error(w, "Invalid limit parameter", http.StatusBadRequest)
			return
		}
	}

	entries, err := s.store.ListAuditEntries(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(entries)
}

// Health check
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":       "healthy",
		"service":      "ipam",
		"cluster_mode": s.raftStore != nil,
	}
	json.NewEncoder(w).Encode(response)
}

// Cluster handlers

func (s *Server) clusterStatus(w http.ResponseWriter, r *http.Request) {
	if s.raftStore == nil {
		http.Error(w, "Not in cluster mode", http.StatusBadRequest)
		return
	}

	info, err := s.raftStore.GetClusterInfo()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(info)
}

func (s *Server) addNode(w http.ResponseWriter, r *http.Request) {
	if s.raftStore == nil {
		http.Error(w, "Not in cluster mode", http.StatusBadRequest)
		return
	}

	var req struct {
		NodeID uint64 `json:"node_id"`
		Addr   string `json:"addr"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.NodeID == 0 || req.Addr == "" {
		http.Error(w, "node_id and addr are required", http.StatusBadRequest)
		return
	}

	if err := s.raftStore.AddNode(req.NodeID, req.Addr); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) removeNode(w http.ResponseWriter, r *http.Request) {
	if s.raftStore == nil {
		http.Error(w, "Not in cluster mode", http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	nodeIDStr := vars["nodeID"]

	nodeID, err := strconv.ParseUint(nodeIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	if err := s.raftStore.RemoveNode(nodeID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
