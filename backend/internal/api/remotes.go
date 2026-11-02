package api

import (
	"encoding/json"
	"net/http"

	"housecontrol/internal/devices"
	"housecontrol/internal/remote"
)

type remoteView struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Vendor       string          `json:"vendor,omitempty"`
	Model        string          `json:"model,omitempty"`
	Description  string          `json:"description,omitempty"`
	Availability string          `json:"availability"`
	Buttons      []remote.Button `json:"buttons"`
	// Defaults: what the buttons do until changed (shown for a new remote).
	Defaults []remote.Action `json:"defaults"`
}

// remotesView is Settings → Remotes: the remotes zigbee2mqtt knows, their
// buttons, and what each controls. Configs for remotes that are gone are kept.
func (s *Server) remotesView() (map[string]any, error) {
	configs, err := s.store.LoadRemotes()
	if err != nil {
		return nil, err
	}
	if configs == nil {
		configs = []remote.Config{}
	}
	list := []remoteView{}
	for _, d := range s.reg.List() {
		if d.Kind != devices.KindRemote {
			continue
		}
		buttons := remote.Buttons(d.Capabilities.Actions)
		defaults := remote.Defaults(buttons)
		if defaults == nil {
			defaults = []remote.Action{}
		}
		list = append(list, remoteView{ID: d.ID, Name: d.Name, Vendor: d.Vendor, Model: d.Model,
			Description: d.Description, Availability: d.Availability, Buttons: buttons, Defaults: defaults})
	}
	return map[string]any{"remotes": list, "configs": configs}, nil
}

func (s *Server) getRemotes(w http.ResponseWriter, _ *http.Request) {
	v, err := s.remotesView()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) putRemotes(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Configs []remote.Config `json:"configs"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	list, err := remote.NormalizeConfigs(in.Configs)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.SaveRemotes(list); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not save: "+err.Error())
		return
	}
	s.getRemotes(w, r)
}
