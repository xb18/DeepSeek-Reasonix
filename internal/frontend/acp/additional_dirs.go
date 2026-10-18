package acp

import (
	"errors"
	"slices"

	"reasonix/internal/contract/config"
)

const (
	maxAdditionalDirectories = 32
	maxAdditionalDirLength   = 4096

	// CodeAdditionalDirectories is the data.code of a refusal that is about the
	// request as a whole rather than one folder.
	CodeAdditionalDirectories = "acp.additional_directories"
	codeAdditionalDirsTooMany = CodeAdditionalDirectories + ".too_many"
	codeAdditionalDirsChanged = CodeAdditionalDirectories + ".session_open"
	codeAdditionalDirsOff     = CodeAdditionalDirectories + ".disabled"
)

// ErrAdditionalDirectoriesDisabled is what a Factory returns when the agent was
// started to write only inside the session's cwd.
var ErrAdditionalDirectoriesDisabled = errors.New("this agent writes only inside the session's cwd")

// AdditionalDirsData is the data of an invalid-params error about the folders a
// client asked to add: a stable code, and the folder it concerns when one does.
type AdditionalDirsData struct {
	Code string `json:"code"`
	Path string `json:"path,omitempty"`
}

// vetAdditionalDirs resolves the folders a client declared as part of the
// session's workspace. They get the vetting a user-granted write root gets, one
// refusal for the whole request: a folder dropped silently would leave the client
// believing the agent can write where it cannot. The result is this session's
// alone; nothing is written to the user's grants.
func vetAdditionalDirs(cwd string, dirs []string) ([]string, *RPCError) {
	if len(dirs) == 0 {
		return nil, nil
	}
	if len(dirs) > maxAdditionalDirectories {
		return nil, additionalDirsError("too many additional directories", AdditionalDirsData{Code: codeAdditionalDirsTooMany})
	}
	cfg, err := config.LoadForRootReadOnly(cwd)
	if err != nil {
		return nil, &RPCError{Code: ErrInternal, Message: err.Error()}
	}
	var out []string
	for _, dir := range dirs {
		if len(dir) > maxAdditionalDirLength {
			return nil, additionalDirsError("additional directory path is too long", AdditionalDirsData{Code: string(config.RootNotDirectory)})
		}
		real, err := cfg.VetExtraRoot(cwd, dir, out)
		if err != nil {
			var refusal *config.WorkspaceRootError
			if errors.As(err, &refusal) {
				return nil, additionalDirsError("this folder cannot be added", AdditionalDirsData{Code: string(refusal.Code), Path: dir})
			}
			return nil, &RPCError{Code: ErrInternal, Message: err.Error()}
		}
		if !slices.Contains(out, real) {
			out = append(out, real)
		}
	}
	return out, nil
}

func additionalDirsError(msg string, data AdditionalDirsData) *RPCError {
	return &RPCError{Code: ErrInvalidParams, Message: msg, Data: data}
}

func sameDirs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	a, b = slices.Sorted(slices.Values(a)), slices.Sorted(slices.Values(b))
	return slices.Equal(a, b)
}

// newSessionError is the error for a Factory that could not build the session:
// a refusal about the extra folders is the client's to fix, anything else is ours.
func newSessionError(method string, err error) *RPCError {
	if errors.Is(err, ErrAdditionalDirectoriesDisabled) {
		return additionalDirsError(method+": "+err.Error(), AdditionalDirsData{Code: codeAdditionalDirsOff})
	}
	return sessionOpenError(method, err)
}
