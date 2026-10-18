package serve

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"reasonix/internal/session/control"
)

func (s *Server) toolSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, s.ctl().ToolSettings())
}

// saveToolDisabled rides the provider-edit grant like the other writes to the
// user file: turning a tool off changes what every later session can do.
func (s *Server) saveToolDisabled(w http.ResponseWriter, r *http.Request) {
	if !s.grants.at(r).providerEdit {
		refuse(w, http.StatusForbidden, "tools.editing_disabled", "tool editing is not enabled on this server", nil)
		return
	}
	var body struct {
		Name     string `json:"name"`
		Disabled *bool  `json:"disabled"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		badBody(w)
		return
	}
	if strings.TrimSpace(body.Name) == "" || body.Disabled == nil {
		refuse(w, http.StatusBadRequest, "tools.no_choice", "name and disabled are required", nil)
		return
	}
	if err := s.ctl().SaveToolDisabled(body.Name, *body.Disabled); err != nil {
		var required *control.ToolRequiredError
		if errors.As(err, &required) {
			refuse(w, http.StatusConflict, "tools.host_required", "this tool is needed by another feature", map[string]any{"tool": required.Tool, "by": required.By})
			return
		}
		saveFailed(w, http.StatusInternalServerError, "tools.save_failed", err)
		return
	}
	writeJSON(w, s.ctl().ToolSettings())
}
