package streams

import (
	"github.com/reugn/go-streams"
	"github.com/reugn/go-streams/flow"
)

var _ streams.Source = (*PharosScanResultSource)(nil)

type PharosScanResultSource struct {
	in chan any
}

func NewPharosScanResultSource(channel chan any) *PharosScanResultSource {
	ps := &PharosScanResultSource{
		in: channel,
	}
	// Create an adapter channel that adapts <-chan gtrs.Message[T] to chan any
	return ps
}

// Out returns the output channel of the ChanSource connector.
func (ps *PharosScanResultSource) Out() <-chan any {
	return ps.in
}

// Via asynchronously streams data to the given Flow and returns it.
func (ps *PharosScanResultSource) Via(operator streams.Flow) streams.Flow {
	flow.DoStream(ps, operator)
	return operator
}
