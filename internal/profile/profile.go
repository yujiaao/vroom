package profile

import (
	"encoding/json"

	"github.com/getsentry/vroom/internal/metadata"
	"github.com/getsentry/vroom/internal/nodetree"
	"github.com/getsentry/vroom/internal/occurrence"
	"github.com/getsentry/vroom/internal/sample"
	"github.com/getsentry/vroom/internal/speedscope"
)

type (
	profileInterface interface {
		GetID() string
		GetOrganizationID() uint64
		GetProjectID() uint64
		GetPlatform() string

		Metadata() metadata.Metadata
		Raw() []byte

		CallTrees() (map[uint64][]*nodetree.Node, error)
		Occurrences() []occurrence.Occurrence
		ReplaceIdleStacks()
		Speedscope() (speedscope.Output, error)
		StoragePath() string
	}

	Profile struct {
		version string

		profile profileInterface

		sample *sample.SampleProfile
		legacy *LegacyProfile
	}

	version struct {
		Version string `json:"version"`
	}
)

func (p *Profile) UnmarshalJSON(b []byte) error {
	var v version
	err := json.Unmarshal(b, &v)
	if err != nil {
		return err
	}
	p.version = v.Version
	switch p.version {
	case "":
		p.legacy = new(LegacyProfile)
		p.profile = p.legacy
		return json.Unmarshal(b, &p.legacy)
	default:
		p.sample = new(sample.SampleProfile)
		p.profile = p.sample
		return json.Unmarshal(b, &p.sample)
	}
}

func (p Profile) MarshalJSON() ([]byte, error) {
	switch p.version {
	case "":
		return json.Marshal(p.legacy)
	default:
		return json.Marshal(p.sample)
	}
}

func (p *Profile) CallTrees() (map[uint64][]*nodetree.Node, error) {
	callTrees, err := p.profile.CallTrees()
	if err != nil {
		return callTrees, err
	}

	for threadId, callTreesForThread := range callTrees {
		collapsedCallTrees := make([]*nodetree.Node, 0, len(callTreesForThread))
		for _, callTree := range callTreesForThread {
			collapsedCallTrees = append(collapsedCallTrees, callTree.Collapse()...)
		}
		callTrees[threadId] = collapsedCallTrees
	}

	return callTrees, nil
}

func (p *Profile) ID() string {
	return p.profile.GetID()
}

func (p *Profile) OrganizationID() uint64 {
	return p.profile.GetOrganizationID()
}

func (p *Profile) ProjectID() uint64 {
	return p.profile.GetProjectID()
}

func (p *Profile) StoragePath() string {
	return p.profile.StoragePath()
}

func (p *Profile) Speedscope() (speedscope.Output, error) {
	return p.profile.Speedscope()
}

func (p *Profile) Metadata() metadata.Metadata {
	return p.profile.Metadata()
}

func (p *Profile) Platform() string {
	return p.profile.GetPlatform()
}

func (p *Profile) Raw() []byte {
	return p.profile.Raw()
}

func (p *Profile) ReplaceIdleStacks() {
	p.profile.ReplaceIdleStacks()
}

func (p *Profile) Occurrences() []occurrence.Occurrence {
	return p.profile.Occurrences()
}
