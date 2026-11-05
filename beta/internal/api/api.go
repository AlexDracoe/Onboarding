// Package api serves the HTTP contract the React dashboard uses (same as the Python service).
// Thin on purpose: parse the request, call the app layer, write JSON. No command logic here.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"beta/internal/app"
	"beta/internal/domain"
	"beta/internal/store"
)

const (
	defaultLimit = 50
	maxLimit     = 500
)

// Consumer-side interfaces: *app.StateManager, *app.VehicleService and *store.SQLite satisfy them.
type vehicleSource interface {
	Get(id uint8) (domain.VehicleState, bool)
	List() []domain.VehicleState
}

type commandSubmitter interface {
	Submit(systemID uint8, name string) (string, error)
}

type commandRecords interface {
	Get(id string) (store.Record, bool, error)
	List(systemID *uint8, limit int) ([]store.Record, error)
}

// Response bodies as structs so the JSON keys keep the contract order (maps are sorted by key).
type errorBody struct {
	Error  string `json:"error"`
	Detail string `json:"detail"`
}

type commandAccepted struct {
	CommandID string `json:"command_id"`
	Status    string `json:"status"`
}

type server struct {
	vehicles vehicleSource
	commands commandSubmitter
	records  commandRecords
}

// NewHandler returns the router with every contract route.
func NewHandler(vehicles vehicleSource, commands commandSubmitter, records commandRecords) http.Handler {
	s := &server{vehicles: vehicles, commands: commands, records: records}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /vehicles/{system_id}/commands/{name}", s.submitCommand)
	mux.HandleFunc("GET /vehicles", s.listVehicles)
	mux.HandleFunc("GET /vehicles/{system_id}", s.getVehicle)
	mux.HandleFunc("GET /commands", s.listCommands)
	mux.HandleFunc("GET /commands/{command_id}", s.getCommand)
	return mux
}

func (s *server) submitCommand(w http.ResponseWriter, r *http.Request) {
	systemID, ok := pathSystemID(w, r)
	if !ok {
		return
	}
	if _, err := readParams(r); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_PARAMS", err.Error())
		return
	}
	// TODO(5c part 2): pass the params to Submit and map its validation error to 422 INVALID_PARAMS.

	name := r.PathValue("name")
	id, err := s.commands.Submit(systemID, name)
	if errors.Is(err, app.ErrUnknownCommand) {
		writeError(w, http.StatusNotFound, "UNKNOWN_COMMAND", "Unknown command: "+name)
		return
	}
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, commandAccepted{CommandID: id, Status: "PENDING"})
}

func (s *server) listVehicles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.vehicles.List())
}

func (s *server) getVehicle(w http.ResponseWriter, r *http.Request) {
	systemID, ok := pathSystemID(w, r)
	if !ok {
		return
	}
	v, found := s.vehicles.Get(systemID)
	if !found {
		writeError(w, http.StatusNotFound, "NO_VEHICLE", fmt.Sprintf("No telemetry from system %d yet", systemID))
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *server) listCommands(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit := defaultLimit
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxLimit {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_PARAMS",
				fmt.Sprintf("limit: must be an integer from 1 to %d", maxLimit))
			return
		}
		limit = n
	}

	var systemID *uint8
	if raw := q.Get("system_id"); raw != "" {
		id, err := parseSystemID(raw)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_PARAMS", err.Error())
			return
		}
		systemID = &id
	}

	recs, err := s.records.List(systemID, limit)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, recs)
}

func (s *server) getCommand(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("command_id")
	rec, found, err := s.records.Get(id)
	if err != nil {
		internalError(w, err)
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "UNKNOWN_COMMAND_ID", "No command "+id)
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

// readParams decodes the optional JSON body. No body means no params; anything else must be a JSON object.
func readParams(r *http.Request) (map[string]any, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil {
		return nil, fmt.Errorf("body: %v", err)
	}
	params := map[string]any{}
	if len(body) == 0 {
		return params, nil
	}
	if err := json.Unmarshal(body, &params); err != nil || params == nil {
		return nil, errors.New("body: must be a JSON object")
	}
	return params, nil
}

// pathSystemID parses {system_id}; on failure it writes the 422 and returns ok == false.
func pathSystemID(w http.ResponseWriter, r *http.Request) (uint8, bool) {
	id, err := parseSystemID(r.PathValue("system_id"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_PARAMS", err.Error())
		return 0, false
	}
	return id, true
}

func parseSystemID(raw string) (uint8, error) {
	n, err := strconv.ParseUint(raw, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("system_id: must be an integer from 0 to 255, got %q", raw)
	}
	return uint8(n), nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("api: write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, detail string) {
	writeJSON(w, status, errorBody{Error: code, Detail: detail})
}

func internalError(w http.ResponseWriter, err error) {
	log.Printf("api: %v", err)
	writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
}
